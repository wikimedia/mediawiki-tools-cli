package devremote

import (
	"context"
	_ "embed"
	"io"
	osexec "os/exec"
	"strings"
)

//go:embed web-editor.sh
var webEditorScript string

// WebEditor manages a host-native editor so its terminal has the VM's tools,
// file paths and private session bridge without mounting the Docker socket.
func WebEditor(ctx context.Context, workspace, metadata, source, action, name string, stdout, stderr io.Writer) error {
	command := osexec.CommandContext(ctx, "bash", "-s", "--", workspace, metadata, source, action, name)
	command.Stdin, command.Stdout, command.Stderr = strings.NewReader(webEditorScript), stdout, stderr
	return command.Run()
}
