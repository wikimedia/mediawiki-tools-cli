//go:build !windows

package devremote

import (
	osexec "os/exec"
	"syscall"
)

func detach(command *osexec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
