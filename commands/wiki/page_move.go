package wiki

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	mwclient "cgt.name/pkg/go-mwclient"
	"cgt.name/pkg/go-mwclient/params"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
)

type wikiMovePage struct {
	ID             int64  `json:"pageid"`
	Namespace      int    `json:"ns"`
	Title          string `json:"title"`
	Missing        bool   `json:"missing"`
	Invalid        bool   `json:"invalid"`
	InvalidReason  string `json:"invalidreason"`
	Redirect       bool   `json:"redirect"`
	AssociatedPage string `json:"associatedpage"`
}

type wikiMove struct {
	From              wikiMovePage
	To                string
	DestinationExists bool
	AlreadyMoved      bool
}

func NewWikiPageMoveCmd() *cobra.Command {
	var prefix, dryRun, moveTalk, bot bool
	var reason string
	var interval time.Duration
	var limit int
	cmd := &cobra.Command{
		Use:   "move FROM TO",
		Short: "Move wiki pages while preserving history and leaving redirects",
		Long: "Move a page to a new title, preserving its history and leaving a redirect at the old title. Pages already redirecting directly to their intended existing destination are skipped. Other existing destinations are refused." +
			"\n\nWith --prefix, FROM and TO are literal title prefixes: every matching page keeps its suffix. Omit wildcards. Prefix moves include matching redirects and work across namespaces. Use --dry-run to inspect the complete plan without moving pages." +
			"\n\n--limit N selects at most N pending source pages, sorted by title. Already-moved pages do not count towards the limit, so rerunning a limited batch advances to the next pages. The default, 0, selects all pending matches. The same limit applies to dry runs. Associated talk pages requested with --move-talk are additional to this limit. For an already-moved content page, an outstanding talk-page move must be handled explicitly." +
			"\n\nMediaWiki automatically bot-marks move log entries when the authenticated session has the bot right. --bot requires that right on every request after login, including during a dry run, and fails if it is unavailable. Bot-marked entries can still be shown in Recent Changes. Bot passwords must include the high-volume editing grant as well as permission to move pages." +
			"\n\nBatches stop on the first error; earlier moves remain in place. Inspect the reported results before retrying a partial batch. --move-talk also asks MediaWiki to move associated talk pages; talk-page errors are reported even if the content page moved successfully.",
		Example: cobrautil.NormalizeExample(`
move 'Old title' 'New title' --wiki www.mediawiki.org --user 'Name@bot' --password-file password.txt
move --prefix 'Cli/ref/mw_docker' 'Cli/ref/mw_dev' --wiki www.mediawiki.org --dry-run
`),
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if interval < 0 {
				return fmt.Errorf("--interval must not be negative")
			}
			if limit < 0 {
				return fmt.Errorf("--limit must not be negative")
			}
			if cmd.Flags().Changed("title") {
				return fmt.Errorf("use FROM and TO arguments for move, rather than --title")
			}
			for _, title := range args {
				if strings.TrimSpace(title) == "" || strings.ContainsAny(title, "|#\r\n\x00") {
					return fmt.Errorf("FROM and TO must be nonempty page titles without fragments or separators")
				}
			}
			if !dryRun && (wikiAnon || wikiUser == "" || (wikiPassword == "" && wikiPasswordFile == "")) {
				return fmt.Errorf("moving pages requires --user and either --password or --password-file; omit --anon")
			}
			if bot && (wikiAnon || wikiUser == "" || (wikiPassword == "" && wikiPasswordFile == "")) {
				return fmt.Errorf("--bot requires --user and either --password or --password-file, including with --dry-run; omit --anon")
			}
			w, err := mwclient.New(normalizeWiki(wiki), cli.HTTPUserAgent)
			if err != nil {
				return err
			}
			if err := loginIfCredentialsProvided(w); err != nil {
				return err
			}
			if !wikiAnon && wikiUser != "" {
				w.Assert = mwclient.AssertUser
			}
			if bot {
				// Move has no module-specific bot parameter. Its log entries
				// inherit bot status from the session's rights; assert that
				// right before planning and on every subsequent request.
				w.Assert = mwclient.AssertBot
			}
			plan, err := planWikiMoves(cmd.Context(), w, args[0], args[1], prefix, limit, moveTalk)
			if err != nil {
				return err
			}
			collisions := 0
			skipped := 0
			var pending []wikiMove
			for _, item := range plan {
				if item.AlreadyMoved {
					skipped++
					if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Skipping %q -> %q (already redirects to destination)\n", item.From.Title, item.To); err != nil {
						return err
					}
					continue
				}
				pending = append(pending, item)
				status := "leave redirect"
				if item.DestinationExists {
					collisions++
					status = "BLOCKED: destination already exists"
				}
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Would move %q -> %q (%s)\n", item.From.Title, item.To, status); err != nil {
					return err
				}
			}
			if collisions > 0 {
				return fmt.Errorf("%d destination(s) already exist; no pages moved", collisions)
			}
			if dryRun {
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "Dry run: %d page(s) to move; %d already moved; no pages moved.\n", len(pending), skipped)
				return err
			}
			if len(pending) == 0 {
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "No pages to move; %d already redirect to their destinations.\n", skipped)
				return err
			}
			plan = pending
			token, err := w.GetToken(mwclient.CSRFToken)
			if err != nil {
				return err
			}
			for i, item := range plan {
				if i > 0 && interval > 0 {
					timer := time.NewTimer(interval)
					select {
					case <-cmd.Context().Done():
						timer.Stop()
						return fmt.Errorf("stopped after %d of %d moves: %w", i, len(plan), cmd.Context().Err())
					case <-timer.C:
					}
				}
				if err := cmd.Context().Err(); err != nil {
					return err
				}
				// Recheck after planning: another editor might have moved the
				// source or created a destination while earlier pages moved.
				pages, err := readWikiMovePages(w, []string{item.From.Title, item.To})
				if err == nil && (pages[moveTitleKey(item.From.Title)].ID != item.From.ID || !pages[moveTitleKey(item.To)].Missing) {
					err = fmt.Errorf("source or destination changed since planning")
				}
				if err != nil {
					return fmt.Errorf("stopped after %d of %d moves, before %q: %w", i, len(plan), item.From.Title, err)
				}
				p := params.Values{"action": "move", "formatversion": "2", "fromid": strconv.FormatInt(item.From.ID, 10), "to": item.To, "reason": reason, "token": token, "watchlist": "nochange"}
				if moveTalk {
					p["movetalk"] = "1"
				}
				reply, err := w.Post(p)
				if err != nil {
					return fmt.Errorf("move %q to %q failed after %d completed moves: %w; inspect the wiki before retrying", item.From.Title, item.To, i, err)
				}
				data, err := reply.Marshal()
				if err != nil {
					return err
				}
				var result struct {
					Move struct {
						From            string          `json:"from"`
						To              string          `json:"to"`
						RedirectCreated bool            `json:"redirectcreated"`
						TalkErrors      json.RawMessage `json:"talkmove-errors"`
					} `json:"move"`
				}
				if err := json.Unmarshal(data, &result); err != nil {
					return err
				}
				if moveTitleKey(result.Move.From) != moveTitleKey(item.From.Title) || moveTitleKey(result.Move.To) != moveTitleKey(item.To) {
					return fmt.Errorf("unexpected move response for %q; inspect the wiki before retrying", item.From.Title)
				}
				if !result.Move.RedirectCreated {
					return fmt.Errorf("%q moved to %q, but no redirect was created; batch stopped", result.Move.From, result.Move.To)
				}
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Moved %q -> %q (redirect left)\n", result.Move.From, result.Move.To); err != nil {
					return err
				}
				if errors := string(result.Move.TalkErrors); errors != "" && errors != "null" && errors != "[]" {
					return fmt.Errorf("content page moved, but moving its talk page failed: %s; batch stopped", errors)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&prefix, "prefix", false, "Move every page with the literal FROM prefix, replacing it with TO")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Read and display the move plan without moving pages")
	cmd.Flags().BoolVar(&moveTalk, "move-talk", false, "Also move associated talk pages when present")
	cmd.Flags().BoolVar(&bot, "bot", false, "Require the bot right so MediaWiki bot-marks the move log entries")
	cmd.Flags().DurationVar(&interval, "interval", 0, "Delay between moves to respect wiki rate limits (for example, 10s)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of pending source pages to move; already-moved pages are skipped (0 means all)")
	cmd.Flags().StringVar(&reason, "reason", cli.DisplayName+" page move", "Reason recorded in the move log")
	return cmd
}

