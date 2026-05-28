//go:build windows

package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	defaultURL         = "http://127.0.0.1:4869/fund"
	defaultBindAddress = "127.0.0.1"
	defaultPort        = 4869
)

type launcherSessionStatus struct {
	Alive bool `json:"alive"`
}

func main() {
	if err := run(); err != nil {
		_ = appendLog(defaultLogPath(mustWorkingDir()), "launcher error: "+err.Error())
		os.Exit(1)
	}
}

func run() error {
	repoPath := flag.String("repo", "", "investool repository path")
	targetURL := flag.String("url", defaultURL, "URL to open")
	browserPath := flag.String("browser", "", "optional browser executable path; leave empty to use the Windows default browser")
	bindAddress := flag.String("bind", defaultBindAddress, "local bind address")
	port := flag.Int("port", defaultPort, "local server port")
	flag.Parse()

	resolvedRepoPath, err := resolveRepoPath(*repoPath)
	if err != nil {
		return err
	}
	logPath := defaultLogPath(resolvedRepoPath)
	if err := appendLog(logPath, "launcher starting"); err != nil {
		return err
	}

	serverExe := filepath.Join(resolvedRepoPath, "bin", "investool-custom.exe")
	if _, err := os.Stat(serverExe); err != nil {
		return fmt.Errorf("server executable not found: %s", serverExe)
	}

	var serverCmd *exec.Cmd
	startedServer := false
	if err := waitForHTTPReady(*targetURL, 2*time.Second); err == nil {
		if err := appendLog(logPath, "server already ready; reusing existing server"); err != nil {
			return err
		}
	} else {
		serverCmd, err = startServer(resolvedRepoPath, serverExe, logPath)
		if err != nil {
			return err
		}
		startedServer = true
		if err := waitForHTTPReady(*targetURL, 60*time.Second); err != nil {
			stopProcess(serverCmd)
			return err
		}
	}

	sessionID, err := newSessionID()
	if err != nil {
		stopStartedServer(startedServer, serverCmd)
		return err
	}
	launchURL, err := addQueryParam(*targetURL, "launcher_session", sessionID)
	if err != nil {
		stopStartedServer(startedServer, serverCmd)
		return err
	}
	statusURL, err := launcherStatusURL(*bindAddress, *port, sessionID)
	if err != nil {
		stopStartedServer(startedServer, serverCmd)
		return err
	}

	if err := appendLog(logPath, "opening browser: "+launchURL); err != nil {
		stopStartedServer(startedServer, serverCmd)
		return err
	}
	if err := openURL(*browserPath, launchURL); err != nil {
		stopStartedServer(startedServer, serverCmd)
		return err
	}

	if err := waitForLauncherSession(statusURL, true, 45*time.Second); err != nil {
		_ = appendLog(logPath, "browser page did not report ready: "+err.Error())
		stopStartedServer(startedServer, serverCmd)
		return err
	}
	waitForLauncherSession(statusURL, false, 0)

	if startedServer {
		_ = appendLog(logPath, "stopping owned server")
		stopProcess(serverCmd)
	}
	return appendLog(logPath, "launcher exiting")
}

func resolveRepoPath(repoPath string) (string, error) {
	if strings.TrimSpace(repoPath) != "" {
		return filepath.Abs(repoPath)
	}
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	// Expected layout: <repo>\bin\investool-app-launcher.exe.
	return filepath.Abs(filepath.Join(filepath.Dir(exePath), ".."))
}

func startServer(repoPath string, serverExe string, logPath string) (*exec.Cmd, error) {
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(serverExe, "webserver", "-c", ".\\config.toml")
	cmd.Dir = repoPath
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return nil, err
	}
	_ = logFile.Close()
	return cmd, nil
}

func resolveBrowserPath(requested string) (string, error) {
	if strings.TrimSpace(requested) != "" {
		if _, err := os.Stat(requested); err == nil {
			return requested, nil
		}
		return "", fmt.Errorf("browser not found: %s", requested)
	}
	return "", errors.New("browser path was empty")
}

func openURL(browserPath string, launchURL string) error {
	if strings.TrimSpace(browserPath) != "" {
		browserExe, err := resolveBrowserPath(browserPath)
		if err != nil {
			return err
		}
		cmd := exec.Command(browserExe, launchURL)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		return cmd.Start()
	}

	cmd := exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", launchURL)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Start()
}

func waitForHTTPReady(readyURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}
	for time.Now().Before(deadline) {
		resp, err := client.Get(readyURL)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 500 {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("server did not become ready at %s within %s", readyURL, timeout)
}

func waitForLauncherSession(statusURL string, wantAlive bool, timeout time.Duration) error {
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Time{}
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	for {
		alive, err := queryLauncherSessionAlive(client, statusURL)
		if err == nil && alive == wantAlive {
			return nil
		}
		if !deadline.IsZero() && time.Now().After(deadline) {
			if err != nil {
				return err
			}
			return fmt.Errorf("launcher session did not reach alive=%v", wantAlive)
		}
		time.Sleep(2 * time.Second)
	}
}

func queryLauncherSessionAlive(client *http.Client, statusURL string) (bool, error) {
	resp, err := client.Get(statusURL)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("session status returned %d", resp.StatusCode)
	}
	status := launcherSessionStatus{}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return false, err
	}
	return status.Alive, nil
}

func newSessionID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func addQueryParam(rawURL string, key string, value string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set(key, value)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func launcherStatusURL(address string, port int, sessionID string) (string, error) {
	statusURL := fmt.Sprintf("http://%s:%d/launcher/session/status", address, port)
	return addQueryParam(statusURL, "session", sessionID)
}

func stopStartedServer(startedServer bool, cmd *exec.Cmd) {
	if startedServer {
		stopProcess(cmd)
	}
}

func stopProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()
}

func defaultLogPath(repoPath string) string {
	return filepath.Join(repoPath, "tmp", "investool_app_launcher.log")
}

func appendLog(path string, message string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = fmt.Fprintf(file, "[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), message)
	return err
}

func mustWorkingDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}
