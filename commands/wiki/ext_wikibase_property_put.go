package wiki

import (
	"fmt"

	mwclient "cgt.name/pkg/go-mwclient"
	"cgt.name/pkg/go-mwclient/params"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

func NewWikiExtWikibasePropertyPutCmd() *cobra.Command {
	var (
		summary  string
		label    string
		datatype string
		bot      bool
		expectID string
		dryRun   bool
	)

	cmd := &cobra.Command{
		Use:     "put",
		Short:   "Create a Wikibase property",
		Long:    "Create a new property with an English label and the selected datatype. This command always creates a property; it does not update an existing one.",
		Example: "  put --profile wikibase-world/addshore --label 'Example property' --datatype string --dry-run",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if wiki == "" {
				return fmt.Errorf("wiki is not set")
			}

			data := map[string]interface{}{
				"labels": map[string]interface{}{
					"en": map[string]string{
						"language": "en",
						"value":    label,
					},
				},
				"datatype": datatype,
			}
			dataJSON := mustMarshalJSON(data)

			if dryRun {
				cmd.Println("Dry run mode: Creating Wikibase property with the following parameters:")
				cmd.Printf("wiki: %s, user: %s, summary: %s, datatype: %s, label: %s, expect-id: %s, bot: %t\n",
					wiki, wikiUser, summary, datatype, label, expectID, bot)
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
				"new":     "property",
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
				return fmt.Errorf("property %s was created, but expected ID was %q; do not retry creation", id, expectID)
			}
			cmd.Println(id)
			return nil
		},
	}

	cmd.Flags().StringVar(&summary, "summary", cli.DisplayName+": Wikibase property update", "Summary of the edit")
	cmd.Flags().StringVar(&label, "label", "text", "English label for the property")
	cmd.Flags().StringVar(&datatype, "datatype", "string", "Property datatype")
	cmd.Flags().BoolVar(&bot, "bot", false, "Mark the edit as a bot edit")
	cmd.Flags().StringVar(&expectID, "expect-id", "", "Fail unless the created property ID matches this value")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the proposed property without creating it")

	return cmd
}
