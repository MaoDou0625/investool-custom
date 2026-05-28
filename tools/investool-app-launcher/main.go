//go:build windows

package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
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

func main() {
	if err := run(); err != nil {
		_ = appendLog(defaultLogPath(mustWorkingDir()), "launcher error: "+err.Error())
		os.Exit(1)
	}
}

func run() error {
	repoPath := flag.String("repo", "", "investool repository path")
	url := flag.String("url", defaultURL, "URL to open")
	browserPath := flag.String("browser", "", "optional browser executable path")
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
	if isPortOpen(*bindAddress, *port) {
		if err := appendLog(logPath, fmt.Sprintf("port %d is already listening; reusing existing server", *port)); err != nil {
			return err
		}
	} else {
		serverCmd, err = startServer(resolvedRepoPath, serverExe, logPath)
		if err != nil {
			return err
		}
		startedServer = true
		if err := waitForPort(*bindAddress, *port, 30*time.Second); err != nil {
			stopProcess(serverCmd)
			return err
		}
	}

	browserExe, err := resolveBrowserPath(*browserPath)
	if err != nil {
		if startedServer {
			stopProcess(serverCmd)
		}
		return err
	}
	profileDir := filepath.Join(os.Getenv("LOCALAPPDATA"), "InvesToolCustom", "AppBrowserProfile")
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		if startedServer {
			stopProcess(serverCmd)
		}
		return err
	}

	if err := appendLog(logPath, "starting browser app: "+browserExe); err != nil {
		if startedServer {
			stopProcess(serverCmd)
		}
		return err
	}
	browserCmd := exec.Command(browserExe,
		"--user-data-dir="+profileDir,
		"--app="+*url,
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-background-mode",
	)
	browserCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := browserCmd.Start(); err != nil {
		if startedServer {
			stopProcess(serverCmd)
		}
		return err
	}

	err = browserCmd.Wait()
	if err != nil {
		_ = appendLog(logPath, "browser process ended with: "+err.Error())
	}
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
	if path, err := exec.LookPath("msedge.exe"); err == nil {
		return path, nil
	}
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Google", "Chrome", "Application", "chrome.exe"),
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	if path, err := exec.LookPath("chrome.exe"); err == nil {
		return path, nil
	}
	return "", errors.New("Microsoft Edge or Google Chrome was not found")
}

func waitForPort(address string, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if isPortOpen(address, port) {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("server did not listen on %s:%d within %s", address, port, timeout)
}

func isPortOpen(address string, port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", address, port), 300*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
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
