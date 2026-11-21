package gerrit

import (
	"bufio"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
)

func gerritHTTPSOptions(cmd *cobra.Command, address string) ([]string, error) {
	username, profile, err := gerritIdentity(cmd)
	if err != nil {
		return nil, err
	}
	if username == "" {
		return nil, fmt.Errorf("no Gerrit HTTP credentials configured; use %s gerrit auth login", cmd.Root().Name())
	}
	if cmd.Flags().Changed("password") || cmd.Flags().Changed("password-stdin") {
		return nil, fmt.Errorf("HTTPS push uses saved Gerrit credentials; use %s gerrit auth login to save a password", cmd.Root().Name())
	}
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
	helper := "!" + quote(executable) + " --no-interaction gerrit auth git-credential --username " + quote(username)
	if profile != "" {
		helper += " --profile " + quote(profile)
	}
	return []string{
		"-c", "credential.helper=",
		"-c", "credential.https://" + gerritSSHHost + ".helper=" + helper,
		"-c", "credential.https://" + gerritSSHHost + ".useHttpPath=true",
		"-c", "core.askPass=",
		"-c", "http." + address + ".followRedirects=false",
		"-c", "http." + address + ".sslVerify=true",
	}, nil
}

func gerritHTTPSPushEnv(env []string) []string {
	// Git disables verification whenever this variable exists, regardless of its value.
	env = slices.DeleteFunc(env, func(entry string) bool {
		key, _, _ := strings.Cut(entry, "=")
		return strings.EqualFold(key, "GIT_SSL_NO_VERIFY")
	})
	return append(env, "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=", "GIT_TRACE_REDACT=1")
}

func newGerritGitCredentialCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "git-credential",
		Hidden: true,
		Short:  "Supply Gerrit HTTPS credentials to Git",
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Root().Annotations == nil {
				cmd.Root().Annotations = map[string]string{}
			}
			cmd.Root().Annotations[cli.TelemetrySkipAnnotation] = "true"
			switch args[0] {
			case "store", "erase":
				return nil
			case "get":
			default:
				return fmt.Errorf("unsupported Git credential operation")
			}
			fields := make(map[string]string)
			scanner := bufio.NewScanner(cmd.InOrStdin())
			for scanner.Scan() && scanner.Text() != "" {
				if key, value, ok := strings.Cut(scanner.Text(), "="); ok {
					fields[key] = value
				}
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("could not read Git credential request: %w", err)
			}
			if fields["protocol"] != "https" ||
				(fields["host"] != gerritSSHHost && fields["host"] != gerritSSHHost+":443") ||
				!strings.HasPrefix(fields["path"], "r/") {
				return fmt.Errorf("saved Gerrit credentials are only available for Wikimedia Gerrit HTTPS Git requests")
			}
			if cmd.Flags().Changed("password") || cmd.Flags().Changed("password-stdin") {
				return fmt.Errorf("Git credential helper only uses saved Gerrit credentials")
			}
			username, password, err := resolveGerritCredentials(cmd)
			if err != nil {
				return err
			}
			if username == "" || password == "" {
				return fmt.Errorf("no Gerrit HTTP credentials configured; use %s gerrit auth login", cmd.Root().Name())
			}
			if requested := fields["username"]; requested != "" && requested != username {
				return fmt.Errorf("Git requested credentials for a different Gerrit account")
			}
			if strings.ContainsAny(username+password, "\r\n\x00") {
				return fmt.Errorf("invalid saved Gerrit credentials")
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "username=%s\npassword=%s\n\n", username, password)
			return err
		},
	}
	return commandmeta.Set(cmd, commandmeta.Hints{DataAccess: []commandmeta.DataAccess{commandmeta.DataSecrets}})
}
