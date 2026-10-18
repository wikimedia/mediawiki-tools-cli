package cloudvps

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

func TestCredentialPromptsFillOnlyMissingValuesAndHideSecret(t *testing.T) {
	for _, tc := range []struct {
		name, project, id, secret string
		prompts                   int
	}{
		{"all missing", "", "", "", 3},
		{"project supplied", "mwcli", "", "", 2},
		{"secret missing", "mwcli", "existing-id", "", 1},
		{"complete", "mwcli", "existing-id", "existing-secret", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			project, id, secret := tc.project, tc.id, tc.secret
			var text bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetErr(&text)
			calls := 0
			ask := func(prompt survey.Prompt, answer interface{}, options ...survey.AskOpt) error {
				calls++
				if len(options) == 0 {
					t.Fatal("required input not validated")
				}
				target := answer.(*string)
				if target == &secret {
					if _, ok := prompt.(*survey.Password); !ok {
						t.Fatal("secret is not a hidden prompt")
					}
					*target = "hidden-test-secret"
				} else {
					if _, ok := prompt.(*survey.Input); !ok {
						t.Fatal("unexpected prompt")
					}
					*target = "test-input"
				}
				return nil
			}
			if err := promptApplicationCredential(cmd, &project, &id, &secret, ask); err != nil {
				t.Fatal(err)
			}
			if calls != tc.prompts {
				t.Fatalf("prompted %d times, want %d", calls, tc.prompts)
			}
			if tc.project != "" && project != tc.project || tc.id != "" && id != tc.id || tc.secret != "" && secret != tc.secret {
				t.Fatal("supplied value overwritten")
			}
			if !strings.Contains(text.String(), "https://horizon.wikimedia.org/identity/application_credentials/") {
				t.Fatal("missing credential creation link")
			}
			if strings.Contains(text.String(), "hidden-test-secret") || strings.Contains(text.String(), "existing-secret") {
				t.Fatal("secret printed")
			}
		})
	}
}

func TestCredentialPromptCancellationStopsSetup(t *testing.T) {
	cancelled := errors.New("cancelled")
	project, id, secret := "", "", ""
	cmd := &cobra.Command{}
	cmd.SetErr(&bytes.Buffer{})
	calls := 0
	err := promptApplicationCredential(cmd, &project, &id, &secret, func(survey.Prompt, interface{}, ...survey.AskOpt) error { calls++; return cancelled })
	if !errors.Is(err, cancelled) || calls != 1 || project != "" || id != "" || secret != "" {
		t.Fatal("continued after cancellation")
	}
}

func TestCredentialAddNoninteractiveRequiresMissingFlags(t *testing.T) {
	previous := cli.Opts
	t.Cleanup(func() { cli.Opts = previous })
	cli.Opts.NoInteraction = true
	for _, args := range [][]string{nil, {"--project", "mwcli"}, {"--project", "mwcli", "--id", "example-id"}} {
		_, err := executeTestCommand(NewAuthAddCmd(), args...)
		if err == nil || !strings.Contains(err.Error(), "--secret") || !strings.Contains(err.Error(), "https://horizon.wikimedia.org/") {
			t.Fatalf("missing guidance: %v", err)
		}
	}
}
