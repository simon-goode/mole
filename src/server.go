package mole

import (
	"crypto/aes"
	"crypto/cipher"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//go:embed index.html
//go:embed noble-aes-gcm.js
var indexHTML embed.FS

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func uniquePath(dir, filename string) string {
	destPath := filepath.Join(dir, filename)
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		return destPath
	}

	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	for i := 1; ; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

func handleIndex(cfg *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := indexHTML.ReadFile("index.html")
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}

		s := string(data)
		s = strings.ReplaceAll(s, "{{MODE}}", cfg.Mode)
		s = strings.ReplaceAll(s, "{{SAFE}}", fmt.Sprintf("%t", cfg.Safe))

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(s))
	}
}

func handleUpload(cfg *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}

		if cfg.Mode == "text" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "text mode does not accept file uploads"})
			return
		}

		contentType := r.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "multipart/form-data") {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expected multipart/form-data"})
			return
		}

		mr, err := r.MultipartReader()
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid multipart form"})
			return
		}

		destDir := cfg.outputDir()

		var results []map[string]interface{}

		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "error reading multipart data"})
				return
			}

			filename := part.FileName()
			if filename == "" {
				continue
			}

			encryptedData, err := io.ReadAll(part)
			if err != nil {
				results = append(results, map[string]interface{}{
					"filename": filename,
					"error":    fmt.Sprintf("error reading encrypted data: %v", err),
				})
				continue
			}

			if len(encryptedData) < 12 {
				results = append(results, map[string]interface{}{
					"filename": filename,
					"error":    "corrupted encrypted data",
				})
				continue
			}

			iv := encryptedData[:12]
			ciphertext := encryptedData[12:]

			block, err := aes.NewCipher(cfg.EncKey)
			if err != nil {
				results = append(results, map[string]interface{}{
					"filename": filename,
					"error":    "decryption setup failed",
				})
				continue
			}

			aesgcm, err := cipher.NewGCM(block)
			if err != nil {
				results = append(results, map[string]interface{}{
					"filename": filename,
					"error":    "decryption setup failed",
				})
				continue
			}

			plaintext, err := aesgcm.Open(nil, iv, ciphertext, nil)
			if err != nil {
				results = append(results, map[string]interface{}{
					"filename": filename,
					"error":    "decryption failed: data may be tampered",
				})
				continue
			}

			destPath := uniquePath(destDir, filename)
			if err := os.WriteFile(destPath, plaintext, 0644); err != nil {
				results = append(results, map[string]interface{}{
					"filename": filename,
					"error":    fmt.Sprintf("cannot create file: %v", err),
				})
				continue
			}

			results = append(results, map[string]interface{}{
				"filename": filepath.Base(destPath),
				"size":     len(plaintext),
				"status":   "ok",
			})

			fmt.Printf("Received: %s (%d bytes)\n", filepath.Base(destPath), len(plaintext))
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{"results": results})
	}
}

func handleText(cfg *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}

		if cfg.Mode != "text" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "not in text mode"})
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read body"})
			return
		}

		if len(body) < 12 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "corrupted encrypted data"})
			return
		}

		iv := body[:12]
		ciphertext := body[12:]

		block, err := aes.NewCipher(cfg.EncKey)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "decryption setup failed"})
			return
		}

		aesgcm, err := cipher.NewGCM(block)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "decryption setup failed"})
			return
		}

		plaintext, err := aesgcm.Open(nil, iv, ciphertext, nil)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "decryption failed: data may be tampered"})
			return
		}

		timestamp := time.Now().Format("2006-01-02_150405")
		filename := fmt.Sprintf("mole_%s.txt", timestamp)

		destPath := uniquePath(cfg.outputDir(), filename)
		if err := os.WriteFile(destPath, plaintext, 0644); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save text"})
			return
		}

		fmt.Printf("Received: %s (%d bytes)\n", filepath.Base(destPath), len(plaintext))

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"filename": filepath.Base(destPath),
			"size":     len(plaintext),
			"status":   "ok",
		})
	}
}

func mainHandler(cfg *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		prefix := fmt.Sprintf("/%s/", cfg.Token)
		path := strings.TrimPrefix(r.URL.Path, prefix)

		switch path {
		case "":
			handleIndex(cfg)(w, r)
		case "noble-aes-gcm.js":
			data, err := indexHTML.ReadFile("noble-aes-gcm.js")
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
				return
			}
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			w.Write(data)
		case "upload":
			handleUpload(cfg)(w, r)
		case "text":
			handleText(cfg)(w, r)
		case "health":
			w.WriteHeader(http.StatusOK)
		default:
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		}
	}
}
