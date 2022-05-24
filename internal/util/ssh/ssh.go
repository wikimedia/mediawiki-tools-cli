package ssh

import (
	"os/exec"
)

func CommandOnSSHHost(host string, port string, tty bool, commandAndArgs []string) *exec.Cmd {
	arguments := []string{}
	if tty {
		arguments = append(arguments, "-t")
	}
	arguments = append(arguments, "-p", port, host, commandAndArgs[0])

	ssh := exec.Command("ssh", arguments...)
	if len(commandAndArgs) > 1 {
		ssh.Args = append(ssh.Args, commandAndArgs[1:]...)
	}
	return ssh
}