func moveTitleKey(title string) string {
	return strings.ReplaceAll(title, "_", " ")
}

func readWikiMovePages(w *mwclient.Client, titles []string) (map[string]wikiMovePage, error) {
	reply, err := w.Get(params.Values{"action": "query", "formatversion": "2", "prop": "info", "inprop": "associatedpage", "titles": strings.Join(titles, "|")})
	if err != nil {
		return nil, err
	}
	data, err := reply.Marshal()
	if err != nil {
		return nil, err
	}
	var result struct {
		Query struct {
			Pages []wikiMovePage `json:"pages"`
		} `json:"query"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	pages := map[string]wikiMovePage{}
	for _, page := range result.Query.Pages {
		if page.Invalid {
			return nil, fmt.Errorf("invalid wiki title %q: %s", page.Title, page.InvalidReason)
		}
		pages[moveTitleKey(page.Title)] = page
	}
	// Follow the API's normalization (namespace aliases, capitalization, spaces).
	normalized, _ := reply.GetObjectArray("query", "normalized")
	for _, item := range normalized {
		from, _ := item.GetString("from")
		to, _ := item.GetString("to")
		if page, ok := pages[moveTitleKey(to)]; ok {
			pages[moveTitleKey(from)] = page
		}
	}
	for _, title := range titles {
		if _, ok := pages[moveTitleKey(title)]; !ok {
			return nil, fmt.Errorf("wiki omitted requested page %q", title)
		}
	}
	return pages, nil
}

func planWikiMoves(ctx context.Context, w *mwclient.Client, from, to string, prefix bool, limit int, moveTalk bool) ([]wikiMove, error) {
	fromPages, err := readWikiMovePages(w, []string{from})
	if err != nil {
		return nil, err
	}
	toPages, err := readWikiMovePages(w, []string{to})
	if err != nil {
		return nil, err
	}
	source, destination := fromPages[moveTitleKey(from)], toPages[moveTitleKey(to)]
	if source.Title == destination.Title {
		return nil, fmt.Errorf("source and destination are the same wiki title")
	}
	pages := []wikiMovePage{source}
	if prefix {
		if strings.HasPrefix(source.Title, destination.Title) || strings.HasPrefix(destination.Title, source.Title) {
			return nil, fmt.Errorf("source and destination prefixes must not overlap")
		}
		pages, err = listWikiMovePrefix(ctx, w, source)
		if err != nil {
			return nil, err
		}
	}
	if len(pages) == 0 || (!prefix && source.Missing) {
		return nil, fmt.Errorf("no pages found for %q", from)
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].Title < pages[j].Title })
	candidates := make([]wikiMove, 0, len(pages))
	seen := map[int64]bool{}
	for _, page := range pages {
		if page.ID <= 0 || seen[page.ID] || !strings.HasPrefix(page.Title, source.Title) {
			return nil, fmt.Errorf("wiki returned an unexpected source page %q", page.Title)
		}
		seen[page.ID] = true
		candidates = append(candidates, wikiMove{From: page, To: destination.Title + strings.TrimPrefix(page.Title, source.Title)})
	}
	var plan []wikiMove
	pending := 0
	for start := 0; start < len(candidates); start += 50 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var targets []string
		var redirectTitles []string
		batch := candidates[start:min(start+50, len(candidates))]
		for _, item := range batch {
			targets = append(targets, item.To)
			if item.From.Redirect {
				redirectTitles = append(redirectTitles, item.From.Title)
			}
		}
		destinations, err := readWikiMovePages(w, targets)
		if err != nil {
			return nil, err
		}
		redirects, err := readWikiMoveRedirects(w, redirectTitles)
		if err != nil {
			return nil, err
		}
		for _, item := range batch {
			target := destinations[moveTitleKey(item.To)]
			item.DestinationExists = !target.Missing
			directRedirect := item.From.Redirect && redirects[moveTitleKey(item.From.Title)] == moveTitleKey(target.Title)
			if directRedirect && target.Missing {
				return nil, fmt.Errorf("page %q redirects to missing destination %q; cannot treat it as an already-completed move", item.From.Title, target.Title)
			}
			item.AlreadyMoved = directRedirect && item.DestinationExists
			if item.AlreadyMoved {
				if moveTalk && item.From.Namespace%2 == 0 {
					if err := checkWikiMoveTalkDone(w, item.From, target); err != nil {
						return nil, err
					}
				}
			} else {
				pending++
			}
			plan = append(plan, item)
			if limit > 0 && pending == limit {
				return plan, nil
			}
		}
	}
	return plan, nil
}

// Read the API's immediate redirect edges, rather than treating a redirect chain
// as a completed move. A redirect to a section is not a redirect to the page.
func readWikiMoveRedirects(w *mwclient.Client, titles []string) (map[string]string, error) {
	redirects := map[string]string{}
	if len(titles) == 0 {
		return redirects, nil
	}
	reply, err := w.Get(params.Values{"action": "query", "formatversion": "2", "redirects": "1", "titles": strings.Join(titles, "|")})
	if err != nil {
		return nil, err
	}
	data, err := reply.Marshal()
	if err != nil {
		return nil, err
	}
	var result struct {
		Query struct {
			Redirects []struct {
				From     string `json:"from"`
				To       string `json:"to"`
				Fragment string `json:"tofragment"`
			} `json:"redirects"`
		} `json:"query"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	for _, redirect := range result.Query.Redirects {
		if redirect.Fragment == "" {
			redirects[moveTitleKey(redirect.From)] = moveTitleKey(redirect.To)
		}
	}
	return redirects, nil
}

func checkWikiMoveTalkDone(w *mwclient.Client, source, destination wikiMovePage) error {
	if source.AssociatedPage == "" || destination.AssociatedPage == "" {
		return fmt.Errorf("cannot verify associated talk pages for already-moved page %q", source.Title)
	}
	pages, err := readWikiMovePages(w, []string{source.AssociatedPage, destination.AssociatedPage})
	if err != nil {
		return err
	}
	oldTalk, newTalk := pages[moveTitleKey(source.AssociatedPage)], pages[moveTitleKey(destination.AssociatedPage)]
	if oldTalk.Missing {
		return nil
	}
	if oldTalk.Redirect && !newTalk.Missing {
		redirects, err := readWikiMoveRedirects(w, []string{oldTalk.Title})
		if err != nil {
			return err
		}
		if redirects[moveTitleKey(oldTalk.Title)] == moveTitleKey(newTalk.Title) {
			return nil
		}
	}
	return fmt.Errorf("page %q already redirects to its destination, but talk page %q is not moved; move that talk page explicitly before retrying --move-talk", source.Title, oldTalk.Title)
}

func listWikiMovePrefix(ctx context.Context, w *mwclient.Client, source wikiMovePage) ([]wikiMovePage, error) {
	prefix := source.Title
	if source.Namespace != 0 {
		_, prefix, _ = strings.Cut(prefix, ":")
	}
	p := params.Values{"action": "query", "formatversion": "2", "generator": "allpages", "prop": "info", "inprop": "associatedpage", "gapnamespace": strconv.Itoa(source.Namespace), "gapprefix": prefix, "gaplimit": "max"}
	var pages []wikiMovePage
	seen := map[string]bool{}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		reply, err := w.Get(p)
		if err != nil {
			return nil, err
		}
		data, err := reply.Marshal()
		if err != nil {
			return nil, err
		}
		var result struct {
			Query struct {
				Pages []wikiMovePage `json:"pages"`
			} `json:"query"`
			Continue map[string]string `json:"continue"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, err
		}
		pages = append(pages, result.Query.Pages...)
		if len(result.Continue) == 0 {
			return pages, nil
		}
		key := result.Continue["gapcontinue"]
		if key == "" || seen[key] {
			return nil, fmt.Errorf("wiki returned invalid prefix pagination")
		}
		seen[key] = true
		for key, value := range result.Continue {
			p[key] = value
		}
	}
}
