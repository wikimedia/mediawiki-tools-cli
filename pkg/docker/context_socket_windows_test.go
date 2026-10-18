package docker

import (
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"
)

func nativeDockerListener(t *testing.T) (net.Listener, string) {
	t.Helper()
	name := fmt.Sprintf("mwcli-docker-%d-%d", os.Getpid(), time.Now().UnixNano())
	listener, err := winio.ListenPipe(`\\.\pipe\`+name, nil)
	if err != nil {
		t.Fatal(err)
	}
	return listener, "npipe:////./pipe/" + name
}
