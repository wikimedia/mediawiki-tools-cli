package wiki

import (
	"fmt"

	mwclient "cgt.name/pkg/go-mwclient"
	"cgt.name/pkg/go-mwclient/params"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

func NewWikiExtWikibaseItemPutCmd() *cobra.Command {
	var (
		summary  string
		label    string
		property string
		value    string
		bot      bool
		expectID string
		dryRun   bool
	)

	cmd := &cobra.Command{
		Use:     "put",
		Short:   "Create a Wikibase item with one string claim",
		Long:    "Create a new item with an English label and one string-valued claim. This command always creates an item; it does not update an existing one.",
		Example: "  put --profile wikibase-world/addshore --label 'Example' --property P1 --value 'Example value' --dry-run",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if wiki == "" {
				return fmt.Errorf("wiki is not set")
			}
			if property == "" {
				return fmt.Errorf("property is not set")
			}

			data := map[string]interface{}{
				"labels": map[string]interface{}{
					"en": map[string]string{
						"language": "en",
						"value":    label,
					},
				},
				"claims": []interface{}{
					map[string]interface{}{
						"mainsnak": map[string]interface{}{
							"snaktype": "value",
							"property": property,
							"datavalue": map[string]string{
								"value": value,
								"type":  "string",
							},
						},
						"type": "statement",
						"rank": "normal",
					},
				},
			}
			dataJSON := mustMarshalJSON(data)

			if dryRun {
				cmd.Println("Dry run mode: Creating Wikibase item with the following parameters:")
				cmd.Printf("wiki: %s, user: %s, summary: %s, label: %s, property: %s, value: %s, expect-id: %s, bot: %t\n",
					wiki, wikiUser, summary, label, property, value, expectID, bot)
				cmd.Printf("data: %s\n", dataJSON)
				return nil
			}

			w, err := mwclient.New(normalizeWiki(wiki), cli.HTTPUserAgent)
			if err != nil {
				return err
			}
			if err := loginIfCredentialsProvided(w); err != nil {
				return fmt.Errorf("wiki authentication failed: %w", err)
			}

			editParams := params.Values{
				"new":     "item",
				"data":    dataJSON,
				"summary": summary,
			}
			if wikiAnon {
				editParams["token"] = anonCSRFToken
			}
			if bot {
				editParams["bot"] = "1"
			}

			id, err := wbEditEntity(w, editParams)
			if err != nil {
				return err
			}

			if expectID != "" && id != expectID {
				return fmt.Errorf("item %s was created, but expected ID was %q; do not retry creation", id, expectID)
			}
			cmd.Println(id)
			return nil
		},
	}

	cmd.Flags().StringVar(&summary, "summary", cli.DisplayName+": Wikibase item update", "Summary of the edit")
	cmd.Flags().StringVar(&label, "label", "first", "English label for the item")
	cmd.Flags().StringVar(&property, "property", "P1", "Property ID for the string-valued claim")
	cmd.Flags().StringVar(&value, "value", "hello", "String value for the claim")
	cmd.Flags().BoolVar(&bot, "bot", false, "Mark the edit as a bot edit")
	cmd.Flags().StringVar(&expectID, "expect-id", "", "Fail unless the created item ID matches this value")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the proposed item without creating it")

	return cmd
}
