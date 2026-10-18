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
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
	sshutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/ssh"
)

//go:embed gerrit.long.md
var gerritLong string

var (
	gerritUsername string
	gerritPassword string
)

func NewGerritCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "gerrit",
		GroupID: "service",
		Short:   "Work with Wikimedia Gerrit",
		Long:    cli.RenderMarkdown(gerritLong),
		RunE:    nil,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cobrautil.CallAllPersistentPreRun(cmd, args)
			// Local .gitreview commands do not need HTTP credentials.
			if cmd.Parent() != nil && cmd.Parent().Name() == "dotgitreview" {
				return nil
			}
			// Credential management must work even if an old keyring is locked.
			if cmd.Parent() != nil && cmd.Parent().Name() == "auth" && cmd.Name() != "status" {
				return nil
			}
			username, _ := cmd.Flags().GetString("username")
			password, _ := cmd.Flags().GetString("password")
			if cmd.Name() == "api" {
				if user, _ := cmd.Flags().GetString("auth-user"); user != "" {
					username = user
				}
				if value, _ := cmd.Flags().GetString("auth-password"); value != "" {
					password = value
				}
			}
			if username == "" {
				username = config.State().Effective.Gerrit.Username
			}
			stdin, _ := cmd.Flags().GetBool("password-stdin")
			if stdin {
				if password != "" {
					return fmt.Errorf("--password-stdin cannot be combined with a password argument")
				}
				var err error
				password, err = secrets.ReadInput(cmd.InOrStdin())
				if err != nil {
					return err
				}
			}
			if password == "" {
				var err error
				password, err = (config.Credential{Key: "gerrit.password", Service: "gerrit", Endpoint: secrets.GerritEndpoint, Account: username}).Read()
				if err != nil {
					return err
				}
			}
			gerritUsername, gerritPassword = username, password
			return nil
		},
	}

	// Add persistent flags for username and password
	cmd.PersistentFlags().String("username", "", "Gerrit username (overriding config)")
	cmd.PersistentFlags().String("password", "", "Gerrit password (prefer --password-stdin)")
	cmd.PersistentFlags().Bool("password-stdin", false, "Read a password from stdin for this command without saving it")

	cmd.AddCommand(NewGerritAPICmd())
	cmd.AddCommand(NewGerritSSHCmd())
	cmd.AddCommand(NewGerritAuthCmd())
	cmd.AddCommand(NewGerritDotGitReviewCmd())

	// Generated command trees are documented and exercised by their generator's
	// fixtures. Keep linti focused on the handwritten Gerrit commands here.
	for _, generated := range []*cobra.Command{
		NewGerritAccessCmd(),
		NewGerritAccountsCmd(),
		NewGerritChangesCmd(),
		NewGerritGroupsCmd(),
		NewGerritProjectsCmd(),
		NewGerritServerCmd(),
		NewGerritPluginsCmd(),
	} {
		if generated.Annotations == nil {
			generated.Annotations = make(map[string]string)
		}
		generated.Annotations["mwcli-lint-skip"] = "Generated Gerrit command group"
		generated.Annotations["mwcli-lint-skip-children"] = "Generated Gerrit commands are maintained by code generation"
		cmd.AddCommand(generated)
	}

	return cmd
}

func sshGerritCommand(args []string) *exec.Cmd {
	return sshutil.CommandOnSSHHost("gerrit.wikimedia.org", "29418", append([]string{"gerrit"}, args...))
}

func newGerritClient(ctx context.Context) (*gerrit.Client, error) {
	return gerrit.NewClient(ctx, secrets.GerritEndpoint+"/", nil)
}

func client(ctx context.Context) *gerrit.Client {
	client, err := newGerritClient(ctx)
	if err != nil {
		panic(err)
	}
	return client
}

func authenticatedClient(ctx context.Context) *gerrit.Client {
	client := client(ctx)
	client.Authentication.SetBasicAuth(
		gerritUsername,
		gerritPassword,
	)
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
