package wiki

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mcpserver"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiapi"
)

type pageInfoInput struct {
	Title           string `json:"title" jsonschema:"Title of one wiki page, for example API:Main page."`
	Wiki            string `json:"wiki,omitempty" jsonschema:"mediawiki (default) or enwiki (English Wikipedia)."`
	FollowRedirects bool   `json:"followRedirects,omitempty" jsonschema:"Resolve wiki redirects before reading metadata. Defaults to false."`
}

func readPageInfo(openClient func(pageInfoInput) (*wikiapi.Client, error)) mcpserver.Action[pageInfoInput, map[string]json.RawMessage] {
	return func(ctx context.Context, input pageInfoInput) (map[string]json.RawMessage, error) {
		if strings.TrimSpace(input.Title) == "" {
			return nil, fmt.Errorf("page title cannot be empty")
		}
		client, err := openClient(input)
		if err != nil {
			return nil, err
		}
		return client.PageInfo(ctx, input.Title, input.FollowRedirects)
	}
}

func newWikiPageInfoCmd() *cobra.Command {
	var input pageInfoInput
	out := apiOutput{}
	run := readPageInfo(func(pageInfoInput) (*wikiapi.Client, error) { return wikiAPIClient(false) })
	cmd := &cobra.Command{
		Annotations: commandmeta.Annotations(mcpserver.ReadHints(commandmeta.DataPublic, commandmeta.DataPrivateMetadata)),
		Use:         "info TITLE", Short: "Read page identity, URLs and protection metadata", Args: cobra.ExactArgs(1), ValidArgsFunction: cobra.NoFileCompletions,
		Long:    "Read metadata for one page without downloading its source. Redirect pages are inspected as pages by default; --follow-redirects asks the selected wiki to resolve them. Missing or invalid titles return an error.",
		Example: "  info Earth\n  info Earth --output jq --format .fullurl",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := out.validate(); err != nil {
				return err
			}
			input.Title = args[0]
			page, err := run(cmd.Context(), input)
			if err != nil {
				return err
			}
			if !out.raw {
				return printResourceOutput(cmd, &out.Output, page)
			}
			data, err := json.Marshal(page)
			if err != nil {
				return err
			}
			return out.print(cmd, data)
		},
	}
	cmd.Flags().BoolVar(&input.FollowRedirects, "follow-redirects", false, "Resolve redirects within the selected wiki before reading metadata")
	out.flags(cmd)
	return cmd
}
