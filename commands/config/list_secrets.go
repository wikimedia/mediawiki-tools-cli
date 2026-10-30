package config

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

func NewListSecretsCmd() *cobra.Command {
	var asJSON bool
	var check bool
	var verify bool
	var details bool
	cmd := &cobra.Command{
		Use: "list [KEY...]", Aliases: []string{"ls"}, Short: "List stored credential metadata without revealing secrets",
		Long: "List credentials and provider references managed by this CLI, including plaintext config values and the local KeePassXC pairing file. Supply keys to inspect only those entries. Displays identities and locations, never secret values. By default external entries are not contacted or unlocked. Use --check to compare plaintext and vault values, and --verify to authenticate each copy separately against its service. Both may prompt in KeePassXC. Neither changes saved credentials. GitLab's separate credential store, external phab.cfg files, and unrelated vault entries are not enumerated.",
		RunE: func(cmd *cobra.Command, args []string) error {
			entries := config.SecretInventory()
			if len(args) > 0 {
				wanted := map[string]bool{}
				for _, key := range args {
					wanted[key] = false
				}
				filtered := make([]config.SecretInfo, 0)
				for _, entry := range entries {
					if _, ok := wanted[entry.Key]; ok {
						filtered = append(filtered, entry)
						wanted[entry.Key] = true
					}
				}
				for _, key := range args {
					if !wanted[key] {
						return fmt.Errorf("no stored credential or reference found for %s", secretColumn(key))
					}
				}
				entries = filtered
			}
			if check || verify {
				if _, err := fmt.Fprintln(cmd.ErrOrStderr(), "Reading requested credentials; KeePassXC may request access approval."); err != nil {
					return err
				}
				config.InspectSecretInventory(cmd.Context(), entries, check, verify)
			}
			if asJSON {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(entries)
			}
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), "Configuration: "+config.Path()); err != nil {
				return err
			}
			if len(entries) == 0 {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "No stored credentials or provider references found.")
				return err
			}
			table := output.NewTable([]interface{}{"KEY", "PROVIDER", "COMPARE", "AUTH"}, nil)
			for _, entry := range entries {
				providerColor := color.FgCyan
				if entry.Provider == "plaintext" || entry.Provider == "pairing-file" {
					providerColor = color.FgYellow
				}
				comparison := strings.ReplaceAll(entry.Comparison, "-", " ")
				comparisonColor := color.FgYellow
				switch entry.Comparison {
				case "match":
					comparisonColor = color.FgGreen
				case "different":
					comparisonColor = color.FgRed
				}
				authColor := color.FgYellow
				switch entry.Validation {
				case "valid":
					authColor = color.FgGreen
				case "invalid", "wrong-account":
					authColor = color.FgRed
				}
				table.AddRow(secretColumn(entry.Key), output.StyledCell{Text: secretColumn(entry.Provider), Color: providerColor}, output.StyledCell{Text: secretColumn(comparison), Color: comparisonColor}, output.StyledCell{Text: secretColumn(entry.Validation), Color: authColor})
			}
			table.Print(cmd.OutOrStdout())
			if details {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), "\nCredential details:"); err != nil {
					return err
				}
				for _, entry := range entries {
					if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  %s (%s)\n", secretColumn(entry.Key), secretColumn(entry.Provider)); err != nil {
						return err
					}
					for _, field := range []struct{ label, value string }{
						{"Service", entry.Service},
						{"Account", entry.Account},
						{"Endpoint", entry.Endpoint},
						{"Status", entry.Status},
						{"Location", entry.Location},
					} {
						if field.value != "" {
							if _, err := fmt.Fprintf(cmd.OutOrStdout(), "    %s: %s\n", field.label, secretColumn(field.value)); err != nil {
								return err
							}
						}
					}
					if entry.EntryID != "" {
						if _, err := fmt.Fprintf(cmd.OutOrStdout(), "    Database: %s\n    Entry: %s\n", secretColumn(entry.DatabaseID), secretColumn(entry.EntryID)); err != nil {
							return err
						}
					}
				}
			} else {
				for _, entry := range entries {
					if entry.Validation == "wrong-account" || strings.Contains(entry.Status, "mismatch") || strings.Contains(entry.Status, "invalid reference") || strings.Contains(entry.Status, "missing") || strings.Contains(entry.Status, "relink") {
						if _, err := fmt.Fprintf(cmd.OutOrStdout(), "\n%s (%s): %s\n", secretColumn(entry.Key), secretColumn(entry.Provider), secretColumn(entry.Status)); err != nil {
							return err
						}
					}
				}
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), "\nUse --details for accounts, endpoints and storage locations."); err != nil {
					return err
				}
			}
			footer := "External credentials were not retrieved or verified."
			if check {
				footer = "Comparison checks equality only; unavailable entries could not be read. Remote service validity was not checked."
			}
			if verify {
				footer = "AUTH checks each stored copy against its service. Unavailable means inconclusive; expired OAuth tokens are not refreshed. Credentials were not changed."
			}
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), "\n"+footer+" GitLab and external phab.cfg stores are managed separately."); err != nil {
				return err
			}
			if check || verify {
				return printSecretSuggestions(cmd.OutOrStdout(), secretSuggestions(entries, cmd.Root().Name()))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output credential metadata as JSON")
	cmd.Flags().BoolVar(&details, "details", false, "Show account, endpoint, status and storage details below the summary")
	cmd.Flags().BoolVar(&check, "check", false, "Compare plaintext copies with their linked vault values without displaying them (may request vault approval)")
	cmd.Flags().BoolVar(&verify, "verify", false, "Authenticate each stored copy with its service without changing credentials (may request vault approval)")
	return cmd
}

