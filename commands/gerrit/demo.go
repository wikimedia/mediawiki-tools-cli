package gerrit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

const patchDemoURL = "https://patchdemo.wmcloud.org/"

func newDemoCmd(openURL func(string) error) *cobra.Command {
	var patchset int
	var create, noBrowser bool
	cmd := &cobra.Command{
		Use:     "demo [change]",
		Short:   "Find Patch Demo wikis for a Gerrit change",
		Long:    "List ready Patch Demo wikis and compare their applied patchsets with the selected public Gerrit patchset. If you omit the change, use the uploaded patchset matching Git HEAD. With --new, open the Patch Demo form and print the exact patchset URL to paste.",
		Example: "demo\ndemo 1370181 --patchset 4\ndemo 1370181 --new --no-browser",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if noBrowser && !create {
				return fmt.Errorf("--no-browser is only used with --new")
			}
			client, target, err := preparePublicChange(cmd.Context(), cmd, args, patchset)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), gerritRequestTimeout)
			defer cancel()
			change, selected, err := target.readChange(ctx, client)
			if err != nil {
				return err
			}
			if create {
				if !validGerritProject(change.Project) {
					return fmt.Errorf("Gerrit returned an invalid project name")
				}
				patchURL := url.URL{
					Scheme: "https",
					Host:   gerritSSHHost,
					Path:   fmt.Sprintf("/r/c/%s/+/%d/%d", change.Project, target.change, selected),
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Patch Demo form: %s\nPaste this in the patches field: %s\n", patchDemoURL, &patchURL)
				if !noBrowser && !cli.Opts.NoInteraction {
					if err := openURL(patchDemoURL); err != nil {
						return fmt.Errorf("could not open Patch Demo; open the URL above manually or use --no-browser: %w", err)
					}
				}
				return nil
			}
			demoCtx, stop := context.WithTimeout(cmd.Context(), gerritRequestTimeout)
			defer stop()
			wikis, err := lookupPatchDemoWikis(demoCtx, target.change)
			if err != nil {
				return fmt.Errorf("look up Patch Demo wikis: %w", err)
			}
			printPatchDemoWikis(cmd, target.change, selected, wikis)
			return nil
		},
	}
	cmd.Flags().BoolVar(&create, "new", false, "Open the create form and print the patchset URL")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "With --new, print the links without opening a browser")
	return publicPatchsetCommand(cmd, &patchset)
}

type patchDemoWiki struct {
	URL     string   `json:"url"`
	Patches []string `json:"patches"`
}

func lookupPatchDemoWikis(ctx context.Context, change int) ([]patchDemoWiki, error) {
	params := url.Values{"action": {"findwikis"}, "change": {strconv.Itoa(change)}}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, patchDemoURL+"api.php?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	client := cli.WithHTTPUserAgent(&http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	})
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Patch Demo returned HTTP %d", response.StatusCode)
	}
	const limit = 4 << 20
	body, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if len(body) > limit {
		return nil, fmt.Errorf("Patch Demo response exceeds 4 MiB")
	}
	var wikis []patchDemoWiki
	if err := json.Unmarshal(body, &wikis); err != nil {
		return nil, fmt.Errorf("decode Patch Demo response: %w", err)
	}
	return wikis, nil
}

func printPatchDemoWikis(cmd *cobra.Command, change, selected int, wikis []patchDemoWiki) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Change %d, patchset %d\n", change, selected)
	table := output.NewTable([]any{"PATCHSET(S)", "STATUS", "WIKI"}, nil)
	prefix := strconv.Itoa(change) + ","
	for _, wiki := range wikis {
		address, err := url.Parse(wiki.URL)
		if err != nil || address.Scheme != "https" || address.User != nil || address.Port() != "" ||
			(address.Hostname() != "patchdemo.wmcloud.org" && !strings.HasSuffix(address.Hostname(), ".catalyst.wmcloud.org")) {
			continue
		}
		var patchsets []int
		for _, patch := range wiki.Patches {
			if value, ok := strings.CutPrefix(patch, prefix); ok {
				if n, err := strconv.Atoi(value); err == nil && n > 0 {
					patchsets = append(patchsets, n)
				}
			}
		}
		if len(patchsets) == 0 {
			continue
		}
		slices.Sort(patchsets)
		patchsets = slices.Compact(patchsets)
		status := "MIXED"
		switch {
		case len(patchsets) == 1 && patchsets[0] == selected:
			status = "MATCH"
		case patchsets[len(patchsets)-1] < selected:
			status = "OLDER"
		case patchsets[0] > selected:
			status = "NEWER"
		}
		values := make([]string, 0, len(patchsets))
		for _, number := range patchsets {
			values = append(values, strconv.Itoa(number))
		}
		table.AddRowS(strings.Join(values, ", "), status, address.String())
	}
	if len(table.Rows) == 0 {
		fmt.Fprintln(out, "No ready Patch Demo wikis found.")
		fmt.Fprintf(out, "Open the create form with %s gerrit demo %d --patchset %d --new\n", cmd.Root().Name(), change, selected)
		return
	}
	table.Print(out)
}
