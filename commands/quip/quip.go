package quip

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/lookpath"
)

type quote struct {
	ID   string `json:"id"`
	Text string `json:"text"`
	URL  string `json:"url"`
}

func NewQuipCmd() *cobra.Command {
	client := cli.NewHTTPClient()
	client.Timeout = 30 * time.Second
	return newQuipCmd(client, "https://bash.toolforge.org")
}

// NewLegacyCmd preserves the original hidden top-level command.
func NewLegacyCmd() *cobra.Command {
	cmd := NewQuipCmd()
	cmd.Hidden = true
	return cmd
}

func newQuipCmd(client *http.Client, baseURL string) *cobra.Command {
	var noFun, link bool
	var output string
	run := func(cmd *cobra.Command, path string) error {
		if output != "text" && output != "json" {
			return fmt.Errorf("invalid --output %q (choose text or json)", output)
		}
		req, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, baseURL+path, nil)
		if err != nil {
			return fmt.Errorf("create quip request: %w", err)
		}
		req.Header.Set("User-Agent", cli.HTTPUserAgent())
		res, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("request quip: %w", err)
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			b, err := io.ReadAll(io.LimitReader(res.Body, 4096))
			if err != nil {
				return fmt.Errorf("read quip response: %w", err)
			}
			return fmt.Errorf("quip service returned HTTP %d: %s", res.StatusCode, string(b))
		}
		doc, err := goquery.NewDocumentFromReader(res.Body)
		if err != nil {
			return fmt.Errorf("parse quip response: %w", err)
		}
		quotes := []quote{}
		doc.Find(".quote").Each(func(_ int, s *goquery.Selection) {
			href := s.Find(`a[href^="/quip/"]`).First().AttrOr("href", "")
			s.Find(".nav").Remove()
			s.Find("br").ReplaceWithHtml("\n")
			text := strings.TrimSpace(s.Text())
			if href != "" && text != "" {
				quotes = append(quotes, quote{ID: strings.TrimPrefix(href, "/quip/"), Text: text, URL: baseURL + href})
			}
		})
		if len(quotes) == 0 && cmd.Name() != "search" {
			return fmt.Errorf("quip service returned no quips")
		}
		if output == "json" {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(quotes)
		}
		if len(quotes) == 0 {
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "No quips found.")
			return err
		}
		for i, q := range quotes {
			if i > 0 {
				if _, err = fmt.Fprintln(cmd.OutOrStdout()); err != nil {
					return err
				}
			}
			text := q.Text
			if !noFun {
				if lookpath.HasExecutable("cowsay") {
					cow := exec.CommandContext(cmd.Context(), "cowsay", "-n")
					cow.Stdin = strings.NewReader(text)
					cow.Stderr = cmd.ErrOrStderr()
					data, err := cow.Output()
					if err != nil {
						return fmt.Errorf("display quip: %w", err)
					}
					text = strings.TrimRight(string(data), "\n")
				}
				if lookpath.HasExecutable("lolcat") {
					rainbow := exec.CommandContext(cmd.Context(), "lolcat")
					rainbow.Stdin = strings.NewReader(text + "\n")
					rainbow.Stdout = cmd.OutOrStdout()
					rainbow.Stderr = cmd.ErrOrStderr()
					if err := rainbow.Run(); err != nil {
						return fmt.Errorf("display quip: %w", err)
					}
				} else if _, err = fmt.Fprintln(cmd.OutOrStdout(), text); err != nil {
					return err
				}
			} else if _, err = fmt.Fprintln(cmd.OutOrStdout(), text); err != nil {
				return err
			}
			if link {
				if _, err = fmt.Fprintln(cmd.OutOrStdout(), q.URL); err != nil {
					return err
				}
			}
		}
		return nil
	}
	cmd := &cobra.Command{
		Use: "quip [id]", Short: "Read quips from bash.toolforge.org",
		Long: "Read a random Wikimedia quip, or retrieve a particular quip by its ID. Search and browse top quips with the subcommands. JSON output contains an array of IDs, text and source URLs and skips display effects.",
		Args: cobra.MaximumNArgs(1),
		Example: cobrautil.NormalizeExample(`quip
quip AU7VVEHK6snAnmqnK_uZ --no-fun --link
quip --output json`),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/random"
			if len(args) > 0 {
				if !regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(args[0]) {
					return fmt.Errorf("invalid quip ID %q", args[0])
				}
				path = "/quip/" + url.PathEscape(args[0])
			}
			return run(cmd, path)
		},
	}
	cmd.PersistentFlags().BoolVarP(&noFun, "no-fun", "n", false, "Disable cowsay and lolcat display effects")
	cmd.PersistentFlags().BoolVar(&link, "link", false, "Include the source link with each quip")
	cmd.PersistentFlags().StringVar(&output, "output", "text", "Output format: text or json")
	var page int
	search := &cobra.Command{Use: "search <query>", Short: "Search Wikimedia quips", Long: "Search quips by text. Pages follow the service's results; --page starts at 1.", Args: cobra.MinimumNArgs(1), Example: cobrautil.NormalizeExample(`search deployment --no-fun
search "unit tests" --page 2 --output json`), RunE: func(cmd *cobra.Command, args []string) error {
		if page < 1 {
			return fmt.Errorf("--page must be at least 1")
		}
		query := url.Values{"q": {strings.Join(args, " ")}, "p": {strconv.Itoa(page - 1)}}
		return run(cmd, "/search?"+query.Encode())
	}}
	search.Flags().IntVar(&page, "page", 1, "Result page to request (starting at 1)")
	top := &cobra.Command{Use: "top", Short: "Read top Wikimedia quips", Long: "Read the top quips returned by bash.toolforge.org.", Args: cobra.NoArgs, Example: cobrautil.NormalizeExample(`top --no-fun --link`), RunE: func(cmd *cobra.Command, _ []string) error { return run(cmd, "/top") }}
	cmd.AddCommand(search, top)
	return cmd
}
