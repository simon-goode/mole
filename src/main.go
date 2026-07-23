package main

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

//go:embed index.html
var indexHTML embed.FS

type Config struct {
	Mode   string
	Safe   bool
	Dir    string
	Port   int
	Token  string
	dirSet bool
}

func (c *Config) outputDir() string {
	if c.Safe {
		return filepath.Join(os.TempDir(), "mole-"+c.Token)
	}
	return c.Dir
}

func (c *Config) validateFile(header interface{ Get(string) string }) error {
	ct := header.Get("Content-Type")

	switch c.Mode {
	case "photos":
		if !strings.HasPrefix(ct, "image/") {
			return fmt.Errorf("only image files are accepted in photos mode")
		}
	case "pdfs":
		if ct != "application/pdf" {
			return fmt.Errorf("only PDF files are accepted in pdfs mode")
		}
	}
	return nil
}

func generateToken() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)[:6]
}

func defaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, "Downloads")
}

func parseArgs(args []string, port int) *Config {
	cfg := &Config{
		Mode: "anything",
		Port: port,
	}

	for _, arg := range args {
		switch arg {
		case "photos", "pdfs", "text", "anything":
			cfg.Mode = arg
		case "safe":
			cfg.Safe = true
		case "go":
			cfg.Mode = "go"
		default:
			if strings.HasPrefix(arg, "-") {
				continue
			}
			cfg.Dir = arg
			cfg.dirSet = true
		}
	}

	if !cfg.dirSet && cfg.Mode != "go" {
		cfg.Dir = defaultDir()
	}

	return cfg
}

func main() {
	portFlag := flag.Int("p", 8080, "port to listen on")
	flag.Parse()

	cfg := parseArgs(flag.Args(), *portFlag)

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

	ip, err := detectIP()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error detecting network: %v\n", err)
		os.Exit(1)
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
			fmt.Printf("Files queued in %s\n", cfg.outputDir())
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

	url := fmt.Sprintf("http://%s:%d/%s/", ip, cfg.Port, token)
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
