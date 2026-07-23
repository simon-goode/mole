package mole

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

type Config struct {
	Mode   string
	Safe   bool
	Dir    string
	Port   int
	IP     string
	Token  string
	EncKey []byte
	dirSet bool
}

func (c *Config) outputDir() string {
	if c.Safe {
		return filepath.Join(os.TempDir(), "mole-"+c.Token)
	}
	return c.Dir
}

func generateToken() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)[:6]
}

func defaultDir() string {
	return "."
}

func parseArgs(args []string) *Config {
	cfg := &Config{
		Mode: "anything",
		Port: 8080,
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-p" || arg == "--port":
			if i+1 < len(args) {
				port, err := strconv.Atoi(args[i+1])
				if err == nil {
					cfg.Port = port
				}
				i++
			}
		case arg == "-i" || arg == "--ip":
			if i+1 < len(args) {
				cfg.IP = args[i+1]
				i++
			}
		case arg == "photos" || arg == "pdfs" || arg == "text" || arg == "anything":
			cfg.Mode = arg
		case arg == "safe":
			cfg.Safe = true
		case arg == "go":
			cfg.Mode = "go"
		case strings.HasPrefix(arg, "-"):
			continue
		default:
			cfg.Dir = arg
			cfg.dirSet = true
		}
	}

	if !cfg.dirSet && cfg.Mode != "go" {
		cfg.Dir = defaultDir()
	}

	return cfg
}

func Run() {
	cfg := parseArgs(os.Args[1:])

	if cfg.Mode == "go" {
		releaseDir := ""
		if cfg.dirSet {
			releaseDir = cfg.Dir
		}
		if err := releaseFiles(releaseDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	token := generateToken()
	cfg.Token = token

	encKey := make([]byte, 32)
	if _, err := rand.Read(encKey); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating encryption key: %v\n", err)
		os.Exit(1)
	}
	cfg.EncKey = encKey

	ip := cfg.IP
	if ip == "" {
		var err error
		ip, err = detectIP()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error detecting network: %v\n", err)
			os.Exit(1)
		}
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		found := false
		for port := cfg.Port + 1; port < cfg.Port+100; port++ {
			listener, err = net.Listen("tcp", fmt.Sprintf(":%d", port))
			if err == nil {
				cfg.Port = port
				found = true
				break
			}
		}
		if !found {
			fmt.Fprintf(os.Stderr, "Error: no available port found (tried %d-%d)\n", cfg.Port, cfg.Port+99)
			os.Exit(1)
		}
	}
	defer listener.Close()

	if err := os.MkdirAll(cfg.outputDir(), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	if cfg.Safe {
		state := SessionState{
			TempDir: cfg.outputDir(),
			DestDir: cfg.Dir,
			Mode:    cfg.Mode,
		}
		if err := saveState(state); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not save session state: %v\n", err)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/%s/", token), mainHandler(cfg))

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: mux,
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nShutting down...")
		server.Close()
		if cfg.Safe {
			tempDir := cfg.outputDir()
			entries, err := os.ReadDir(tempDir)
			if err == nil && len(entries) == 0 {
				os.RemoveAll(tempDir)
				clearState()
			} else {
				fmt.Printf("Files queued in %s\n", tempDir)
			}
		}
		os.Exit(0)
	}()

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			os.Exit(1)
		}
	}()

	if !checkConnectivity(ip, cfg.Port, token) {
		fmt.Println("\n⚠  You're probably on a network that blocks peer-to-peer connections.")
		fmt.Println("   Phones may not be able to reach this device.")
		fmt.Println()
	}

	b64Key := base64.RawURLEncoding.EncodeToString(encKey)
	url := fmt.Sprintf("http://%s:%d/%s/#%s", ip, cfg.Port, token, b64Key)
	showQR(url)

	fmt.Printf("\n📡  Server: %s\n", url)
	fmt.Printf("    Mode: %s\n", cfg.Mode)
	if cfg.Safe {
		fmt.Printf("    📦 Safe mode: files queued to temp. Run 'mole go' to release to %s\n", cfg.Dir)
	} else {
		fmt.Printf("    📁 Saving to: %s\n", cfg.Dir)
	}
	fmt.Println("    Press Ctrl+C to stop.")

	select {}
}
