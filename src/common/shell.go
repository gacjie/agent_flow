package common

import (
	"os/exec"
	"runtime"
	"sync"
)

type ShellInfo struct {
	Name string // "bash" / "powershell" / "cmd"
	Path string
	Flag string // "-c" / "-Command" / "/c"
}

var (
	detectedShell *ShellInfo
	shellOnce     sync.Once
)

func DetectShell() *ShellInfo {
	shellOnce.Do(func() {
		if runtime.GOOS == "windows" {
			detectedShell = detectWindows()
		} else {
			detectedShell = detectUnix()
		}
	})
	return detectedShell
}

func GetShell() *ShellInfo {
	return DetectShell()
}

func detectWindows() *ShellInfo {
	candidates := []ShellInfo{
		{"bash", "bash", "-c"},
		{"powershell", "powershell", "-Command"},
		{"cmd", "cmd", "/c"},
	}
	for _, c := range candidates {
		if path, err := exec.LookPath(c.Name); err == nil {
			c.Path = path
			return &c
		}
	}
	return &ShellInfo{Name: "cmd", Path: "cmd", Flag: "/c"}
}

func detectUnix() *ShellInfo {
	candidates := []ShellInfo{
		{"bash", "bash", "-c"},
		{"sh", "sh", "-c"},
	}
	for _, c := range candidates {
		if path, err := exec.LookPath(c.Name); err == nil {
			c.Path = path
			return &c
		}
	}
	return &ShellInfo{Name: "sh", Path: "/bin/sh", Flag: "-c"}
}
