package ssh

import (
	"os/exec"
)

func CommandOnSSHHost(host string, port string, commandAndArgs []string) *exec.Cmd {
	ssh := exec.Command("ssh", "-p", port, host, commandAndArgs[0])
	if len(commandAndArgs) > 1 {
		ssh.Args = append(ssh.Args, commandAndArgs[1:]...)
	}
	return ssh
}
