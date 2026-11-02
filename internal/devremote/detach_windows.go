package devremote

import (
	osexec "os/exec"
	"syscall"
)

func detach(command *osexec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000008 | 0x00000200}
}
