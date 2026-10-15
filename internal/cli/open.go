package cli

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func isWSL() bool {
	return runtime.GOOS == "linux" && os.Getenv("WSL_DISTRO_NAME") != ""
}

func capture(dir, name string, args ...string) string {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// viewDir is where the HTML page is written. On WSL a Windows browser cannot always open
// files under \\wsl.localhost\..., so the Windows temp folder is used (reachable from both sides).
func viewDir() string {
	if isWSL() {
		if winTemp := capture("/mnt/c", "cmd.exe", "/c", "echo %TEMP%"); winTemp != "" {
			if linux := capture("", "wslpath", "-u", winTemp); linux != "" {
				if st, err := os.Stat(linux); err == nil && st.IsDir() {
					return linux
				}
			}
		}
	}
	return os.TempDir()
}

func launch(name string, args ...string) bool {
	cmd := exec.Command(name, args...)
	if err := cmd.Start(); err != nil {
		return false
	}
	go func() { _ = cmd.Wait() }() // exit status is ignored: explorer.exe returns 1 on success
	return true
}

// OpenFile opens a file with the OS default handler (the browser for .html).
func OpenFile(file string) bool {
	switch {
	case runtime.GOOS == "darwin":
		return launch("open", file)
	case runtime.GOOS == "windows":
		return launch("rundll32", "url.dll,FileProtocolHandler", file)
	case isWSL():
		if launch("wslview", file) {
			return true
		}
		if win := capture("", "wslpath", "-w", file); win != "" {
			return launch("explorer.exe", win)
		}
		return false
	}
	return launch("xdg-open", file)
}
