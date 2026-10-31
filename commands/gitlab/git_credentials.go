package gitlab

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.com/gitlab-org/cli/mwcli"
)

func newGitCredentialCmd(session *profileSession) *cobra.Command {
	return &cobra.Command{Use: "git-credential", Hidden: true, Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if args[0] == "store" || args[0] == "erase" {
			return nil
		}
		if args[0] != "get" {
			return fmt.Errorf("unsupported Git credential operation")
		}
		fields := map[string]string{}
		scanner := bufio.NewScanner(cmd.InOrStdin())
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				break
			}
			key, value, ok := strings.Cut(line, "=")
			if ok {
				fields[key] = value
			}
		}
		if scanner.Err() != nil {
			return fmt.Errorf("could not read Git credential request")
		}
		if raw := fields["url"]; raw != "" {
			u, err := url.Parse(raw)
			if err != nil {
				return fmt.Errorf("invalid Git credential URL")
			}
			fields["host"], fields["protocol"] = u.Host, u.Scheme
			if u.User != nil {
				fields["username"] = u.User.Username()
			}
		}
		if fields["protocol"] != "https" {
			return fmt.Errorf("GitLab profile credentials require HTTPS")
		}
		profile, err := session.credentials()
		if err != nil {
			return err
		}
		var token string
		if profile != nil {
			if !strings.EqualFold(fields["host"], profile.Host) {
				return fmt.Errorf("Git credential host does not match the selected GitLab profile")
			}
			if user := fields["username"]; user != "" && user != "oauth2" && !strings.EqualFold(user, profile.Username) {
				return fmt.Errorf("Git credential username does not match the selected GitLab profile")
			}
			token, err = profile.Token()
		} else {
			// Preserve the previously installed mw helper until a profile is imported.
			host := fields["host"]
			if host != "gitlab.wikimedia.org" {
				return fmt.Errorf("no GitLab profile selected for this host")
			}
			token, err = mwcli.LegacyGitToken(host)
		}
		if err != nil {
			return err
		}
		if strings.ContainsAny(token, "\r\n\x00") {
			return fmt.Errorf("invalid saved Git credential")
		}
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "protocol=https\nhost=%s\nusername=oauth2\npassword=%s\n\n", fields["host"], token)
		return err
	}}
}

// Git subprocesses inherit a host-scoped helper, never the token itself. Empty
// helper entries reset earlier helpers so they cannot silently use another user.
func profileGitEnvironment(name string, profile *mwcli.ProfileCredentials) (func(), error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	count := 0
	if raw := os.Getenv("GIT_CONFIG_COUNT"); raw != "" {
		count, err = strconv.Atoi(raw)
		if err != nil || count < 0 || count > 1000 {
			return nil, fmt.Errorf("invalid inherited GIT_CONFIG_COUNT")
		}
	}
	helper := "!" + shellQuote(executable) + " gitlab --profile " + shellQuote(name) + " auth git-credential"
	values := map[string]string{profileEnvironment: name, "GIT_TERMINAL_PROMPT": "0", "GIT_CONFIG_COUNT": strconv.Itoa(count + 3)}
	for i, value := range []string{"", helper} {
		values[fmt.Sprintf("GIT_CONFIG_KEY_%d", count+i)] = "credential.https://" + profile.Host + ".helper"
		values[fmt.Sprintf("GIT_CONFIG_VALUE_%d", count+i)] = value
	}
	values[fmt.Sprintf("GIT_CONFIG_KEY_%d", count+2)] = "core.askPass"
	values[fmt.Sprintf("GIT_CONFIG_VALUE_%d", count+2)] = ""
	// Suppress upstream environment authentication and interactive Git fallbacks.
	for _, key := range []string{"GITLAB_TOKEN", "GITLAB_ACCESS_TOKEN", "OAUTH_TOKEN", "CI_JOB_TOKEN", "GIT_ASKPASS", "SSH_ASKPASS"} {
		values[key] = ""
	}
	// These credentials must not override the profile in nested mw invocations.
	var restore []func()
	for key, value := range values {
		previous, present := os.LookupEnv(key)
		k := key
		restore = append(restore, func() {
			if present {
				_ = os.Setenv(k, previous)
			} else {
				_ = os.Unsetenv(k)
			}
		})
		if err := os.Setenv(key, value); err != nil {
			for _, undo := range restore {
				undo()
			}
			return nil, err
		}
	}
	return func() {
		for _, undo := range restore {
			undo()
		}
	}, nil
}

func installProfileCommands(root *cobra.Command, factory mwcli.Factory, session *profileSession) {
	// Protect all embedded execution paths, including ones whose own persistent
	// pre-run hook would otherwise bypass a parent authentication check.
	var visit func(*cobra.Command)
	visit = func(cmd *cobra.Command) {
		for _, child := range cmd.Commands() {
			visit(child)
		}
		runE, run := cmd.RunE, cmd.Run
		if runE == nil && run == nil {
			return
		}
		cmd.Run = nil
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			top := cmd
			for top.Parent() != nil && top.Parent() != root {
				top = top.Parent()
			}
			if top.Name() == "config" {
				// Upstream config commands can print or save tokens; profiles use auth.
				for _, arg := range args {
					if mwcli.IsCredentialKey(arg) {
						return fmt.Errorf("manage credentials with gitlab auth login and config secrets")
					}
				}
			}
			skip := top.Name() == "auth" || top.Name() == "config" || top.Name() == "alias" || top.Name() == "completion" || top.Name() == "version"
			if !skip {
				profile, err := session.credentials()
				if err != nil {
					return err
				}
				if profile != nil {
					host := profile.Host
					if flag := cmd.Flags().Lookup("hostname"); flag != nil {
						if cmd.Flags().Changed("hostname") {
							host = mwcli.Hostname(flag.Value.String())
						} else {
							_ = flag.Value.Set(profile.Host)
						}
					} else if err := mwcli.ValidateRepositoryHost(factory, profile.Host); err != nil {
						return err
					}
					if !strings.EqualFold(host, profile.Host) {
						return fmt.Errorf("selected GitLab profile belongs to %s, not %s", profile.Host, host)
					}
					if top.Name() == "api" {
						restoreHost, err := mwcli.OverrideAPIHost(factory, host)
						if err != nil {
							return err
						}
						defer restoreHost()
					}
					if _, err := profile.Token(); err != nil {
						return err
					}
					name, err := session.name()
					if err != nil {
						return err
					}
					restore, err := profileGitEnvironment(name, profile)
					if err != nil {
						return err
					}
					defer restore()
				}
			}
			if runE != nil {
				return runE(cmd, args)
			}
			run(cmd, args)
			return nil
		}
	}
	visit(root)
	for _, child := range root.Commands() {
		if child.Name() == "auth" {
			root.RemoveCommand(child)
		}
	}
	root.AddCommand(newGitlabAuthCmd(session))
}
