//go:build !windows

package devremote

import (
	"context"
	"net"
	"path/filepath"
)

func controlAddress(directory string) string             { return filepath.Join(directory, "control.sock") }
func sshControlPath(directory string) string             { return filepath.Join(directory, "ssh") }
func listenControl(address string) (net.Listener, error) { return net.Listen("unix", address) }
func dialControl(ctx context.Context, address string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, "unix", address)
}
