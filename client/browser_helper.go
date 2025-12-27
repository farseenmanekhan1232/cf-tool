package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	stdhttp "net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"time"

	http "github.com/bogdanfinn/fhttp"
	"github.com/fatih/color"
	"github.com/xalanq/cf-tool/cookiejar"
)

// CookiePayload represents cookies sent from browser
type CookiePayload struct {
	Cookies string `json:"cookies"`
	Handle  string `json:"handle"`
}

// LoginBrowserLocal opens user's REAL browser (not automated) for login
func (c *Client) LoginBrowserLocal() error {
	color.Cyan("Starting browser login...")

	// Find an available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("failed to start local server: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	// Channel to receive login result
	resultChan := make(chan *CookiePayload, 1)
	errChan := make(chan error, 1)

	// Create HTTP server
	mux := stdhttp.NewServeMux()

	// Handle CORS preflight and cookie callback
	mux.HandleFunc("/callback", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(stdhttp.StatusOK)
			return
		}

		if r.Method != "POST" {
			stdhttp.Error(w, "Method not allowed", stdhttp.StatusMethodNotAllowed)
			return
		}

		var payload CookiePayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			stdhttp.Error(w, "Invalid payload", stdhttp.StatusBadRequest)
			return
		}

		if payload.Cookies == "" || payload.Handle == "" {
			stdhttp.Error(w, "Missing data", stdhttp.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		resultChan <- &payload
	})

	server := &stdhttp.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", port),
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != stdhttp.ErrServerClosed {
			errChan <- err
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// Open browser to Codeforces with port parameter for extension
	loginURL := fmt.Sprintf("%s?cf_port=%d", c.host, port)
	if err := openBrowser(loginURL); err != nil {
		color.Yellow("Could not open browser. Please open: %s", loginURL)
	}

	// Print instructions
	fmt.Println()
	color.Cyan("╔══════════════════════════════════════════════════════════════╗")
	color.Cyan("║                    CF-TOOL LOGIN                             ║")
	color.Cyan("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	color.HiGreen("🔌 Login will complete automatically with the CF-Tool extension!")
	fmt.Println()
	color.White("   1. Log in to Codeforces in the browser")
	color.White("   2. That's it! The extension handles the rest.")
	fmt.Println()
	color.Yellow("   Don't have the extension?")
	color.White("   Install: https://github.com/farseenmanekhan1232/cf-tool/extension")
	fmt.Println()

	color.Cyan("────────────────────────────────────────────────────────────────")
	color.Yellow("⏳ Waiting for login... (Ctrl+C to cancel)")
	color.Cyan("────────────────────────────────────────────────────────────────")
	fmt.Println()

	// Wait for result
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	var payload *CookiePayload
	select {
	case payload = <-resultChan:
	case err := <-errChan:
		server.Shutdown(context.Background())
		return fmt.Errorf("server error: %v", err)
	case <-ctx.Done():
		server.Shutdown(context.Background())
		return fmt.Errorf("login timeout")
	}

	server.Shutdown(context.Background())

	// Parse cookies
	jar, _ := cookiejar.New(nil)
	u, _ := url.Parse(c.host)
	httpCookies := parseCookieString(payload.Cookies)

	if len(httpCookies) == 0 {
		return fmt.Errorf("no valid cookies received")
	}

	jar.SetCookies(u, httpCookies)
	c.client.SetCookieJar(jar)
	c.Jar = jar
	c.Handle = payload.Handle

	color.Green("✓ Login successful!")
	color.Green("Welcome %v~", c.Handle)
	return c.save()
}

// parseCookieString parses cookie string to http.Cookie slice
func parseCookieString(cookieStr string) []*http.Cookie {
	var cookies []*http.Cookie
	for _, part := range splitCookies(cookieStr) {
		if idx := indexOf(part, "="); idx > 0 {
			cookies = append(cookies, &http.Cookie{
				Name:   trim(part[:idx]),
				Value:  trim(part[idx+1:]),
				Domain: ".codeforces.com",
				Path:   "/",
			})
		}
	}
	return cookies
}

func splitCookies(s string) []string {
	var result []string
	current := ""
	for i := 0; i < len(s); i++ {
		if i < len(s)-1 && s[i] == ';' && s[i+1] == ' ' {
			result = append(result, current)
			current = ""
			i++ // skip space
		} else {
			current += string(s[i])
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func indexOf(s string, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func trim(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

// getChromeUserDataDir returns Chrome user data directory (kept for reference)
func getChromeUserDataDir() (string, error) {
	var dir string
	switch goruntime.GOOS {
	case "darwin":
		dir = filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "Google", "Chrome")
	case "linux":
		dir = filepath.Join(os.Getenv("HOME"), ".config", "google-chrome")
	case "windows":
		dir = filepath.Join(os.Getenv("LOCALAPPDATA"), "Google", "Chrome", "User Data")
	default:
		return "", fmt.Errorf("unsupported OS")
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return "", err
	}
	return dir, nil
}

// openBrowser opens URL in default system browser
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch goruntime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform")
	}
	return cmd.Start()
}
