package wiki

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiapi"
)

func newWikiPageDiffCmd() *cobra.Command {
	out := output.Output{}
	cmd := &cobra.Command{
		Use: "diff SELECTOR [SELECTOR]", Short: "Compare page titles or revisions", Args: cobra.RangeArgs(1, 2), ValidArgsFunction: cobra.NoFileCompletions,
		Long:    "One selector shows that title/revision's latest change against its predecessor. Two selectors compare FROM to TO. Selectors accept rev:123 or bare positive revision numbers, title:Earth or ordinary nonnumeric page titles. Use title:123 for a numeric page title.\n\nText output requests the wiki's unified diff where available. Older wikis fall back to readable +/-/context lines; fallback text is not a patch file. --output html emits native diff HTML; json/jq expose the native comparison metadata and body. No pages are edited.",
		Example: "  diff Earth\n  diff rev:123 rev:124\n  diff title:Earth title:Moon --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := output.ValidateJSONFormat(out.Format); err != nil {
				return err
			}
			switch out.Type {
			case "text", "html":
				if out.Format != "" {
					return fmt.Errorf("--format applies only to json/jq output")
				}
			case "json":
			case "jq":
				if out.Format == "" {
					return fmt.Errorf("--output jq requires --format")
				}
			default:
				return fmt.Errorf("unsupported diff output format")
			}
			for _, selector := range args {
				if _, err := wikiapi.ParseDiffSelector(selector); err != nil {
					return err
				}
			}
			client, err := wikiAPIClient(false)
			if err != nil {
				return err
			}
			to := ""
			if len(args) == 2 {
				to = args[1]
			}
			diff, err := client.PageDiff(cmd.Context(), args[0], to, out.Type == "text")
			if err != nil {
				return err
			}
			if out.Type == "json" || out.Type == "jq" {
				return printResourceOutput(cmd, &out, diff)
			}
			var body string
			if json.Unmarshal(diff["body"], &body) != nil {
				return fmt.Errorf("wiki returned a non-text diff body")
			}
			if out.Type == "text" {
				body, err = wikiapi.DiffText(body)
				if err != nil {
					return err
				}
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), body)
			return err
		},
	}
	out.AddFlagsWithOpts(cmd, output.WithDefaultOutput(output.Type("text")), output.WithAdditionalTypes(output.Type("text"), output.Type("html")), output.WithFilterFlagDisabled())
	return cmd
}