type secretSuggestion struct {
	reason, command string
}

func printSecretSuggestions(writer io.Writer, suggestions []secretSuggestion) error {
	if len(suggestions) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(writer, "\n"+output.StyledText(writer, "Suggested commands (nothing run automatically):", color.Bold)); err != nil {
		return err
	}
	for _, suggestion := range suggestions {
		if _, err := fmt.Fprintf(writer, "\n  %s\n  %s\n", output.StyledText(writer, "# "+suggestion.reason, color.Faint), output.StyledText(writer, "$ "+suggestion.command, color.FgCyan, color.Bold)); err != nil {
			return err
		}
	}
	return nil
}

func secretSuggestions(entries []config.SecretInfo, executable string) []secretSuggestion {
	plaintext := map[string]config.SecretInfo{}
	for _, entry := range entries {
		if entry.Source == "plaintext-config" {
			plaintext[entry.Key] = entry
		}
	}
	var suggestions []secretSuggestion
	for _, entry := range entries {
		if entry.Source != "credential-reference" || (entry.Provider != "keyring" && entry.Provider != "keepassxc") {
			continue
		}
		plain, hasPlain := plaintext[entry.Key]
		// Existing migration commands preserve account bindings. Never offer an
		// overwrite or cleanup command when those bindings differ.
		if hasPlain && (entry.Account != plain.Account || entry.Endpoint != plain.Endpoint) {
			continue
		}
		base := shellWord(executable) + " config secrets "
		migration := base + "migrate " + shellWord(entry.Key) + " --provider " + entry.Provider
		switch {
		case hasPlain && entry.Comparison == "match":
			suggestions = append(suggestions, secretSuggestion{"Remove duplicate plaintext; rechecks equality first.", migration})
		case hasPlain && plain.Validation == "valid" && entry.Validation == "invalid":
			suggestions = append(suggestions, secretSuggestion{"Keep valid plaintext: replace the vault value, then remove plaintext after verification.", migration + " --replace"})
		case hasPlain && entry.Comparison == "different" && (entry.Validation == "" || plain.Validation == ""):
			suggestions = append(suggestions, secretSuggestion{"Check which copy authenticates before choosing one.", base + "list " + shellWord(entry.Key) + " --check --verify"})
		case !hasPlain && entry.Key == "gerrit.password" && entry.Validation == "invalid":
			suggestions = append(suggestions, secretSuggestion{"Log in again to replace the invalid Gerrit credential.", shellWord(executable) + " gerrit auth login --provider " + entry.Provider})
		}
	}
	return suggestions
}

// Commands are displayed for pasting into a shell, so quote metadata rather
// than interpreting punctuation or substituting account names into commands.
func shellWord(value string) string {
	if value != "" && !strings.ContainsAny(value, " \t\r\n'\"`$;&|<>(){}[]*?!\\#~") && !strings.ContainsFunc(value, unicode.IsControl) {
		return value
	}
	return "'" + strings.ReplaceAll(secretColumn(value), "'", "'\"'\"'") + "'"
}

func secretColumn(value string) string {
	if value == "" {
		return "-"
	}
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, value)
}
