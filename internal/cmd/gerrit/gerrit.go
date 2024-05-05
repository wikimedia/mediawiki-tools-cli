package gerrit

import (
	"context"
	_ "embed"
	"fmt"
	"net/url"
	"os/exec"
	"strings"

	"github.com/andygrunwald/go-gerrit"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	sshutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/ssh"
)

//go:embed gerrit.long.md
var gerritLong string

func NewGerritCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "gerrit",
		GroupID: "service",
		Short:   "Interact with the Wikimedia Gerrit instance (WORK IN PROGRESS)",
		Long:    cli.RenderMarkdown(gerritLong),
		RunE:    nil,
	}

	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Service"

	cmd.AddCommand(NewGerritAPICmd())
	cmd.AddCommand(NewGerritSSHCmd())
	cmd.AddCommand(NewGerritAuthCmd())
	cmd.AddCommand(NewGerritDotGitReviewCmd())

	// Add auto generated commands...
	cmd.AddCommand(NewGerritAccessCmd())
	cmd.AddCommand(NewGerritAccountsCmd())
	cmd.AddCommand(NewGerritChangesCmd())
	cmd.AddCommand(NewGerritGroupsCmd())
	cmd.AddCommand(NewGerritProjectsCmd())
	cmd.AddCommand(NewGerritServerCmd())
	cmd.AddCommand(NewGerritPluginsCmd())

	return cmd
}

func sshGerritCommand(args []string) *exec.Cmd {
	return sshutil.CommandOnSSHHost("gerrit.wikimedia.org", "29418", append([]string{"gerrit"}, args...))
}

func client(ctx context.Context) *gerrit.Client {
	client, err := gerrit.NewClient(ctx, "https://gerrit.wikimedia.org/r/", nil)
	if err != nil {
		panic(err)
	}
	return client
}

func authenticatedClient(ctx context.Context) *gerrit.Client {
	config := LoadConfig()
	client := client(ctx)
	client.Authentication.SetBasicAuth(config.Username, config.Password)
	return client
}

func addParamToPath(path string, name string, value string) string {
	// URL encode value
	value = url.QueryEscape(value)
	if strings.Contains(path, fmt.Sprintf("{%s}", name)) {
		// Replace {key} with value
		path = strings.Replace(path, fmt.Sprintf("{%s}", name), value, -1)
	} else if value != "" {
		// Append ?key=value
		if strings.Contains(path, "?") {
			path += fmt.Sprintf("&%s=%s", name, value)
		} else {
			path += fmt.Sprintf("?%s=%s", name, value)
		}
	}
	return path
}
