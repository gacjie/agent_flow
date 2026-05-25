//go:build windows

package tool

import (
	"fmt"
	"os/exec"
	"syscall"
)

func setProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

func killProcGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		exec.Command("taskkill", "/F", "/T", "/PID",
			fmt.Sprintf("%d", cmd.Process.Pid)).Run()
	}
}
