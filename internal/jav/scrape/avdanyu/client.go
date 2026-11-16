// Package avdanyu implements male-actor lookup from the public AV男優まとめ
// (avdanyuwiki.com) search pages. The site is protected by Cloudflare, so the
// actual page fetch is delegated to a locally running Chromium/Brave debugging
// endpoint via the bundled fetch.mjs helper.
package avdanyu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Work is one movie entry returned by the avdanyu search page.
type Work struct {
	Title       string   `json:"title"`
	Code        string   `json:"code"`
	Studio      string   `json:"studio"`
	Series      string   `json:"series"`
	DurationMin int      `json:"duration_min"`
	ReleaseDate string   `json:"release_date"`
	Actors      []string `json:"actors"`
}

// Result is the raw result produced by fetch.mjs.
type Result struct {
	Keyword string `json:"keyword"`
	Title   string `json:"title"`
	Works   []Work `json:"works"`
	Error   string `json:"error"`
	Message string `json:"message"`
}

// ErrUnavailable is returned when the local browser helper cannot be used.
var ErrUnavailable = errors.New("avdanyu: local browser helper unavailable")

// Client runs the bundled Node helper against a local Chromium/Brave CDP endpoint.
type Client struct {
	ScriptPath    string
	localScript   string
	CDPBase       string
	NodeBin       string
	Timeout       time.Duration
}

// NewClient builds a client. The CDP endpoint can be overridden with the
// AVDANYU_CDP environment variable; the default matches the local Brave setup.
func NewClient() *Client {
	cdp := strings.TrimSpace(os.Getenv("AVDANYU_CDP"))
	if cdp == "" {
		cdp = "http://127.0.0.1:9222"
	}
	script := strings.TrimSpace(os.Getenv("AVDANYU_SCRIPT"))
	node := strings.TrimSpace(os.Getenv("AVDANYU_NODE"))
	if node == "" {
		// The dev backend runs inside WSL while the browser/Node live on Windows.
		// WSL interop exposes Windows executables as *.exe on the Linux PATH.
		if runtime.GOOS == "linux" {
			node = "node.exe"
		} else {
			node = "node"
		}
	}
	if script == "" {
		_, thisFile, _, ok := runtime.Caller(0)
		if ok {
			script = filepath.Join(filepath.Dir(thisFile), "fetch.mjs")
		}
	}
	localScript := script
	// runtime.Caller inside WSL returns a /mnt/<drive>/... path, but the
	// Windows Node process needs a native drive path.
	if runtime.GOOS == "linux" {
		script = toWindowsPath(script)
	}
	return &Client{ScriptPath: script, localScript: localScript, CDPBase: cdp, NodeBin: node, Timeout: 60 * time.Second}
}

// toWindowsPath converts a WSL mount path (/mnt/e/foo) to a Windows path (E:\foo).
func toWindowsPath(p string) string {
	if len(p) >= 7 && strings.HasPrefix(p, "/mnt/") && p[6] == '/' {
		drive := p[5]
		if (drive >= 'a' && drive <= 'z') || (drive >= 'A' && drive <= 'Z') {
			return strings.ToUpper(string(drive)) + ":" + strings.ReplaceAll(p[6:], "/", "\\")
		}
	}
	return p
}

func (c *Client) helperPath() string {
	if c.localScript != "" {
		return c.localScript
	}
	return c.ScriptPath
}

// Available reports whether the Node helper and script are present.
func (c *Client) Available() bool {
	if c == nil {
		return false
	}
	if _, err := os.Stat(c.helperPath()); err != nil {
		return false
	}
	if _, err := exec.LookPath(c.NodeBin); err != nil {
		return false
	}
	return true
}

// Lookup searches avdanyu for the supplied keyword (JAV code or actress name).
func (c *Client) Lookup(ctx context.Context, keyword string) (*Result, error) {
	if c == nil || c.ScriptPath == "" {
		return nil, ErrUnavailable
	}
	if _, err := os.Stat(c.helperPath()); err != nil {
		return nil, ErrUnavailable
	}
	node, err := exec.LookPath(c.NodeBin)
	if err != nil {
		return nil, ErrUnavailable
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, node, c.ScriptPath, keyword, c.CDPBase)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("%w: helper exit %d: %s", ErrUnavailable, exitErr.ExitCode(), strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	var result Result
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("avdanyu: decode helper output: %w", err)
	}
	if result.Error == "cloudflare_blocked" {
		return &result, fmt.Errorf("avdanyu: 浏览器验证未通过，请先在 Brave 里打开一次 avdanyuwiki.com")
	}
	if result.Error != "" && result.Error != "fetch_failed" {
		return &result, fmt.Errorf("avdanyu: %s %s", result.Error, result.Message)
	}
	return &result, nil
}
