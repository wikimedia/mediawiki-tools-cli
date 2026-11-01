package wiki

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiapi"
)

func newActionRefreshCmd() *cobra.Command {
	return &cobra.Command{
		Use: "refresh [MODULE...]", Short: "Refresh public Action API schemas and site capabilities",
		Long:              "Refresh cached public parameter schemas and site capabilities without credentials. With no modules, fetch all action and query modules. Tab completion reads this cache offline; raw requests do not require discovery.",
		Example:           "  refresh\n  refresh main query query+revisions wbgetentities",
		ValidArgsFunction: completeAPIModules,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
			defer cancel()
			wikiContext = ctx
			client, err := wikiAPIClient(true)
			if err != nil {
				return err
			}
			modules, err := (wikiapi.Discovery{Client: client}).RefreshAction(ctx, args)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Cached %d public Action modules for %s\n", len(modules), client.Endpoint)
			return nil
		},
	}
}

func newActionDescribeCmd() *cobra.Command {
	out := apiOutput{}
	cmd := &cobra.Command{
		Example: "  describe query+revisions",
		Use:     "describe MODULE", Short: "Read cached Action module parameters (refresh explicitly to update)", Args: cobra.ExactArgs(1),
		ValidArgsFunction: completeAPIModules,
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := completionDiscovery()
			if err != nil {
				return err
			}
			module, ok := d.CachedModule(args[0])
			if !ok {
				return fmt.Errorf("module is not cached; run wiki api refresh %s", args[0])
			}
			data, err := json.Marshal(module)
			if err != nil {
				return err
			}
			return out.print(cmd, data)
		},
	}
	out.flags(cmd)
	return cmd
}

func newRESTRefreshCmd() *cobra.Command {
	return &cobra.Command{
		Use: "refresh", Short: "Refresh available public REST OpenAPI schemas", Args: cobra.NoArgs,
		Long: "Discover and cache public OpenAPI schemas served by this wiki. Uses MediaWiki REST discovery, with an older Wikibase OpenAPI fallback. Raw REST calls work even if this wiki does not publish schemas. Never authenticates or follows external specification URLs.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
			defer cancel()
			wikiContext = ctx
			client, err := wikiAPIClient(true)
			if err != nil {
				return err
			}
			names, err := (wikiapi.Discovery{Client: client}).RefreshREST(ctx)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Cached %d public REST schemas for %s\n", len(names), client.Endpoint)
			return nil
		},
	}
}

func newRESTDescribeCmd() *cobra.Command {
	out := apiOutput{}
	cmd := &cobra.Command{
		Example: "  describe wikibase/v1",
		Use:     "describe [MODULE]", Short: "Read the cached REST module index or a module's OpenAPI document", Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := completionDiscovery()
			if err != nil {
				return err
			}
			var value interface{}
			if len(args) == 0 {
				index, ok := d.CachedRESTIndex()
				if !ok {
					return fmt.Errorf("REST schemas are not cached; run wiki rest refresh")
				}
				value = index
			} else {
				spec, ok := d.CachedREST(args[0])
				if !ok {
					return fmt.Errorf("REST module is not cached; run wiki rest refresh")
				}
				value = spec
			}
			data, err := json.Marshal(value)
			if err != nil {
				return err
			}
			return out.print(cmd, data)
		},
	}
	out.flags(cmd)
	return cmd
}

// Completion deliberately resolves only public metadata. It does not call
// prepareWikiProfile, open a credential, refresh a token or contact a wiki.
func completionDiscovery() (wikiapi.Discovery, error) {
	endpoint := wiki
	if endpoint == "" {
		profile, err := selectedWikiProfile(wikiProfile)
		if err != nil {
			return wikiapi.Discovery{}, err
		}
		endpoint = profile.Wiki
	}
	client, err := wikiapi.New(endpoint, &http.Client{})
	return wikiapi.Discovery{Client: client}, err
}

// Keep schema descriptions on one line for shell completion protocols.
func completionDescription(value string) string { return strings.Join(strings.Fields(value), " ") }
