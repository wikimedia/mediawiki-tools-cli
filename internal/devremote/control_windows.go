package devremote

import (
	"context"
	"net"
	"path/filepath"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

func controlAddress(directory string) string {
	return `\\.\pipe\a-wikimedia-cli-` + filepath.Base(directory)
}

// Windows OpenSSH does not support connection multiplexing.
func sshControlPath(string) string { return "" }
func listenControl(address string) (net.Listener, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	return winio.ListenPipe(address, &winio.PipeConfig{
		SecurityDescriptor: "D:P(A;;GA;;;SY)(A;;GA;;;" + user.User.Sid.String() + ")",
	})
}
func dialControl(ctx context.Context, address string) (net.Conn, error) {
	return winio.DialPipeContext(ctx, address)
}
