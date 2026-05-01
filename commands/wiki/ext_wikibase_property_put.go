package wiki

import (
	"fmt"

	mwclient "cgt.name/pkg/go-mwclient"
	"cgt.name/pkg/go-mwclient/params"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
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
		Use:   "put",
		Short: "Create a Wikibase property",
		Run: func(cmd *cobra.Command, args []string) {
			if wiki == "" {
				logrus.Fatal("wiki is not set")
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
				fmt.Println("Dry run mode: Creating Wikibase property with the following parameters:")
				fmt.Printf("wiki: %s, user: %s, summary: %s, datatype: %s, label: %s, expect-id: %s, bot: %t\n",
					wiki, wikiUser, summary, datatype, label, expectID, bot)
				fmt.Printf("data: %s\n", dataJSON)
				return
			}

			w, err := mwclient.New(normalizeWiki(wiki), "mwcli")
			if err != nil {
				panic(err)
			}
			defaultErrorHandling().handle(loginIfCredentialsProvided(w))

			editParams := params.Values{
				"new":     "property",
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
				logrus.Fatalf("expected property id %q, got %q", expectID, id)
			}
			fmt.Println(id)
		},
	}

	cmd.Flags().StringVar(&summary, "summary", "mwcli wikibase property put", "Summary of the edit")
	cmd.Flags().StringVar(&label, "label", "text", "English label for the property")
	cmd.Flags().StringVar(&datatype, "datatype", "string", "Property datatype")
	cmd.Flags().BoolVar(&bot, "bot", false, "Bot edit")
	cmd.Flags().StringVar(&expectID, "expect-id", "", "If set, fail unless created property id matches this value")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "If set, only print the action that would be performed")

	return cmd
}
