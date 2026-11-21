package gerrit

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"

	gogerrit "github.com/andygrunwald/go-gerrit"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	"golang.org/x/crypto/ssh"
)

func newSSHKeyCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "ssh-key", Short: "Manage SSH public keys on your Gerrit account"}
	add := &cobra.Command{
		Use:     "add <public-key-file>",
		Short:   "Register an SSH public key with the selected Gerrit account",
		Long:    "Upload one existing OpenSSH public key (typically a .pub file) using the selected Gerrit profile's HTTP credentials.",
		Example: "add ~/.ssh/id_ed25519.pub",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if gerritUsername == "" || gerritPassword == "" {
				return fmt.Errorf("no credentials found; use mw gerrit auth login")
			}
			key, err := readGerritPublicKey(args[0])
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), gerritRequestTimeout)
			defer cancel()
			client := authenticatedClient(ctx)
			info, err := addGerritSSHKey(ctx, client, key)
			if err != nil {
				return fmt.Errorf("register SSH public key: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Registered SSH key %d for %s (%s)\n", info.Seq, gerritUsername, info.Algorithm)
			return nil
		},
	}
	cmd.AddCommand(commandmeta.Set(add, commandmeta.Hints{
		ToolAnnotations: commandmeta.ToolAnnotations{DestructiveHint: true, IdempotentHint: true, OpenWorldHint: true},
		DataAccess:      []commandmeta.DataAccess{commandmeta.DataSuppliedContent, commandmeta.DataAccountMetadata, commandmeta.DataSecrets},
	}))
	return cmd
}

func readGerritPublicKey(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	const limit = 64 << 10
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, fmt.Errorf("provide an OpenSSH public-key file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	data = bytes.TrimSpace(data)
	fields := bytes.Fields(data)
	if len(data) > limit || len(fields) < 2 || bytes.ContainsAny(data, "\r\n\x00") {
		return nil, fmt.Errorf("provide one OpenSSH public key (usually a .pub file)")
	}
	decoded, err := base64.StdEncoding.DecodeString(string(fields[1]))
	if err == nil {
		key, err := ssh.ParsePublicKey(decoded)
		if err == nil && key.Type() == string(fields[0]) {
			return data, nil
		}
	}
	return nil, fmt.Errorf("provide one OpenSSH public key (usually a .pub file)")
}

func addGerritSSHKey(ctx context.Context, client *gogerrit.Client, key []byte) (*gogerrit.SSHKeyInfo, error) {
	request, err := client.NewRequest(ctx, http.MethodPost, "/accounts/self/sshkeys", nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "text/plain")
	request.Body = io.NopCloser(bytes.NewReader(key))
	request.ContentLength = int64(len(key))

	var info gogerrit.SSHKeyInfo
	response, err := client.Do(request, &info)
	if err != nil {
		if response != nil {
			response.Body.Close()
		}
		return nil, err
	}
	return &info, nil
}
