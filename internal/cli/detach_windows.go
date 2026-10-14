//go:build windows

package cli

import (
	"os/exec"
	"syscall"
)

func detach(c *exec.Cmd) {
	const detachedProcess = 0x00000008
	const createNewProcessGroup = 0x00000200
	const createNoWindow = 0x08000000
	c.SysProcAttr = &syscall.SysProcAttr{CreationFlags: detachedProcess | createNewProcessGroup | createNoWindow, HideWindow: true}
}
