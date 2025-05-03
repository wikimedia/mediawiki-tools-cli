package wiki

import (
	_ "embed"
	"fmt"
	"strings"

	mwclient "cgt.name/pkg/go-mwclient"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
)

func NewWikiPageListCmd() *cobra.Command {
	var prefix string
	var namespace int
	var limit int
	var from string
	var to string
	var filterredir string
	var filterlanglinks string
	var minsize int
	var maxsize int
	var prtype string
	var prlevel string
	var prfiltercascade string
	var prexpiry string
	var dir string
	var dryRun bool
	var grep string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "MediaWiki Wiki Page List",
		RunE:  nil,
		Example: cobrautil.NormalizeExample(`
# List pages on test.wikipedia.org
list --wiki https://test.wikipedia.org/w/api.php --user ${user} --password ${password} --prefix ${prefix} --namespace ${namespace} --limit ${limit}
`),
		Run: func(cmd *cobra.Command, args []string) {
			if dryRun {
				fmt.Println("Dry run mode: Listing pages with the following parameters:")
				fmt.Printf("wiki: %s, user: %s, prefix: %s, namespace: %d, limit: %d, from: %s, to: %s, filterredir: %s, filterlanglinks: %s, minsize: %d, maxsize: %d, prtype: %s, prlevel: %s, prfiltercascade: %s, prexpiry: %s, dir: %s, grep: %s\n",
					wiki, wikiUser, prefix, namespace, limit, from, to, filterredir, filterlanglinks, minsize, maxsize, prtype, prlevel, prfiltercascade, prexpiry, dir, grep)
				return
			}

			if wiki == "" {
				logrus.Fatal("wiki is not set")
			}
			if wikiUser == "" {
				logrus.Fatal("wiki user is not set")
			}
			if wikiPassword == "" {
				logrus.Fatal("wiki password is not set")
			}

			w, err := mwclient.New(normalizeWiki(wiki), "mwcli")
			if err != nil {
				panic(err)
			}

			defaultErrorHandling().handle(w.Login(wikiUser, wikiPassword))

			// List pages with pagination
			listParams := map[string]string{
				"action":  "query",
				"list":    "allpages",
				"aplimit": "max",
			}

			if prefix != "" {
				listParams["apprefix"] = prefix
			}

			if namespace != 0 {
				listParams["apnamespace"] = fmt.Sprintf("%d", namespace)
			}

			if limit > 0 && grep == "" {
				listParams["aplimit"] = fmt.Sprintf("%d", limit)
			}

			if from != "" {
				listParams["apfrom"] = from
			}

			if to != "" {
				listParams["apto"] = to
			}

			if filterredir != "" {
				listParams["apfilterredir"] = filterredir
			}

			if filterlanglinks != "" {
				listParams["apfilterlanglinks"] = filterlanglinks
			}

			if minsize > 0 {
				listParams["apminsize"] = fmt.Sprintf("%d", minsize)
			}

			if maxsize > 0 {
				listParams["apmaxsize"] = fmt.Sprintf("%d", maxsize)
			}

			if prtype != "" {
				listParams["apprtype"] = prtype
			}

			if prlevel != "" {
				listParams["apprlevel"] = prlevel
			}

			if prfiltercascade != "" {
				listParams["apprfiltercascade"] = prfiltercascade
			}

			if prexpiry != "" {
				listParams["apprexpiry"] = prexpiry
			}

			switch dir {
			case "asc", "ASC":
				dir = "ascending"
			case "desc", "DESC":
				dir = "descending"
			}

			if dir != "" {
				listParams["apdir"] = dir
			}

			totalPages := 0
			for {
				res, err := w.Get(listParams)
				if err != nil {
					panic(err)
				}

				query, err := res.GetObject("query")
				if err != nil {
					panic(err)
				}
				allPages, err := query.GetObjectArray("allpages")
				if err != nil {
					panic(err)
				}
				for _, page := range allPages {
					title, err := page.GetString("title")
					if err != nil {
						panic(err)
					}
					if grep == "" || strings.Contains(title, grep) {
						fmt.Println(title)
						totalPages++
						if limit > 0 && totalPages >= limit {
							return
						}
					}
				}

				cont, err := res.GetObject("continue")
				if err != nil {
					break
				}
				apcontinue, err := cont.GetString("apcontinue")
				if err != nil {
					break
				}
				listParams["apcontinue"] = apcontinue
			}
		},
	}

	cmd.Flags().StringVar(&prefix, "prefix", "", "Prefix for filtering page titles")
	cmd.Flags().IntVar(&namespace, "namespace", 0, "Namespace ID for filtering page titles")
	cmd.Flags().IntVar(&limit, "limit", 0, "Limit the number of pages returned")
	cmd.Flags().StringVar(&from, "from", "", "The page title to start enumerating from")
	cmd.Flags().StringVar(&to, "to", "", "The page title to stop enumerating at")
	cmd.Flags().StringVar(&filterredir, "filterredir", "all", "Which pages to list (all, nonredirects, redirects)")
	cmd.Flags().StringVar(&filterlanglinks, "filterlanglinks", "all", "Filter based on whether a page has langlinks (all, withlanglinks, withoutlanglinks)")
	cmd.Flags().IntVar(&minsize, "minsize", 0, "Limit to pages with at least this many bytes")
	cmd.Flags().IntVar(&maxsize, "maxsize", 0, "Limit to pages with at most this many bytes")
	cmd.Flags().StringVar(&prtype, "prtype", "", "Limit to protected pages only (edit, move, upload)")
	cmd.Flags().StringVar(&prlevel, "prlevel", "", "Filter protections based on protection level (autoconfirmed, sysop)")
	cmd.Flags().StringVar(&prfiltercascade, "prfiltercascade", "all", "Filter protections based on cascadingness (all, cascading, noncascading)")
	cmd.Flags().StringVar(&prexpiry, "prexpiry", "all", "Which protection expiry to filter the page on (all, definite, indefinite)")
	cmd.Flags().StringVar(&dir, "dir", "ascending", "The direction in which to list (ascending, descending)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "If set, only print the action that would be performed")
	cmd.Flags().StringVar(&grep, "grep", "", "Filter the resulting list to only include titles that contain this string")

	return cmd
}
