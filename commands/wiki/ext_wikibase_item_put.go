package wiki

import (
	"fmt"

	mwclient "cgt.name/pkg/go-mwclient"
	"cgt.name/pkg/go-mwclient/params"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
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
		Use:   "put",
		Short: "Create a Wikibase item with one string claim",
		Run: func(cmd *cobra.Command, args []string) {
			if wiki == "" {
				logrus.Fatal("wiki is not set")
			}
			if property == "" {
				logrus.Fatal("property is not set")
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
				fmt.Println("Dry run mode: Creating Wikibase item with the following parameters:")
				fmt.Printf("wiki: %s, user: %s, summary: %s, label: %s, property: %s, value: %s, expect-id: %s, bot: %t\n",
					wiki, wikiUser, summary, label, property, value, expectID, bot)
				fmt.Printf("data: %s\n", dataJSON)
				return
			}

			w, err := mwclient.New(normalizeWiki(wiki), "mwcli")
			if err != nil {
				panic(err)
			}
			defaultErrorHandling().handle(loginIfCredentialsProvided(w))

			editParams := params.Values{
				"new":     "item",
				"data":    dataJSON,
				"summary": summary,
			}
			if bot {
				editParams["bot"] = "1"
			}

			id, err := wbEditEntity(w, editParams)
			if err != nil {
				panic(err)
			}

			if expectID != "" && id != expectID {
				logrus.Fatalf("expected item id %q, got %q", expectID, id)
			}
			fmt.Println(id)
		},
	}

	cmd.Flags().StringVar(&summary, "summary", "mwcli wikibase item put", "Summary of the edit")
	cmd.Flags().StringVar(&label, "label", "first", "English label for the item")
	cmd.Flags().StringVar(&property, "property", "P1", "Property id used for the claim")
	cmd.Flags().StringVar(&value, "value", "hello", "String value for the claim")
	cmd.Flags().BoolVar(&bot, "bot", false, "Bot edit")
	cmd.Flags().StringVar(&expectID, "expect-id", "", "If set, fail unless created item id matches this value")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "If set, only print the action that would be performed")

	return cmd
}
