package phabricator

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

type phabPaste struct {
	ID     int    `json:"id"`
	PHID   string `json:"phid"`
	Fields struct {
		Title        string `json:"title"`
		URI          string `json:"uri"`
		Language     string `json:"language"`
		Status       string `json:"status"`
		AuthorPHID   string `json:"authorPHID"`
		DateCreated  int64  `json:"dateCreated"`
		DateModified int64  `json:"dateModified"`
	} `json:"fields"`
	Attachments struct {
		Content struct {
			Content string `json:"content"`
		} `json:"content"`
	} `json:"attachments"`
}

func (c *conduitClient) getPaste(ref string) (*phabPaste, error) {
	parsed, err := parseObjectReference(ref)
	if err != nil || parsed.Ref[0] != 'P' {
		return nil, fmt.Errorf("expected a paste reference such as P123")
	}
	if err := parsed.checkEndpoint(c.baseURL); err != nil {
		return nil, err
	}
	var id int
	_, _ = fmt.Sscanf(parsed.Ref, "P%d", &id)
	result, err := c.post("paste.search", map[string]interface{}{
		"constraints": map[string]interface{}{"ids": []int{id}},
		"attachments": map[string]bool{"content": true},
	})
	if err != nil {
		return nil, err
	}
	var found struct {
		Data []phabPaste `json:"data"`
	}
	if err := json.Unmarshal(result, &found); err != nil {
		return nil, fmt.Errorf("decode paste search: %w", err)
	}
	if len(found.Data) != 1 || found.Data[0].ID != id || found.Data[0].PHID == "" {
		return nil, fmt.Errorf("paste %s was not found or is not visible to this profile", parsed.Ref)
	}
	// Distinguish an empty paste from an omitted content attachment.
	var content struct {
		Data []struct {
			Attachments struct {
				Content *struct {
					Content *string `json:"content"`
				} `json:"content"`
			} `json:"attachments"`
		} `json:"data"`
	}
	if err := json.Unmarshal(result, &content); err != nil {
		return nil, fmt.Errorf("decode paste content: %w", err)
	}
	if content.Data[0].Attachments.Content == nil || content.Data[0].Attachments.Content.Content == nil {
		return nil, fmt.Errorf("Phabricator did not return paste content")
	}
	return &found.Data[0], nil
}

func (c *conduitClient) editPaste(ref string, transactions []map[string]interface{}) (int, error) {
	params := map[string]interface{}{"transactions": transactions}
	if ref != "" {
		parsed, err := parseObjectReference(ref)
		if err != nil || parsed.Ref[0] != 'P' {
			return 0, fmt.Errorf("expected a paste reference such as P123")
		}
		if err := parsed.checkEndpoint(c.baseURL); err != nil {
			return 0, err
		}
		ref = parsed.Ref
		params["objectIdentifier"] = ref
	}
	result, err := c.post("paste.edit", params)
	if err != nil {
		return 0, err
	}
	var edited struct {
		Object struct {
			ID int `json:"id"`
		} `json:"object"`
	}
	if err := json.Unmarshal(result, &edited); err != nil {
		return 0, fmt.Errorf("decode paste edit: %w", err)
	}
	if edited.Object.ID <= 0 || (ref != "" && fmt.Sprintf("P%d", edited.Object.ID) != ref) {
		return 0, fmt.Errorf("Phabricator did not confirm the expected paste ID; check your pastes before retrying")
	}
	return edited.Object.ID, nil
}

func newPhabricatorPasteCmd(site *string) *cobra.Command {
	cmd := &cobra.Command{Use: "paste", Short: "Read, create and edit Phabricator pastes"}
	cmd.AddCommand(newPhabricatorObjectViewCmd(site, true))
	for _, editing := range []bool{false, true} {
		cmd.AddCommand(newPhabricatorPasteWriteCmd(site, editing, cli.CanAskForInput, func() (bool, error) {
			var yes bool
			err := survey.AskOne(&survey.Confirm{Message: "Save this Phabricator paste?", Default: false}, &yes)
			return yes, err
		}))
	}
	return cmd
}

func newPhabricatorPasteWriteCmd(site *string, editing bool, canAsk func() bool, confirm func() (bool, error)) *cobra.Command {
	var title, file, language, status, visibility string
	var yes, dryRun bool
	cmd := &cobra.Command{
		Use:     "create",
		Aliases: []string{"send"},
		Short:   "Create a Phabricator paste from a file or stdin",
		Long: "Preview and save a paste using the selected profile. Content must be UTF-8 text. " +
			"Use --file - for stdin. Confirm interactively or use --yes. " +
			"--dry-run previews changes without reading credentials or contacting Phabricator. " +
			"Unless --visibility is provided, Phabricator chooses the view policy; it may be public.",
		Example: "create --title 'Example output' --file output.txt\ncreate --title 'Example output' --file - --yes",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			var ref objectReference
			var err error
			if editing {
				ref, err = parseObjectReference(args[0])
				if err != nil {
					return err
				}
				if ref.Ref[0] != 'P' {
					return fmt.Errorf("expected a paste reference such as P123")
				}
			}
			transactions := []map[string]interface{}{}
			add := func(kind, value string) {
				transactions = append(transactions, map[string]interface{}{"type": kind, "value": value})
			}
			if !editing || cmd.Flags().Changed("title") {
				title = strings.TrimSpace(title)
				if title == "" {
					return fmt.Errorf("--title must not be empty")
				}
				add("title", title)
			}
			if !editing || cmd.Flags().Changed("file") {
				if file == "" {
					return fmt.Errorf("--file must name a file or - for stdin")
				}
				data, err := readTaskFile(cmd, file)
				if err != nil {
					return err
				}
				if len(data) == 0 || !utf8.Valid(data) || strings.ContainsRune(string(data), '\x00') {
					return fmt.Errorf("paste content must be nonempty UTF-8 text without NUL bytes")
				}
				add("text", string(data))
			}
			for _, field := range []struct{ flag, transaction, value string }{
				{"language", "language", language}, {"status", "status", status}, {"visibility", "view", visibility},
			} {
				if cmd.Flags().Changed(field.flag) {
					if strings.TrimSpace(field.value) == "" {
						return fmt.Errorf("--%s must not be empty", field.flag)
					}
					if field.flag == "status" && field.value != "active" && field.value != "archived" {
						return fmt.Errorf("--status must be active or archived")
					}
					add(field.transaction, field.value)
				}
			}
			if len(transactions) == 0 {
				return fmt.Errorf("specify at least one field to change")
			}
			cmd.Printf("Paste: %s\n", ref.Ref)
			for _, transaction := range transactions {
				cmd.Printf("%s: %s\n", transaction["type"], transaction["value"])
			}
			if dryRun {
				cmd.Println("Preview only; no paste saved.")
				return nil
			}
			if !yes {
				if !canAsk() {
					return fmt.Errorf("paste was not saved; use --yes to save without an interactive confirmation")
				}
				approved, err := confirm()
				if err != nil {
					return err
				}
				if !approved {
					cmd.Println("Cancelled; no paste saved.")
					return nil
				}
			}
			client, _, err := clientFromSite(*site)
			if err != nil {
				return err
			}
			if err := ref.checkEndpoint(client.baseURL); err != nil {
				return err
			}
			id, err := client.editPaste(ref.Ref, transactions)
			if err != nil {
				return err
			}
			cmd.Printf("Saved P%d: %s/P%d\n", id, strings.TrimRight(client.baseURL, "/"), id)
			return nil
		},
	}
	if editing {
		cmd.Use = "edit <P123|URL>"
		cmd.Aliases = nil
		cmd.Short = "Edit paste content, title, language or visibility"
		cmd.Example = "edit P123 --file revised.txt\nedit P123 --title 'New title' --yes\nedit P123 --status archived --dry-run"
		cmd.Args = cobra.ExactArgs(1)
	}
	cmd.Flags().StringVar(&title, "title", "", "Paste title")
	defaultFile := "-"
	if editing {
		defaultFile = ""
	}
	cmd.Flags().StringVar(&file, "file", defaultFile, "Read paste content from a file, or - for stdin")
	cmd.Flags().StringVar(&language, "language", "", "Phabricator syntax-highlighting language key")
	cmd.Flags().StringVar(&status, "status", "", "Paste status: active or archived")
	cmd.Flags().StringVar(&visibility, "visibility", "", "View policy constant or policy PHID (for example users or no-one)")
	cmd.Flags().BoolVar(&yes, "yes", false, "Save without asking for confirmation")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without accessing credentials or saving a paste")
	return cmd
}
