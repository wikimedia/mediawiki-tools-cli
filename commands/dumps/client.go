package dumps

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"
	"unicode"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"golang.org/x/net/html"
)

const officialURL = "https://dumps.wikimedia.org/"

type dataset struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Description string `json:"description"`
}

var datasets = []dataset{
	{"current", "other/mediawiki_content_current/", "Current revision content exports (XML); replaces legacy content dumps"},
	{"history", "other/mediawiki_content_history/", "Full revision history exports (XML); replaces legacy content dumps"},
	{"wikidata", "wikidatawiki/entities/", "Wikidata entities and lexemes (JSON, Turtle, N-Triples)"},
	{"commons", "commonswiki/entities/", "Commons structured data entities (JSON and RDF)"},
	{"wikibase", "other/wikibase/", "Wikibase datasets and metadata"},
	{"other", "other/", "Other datasets, including titles, search indexes, HTML and analytics"},
	{"analytics", "other/analytics/", "Pageviews, media requests and other analytics dumps"},
	{"sql", "", "Per-wiki SQL tables and legacy XML dumps; browse a wiki such as enwiki/"},
}

type mirror struct {
	Name           string `json:"name"`
	URL            string `json:"url"`
	Notes          string `json:"notes"`
	EntityPrefix   string `json:"entity_prefix,omitempty"`
	WikiPrefix     string `json:"wiki_prefix,omitempty"`
	NoWikiLatest   bool   `json:"no_wiki_latest,omitempty"`
	NoEntityLatest bool   `json:"no_entity_latest,omitempty"`
}

// Coverage and freshness vary; this is a catalog, not an availability guarantee.
// Source: https://meta.wikimedia.org/wiki/Mirroring_Wikimedia_project_XML_dumps
var mirrors = []mirror{
	{Name: "wikimedia", URL: officialURL, Notes: "Official source; at most 3 connections per IP"},
	{Name: "your", URL: "https://dumps.wikimedia.your.org/", Notes: "Your.org, United States; coverage varies"},
	{
		Name: "umu", URL: "https://ftp.acc.umu.se/mirror/wikimedia.org/", Notes: "Umea University, Sweden; per-wiki dumps under dumps/ (dated paths only); entities under other/wikibase/",
		EntityPrefix: "other/wikibase/", WikiPrefix: "dumps/", NoWikiLatest: true,
	},
	{
		Name: "scatter", URL: "https://files.scatter.red/wikimedia/", Notes: "United States; selected, possibly stale datasets; entities under other/wikibase/; use dated paths instead of unreliable latest aliases",
		EntityPrefix: "other/wikibase/", NoWikiLatest: true, NoEntityLatest: true,
	},
	{Name: "clarkson", URL: "https://wikimedia.mirror.clarkson.edu/", Notes: "United States; may contain stale data"},
	{Name: "pdapps", URL: "https://wikipedia.mirror.pdapps.org/", Notes: "Russia; recent dumps"},
}

func knownMirror(base *url.URL) *mirror {
	for i := range mirrors {
		if base.String() == mirrors[i].URL {
			return &mirrors[i]
		}
	}
	return nil
}

func entityPath(relative string) (string, bool) {
	for _, wiki := range []string{"wikidatawiki", "commonswiki"} {
		if rest, ok := strings.CutPrefix(relative, wiki+"/entities/"); ok {
			return wiki + "/" + rest, true
		}
	}
	return "", false
}

func wikiPath(relative string) bool {
	first, _, _ := strings.Cut(relative, "/")
	for _, suffix := range []string{"wiki", "wiktionary", "wikibooks", "wikinews", "wikiquote", "wikisource", "wikiversity", "wikivoyage", "wikimedia"} {
		if strings.HasSuffix(first, suffix) {
			return true
		}
	}
	return false
}

// mirrorTarget accepts canonical Wikimedia paths, not paths from a mirror's
// filesystem. Explicit known base URLs get the same mapping as mirror names.
func mirrorTarget(base *url.URL, relative string) (*url.URL, error) {
	m := knownMirror(base)
	if m == nil {
		return base.JoinPath(relative), nil
	}
	relative = canonicalMirrorPath(base, relative)
	if rest, ok := entityPath(relative); ok && m.EntityPrefix != "" {
		if m.NoEntityLatest && strings.HasPrefix(path.Base(relative), "latest-") {
			return nil, fmt.Errorf("%s latest entity aliases are not reliable dump files; list the entity directory and select a dated path", m.Name)
		}
		return base.JoinPath(m.EntityPrefix + rest), nil
	}
	if wikiPath(relative) {
		_, rest, _ := strings.Cut(relative, "/")
		if m.NoWikiLatest && (rest == "latest" || strings.HasPrefix(rest, "latest/")) {
			return nil, fmt.Errorf("%s has no per-wiki latest alias; list the wiki directory and select a dated path", m.Name)
		}
		relative = m.WikiPrefix + relative
	}
	return base.JoinPath(relative), nil
}

func canonicalMirrorPath(base *url.URL, relative string) string {
	m := knownMirror(base)
	if m == nil {
		return relative
	}
	if m.EntityPrefix != "" {
		for _, wiki := range []string{"wikidatawiki", "commonswiki"} {
			if rest, ok := strings.CutPrefix(relative, m.EntityPrefix+wiki+"/"); ok {
				return wiki + "/entities/" + rest
			}
		}
	}
	if m.WikiPrefix != "" {
		if rest, ok := strings.CutPrefix(relative, m.WikiPrefix); ok && wikiPath(rest) {
			return rest
		}
	}
	return relative
}

func mirrorURL(value string) (*url.URL, error) {
	for _, m := range mirrors {
		if value == m.Name {
			value = m.URL
			break
		}
	}
	u, err := url.Parse(value)
	if err != nil || u.Host == "" || u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("mirror must be a known name or an HTTPS base URL")
	}
	if strings.IndexFunc(u.Path, unicode.IsControl) >= 0 {
		return nil, fmt.Errorf("invalid mirror path")
	}
	if !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
	}
	return u, nil
}

func dumpPath(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if strings.HasPrefix(value, "/") || strings.ContainsAny(value, `\?#%`) || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return "", fmt.Errorf("use a relative dump path or dataset shortcut, not a URL")
	}
	for _, component := range strings.Split(value, "/") {
		if component == "." || component == ".." {
			return "", fmt.Errorf("dump paths must not contain . or .. components")
		}
	}
	first, rest, _ := strings.Cut(value, "/")
	for _, d := range datasets {
		if first == d.Name {
			return d.Path + rest, nil
		}
	}
	if strings.Contains(first, ":") {
		return "", fmt.Errorf("use --mirror to select a server")
	}
	return value, nil
}

type dumpClient struct {
	http    *http.Client
	approve func(*url.URL) error
}

func newClient(approve func(*url.URL) error) *dumpClient {
	client := &dumpClient{approve: approve}
	client.http = &http.Client{
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			TLSHandshakeTimeout:   15 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			IdleConnTimeout:       30 * time.Second,
			MaxConnsPerHost:       1,
		},
	}
	client.http.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many dump redirects")
		}
		if req.URL.Scheme != "https" {
			return fmt.Errorf("refusing a non-HTTPS dump redirect")
		}
		return client.approve(req.URL)
	}
	return client
}

func (c *dumpClient) request(ctx context.Context, method string, u *url.URL, headers http.Header) (*http.Response, error) {
	if err := c.approve(u); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", cli.HTTPUserAgent())
	req.Header.Set("Accept-Encoding", "identity")
	for key, values := range headers {
		req.Header[key] = values
	}
	return c.http.Do(req)
}

func (c *dumpClient) metadata(ctx context.Context, u *url.URL) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	resp, err := c.request(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: HTTP %d", u, resp.StatusCode)
	}
	const limit = 16 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if len(data) > limit {
		return nil, fmt.Errorf("dump metadata exceeds 16 MiB")
	}
	return data, nil
}

type entry struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Directory bool   `json:"directory"`
	URL       string `json:"url"`
}

func (c *dumpClient) list(ctx context.Context, base *url.URL, relative string) ([]entry, error) {
	isPage := strings.HasSuffix(relative, ".html") || strings.HasSuffix(relative, ".htm")
	if relative != "" && !strings.HasSuffix(relative, "/") && !isPage {
		relative += "/"
	}
	directory, err := mirrorTarget(base, relative)
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(directory.Path, "/") && !isPage {
		directory.Path += "/"
	}
	data, err := c.metadata(ctx, directory)
	if err != nil {
		return nil, err
	}
	root, err := html.Parse(strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}
	result := make([]entry, 0)
	seen := map[string]bool{}
	var visit func(*html.Node)
	visit = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "a" {
			for _, attr := range node.Attr {
				if attr.Key != "href" {
					continue
				}
				ref, err := url.Parse(attr.Val)
				if err != nil || ref.RawQuery != "" || ref.Fragment != "" {
					continue
				}
				target := directory.ResolveReference(ref)
				if target.Scheme != base.Scheme || target.Host != base.Host || !strings.HasPrefix(target.Path, base.Path) {
					continue
				}
				relativePath := canonicalMirrorPath(base, strings.TrimPrefix(target.Path, base.Path))
				if target.Path == directory.Path || strings.HasPrefix(directory.Path, target.Path) {
					continue
				}
				name := strings.TrimPrefix(target.Path, directory.Path)
				if name == target.Path {
					name = relativePath
				}
				isDir := strings.HasSuffix(name, "/")
				if name == "" || strings.IndexFunc(name, unicode.IsControl) >= 0 || seen[relativePath] {
					continue
				}
				seen[relativePath] = true
				result = append(result, entry{Name: name, Path: relativePath, Directory: isDir, URL: target.String()})
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(root)
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func contentExport(relative string) bool {
	return (strings.HasPrefix(relative, "other/mediawiki_content_current/") || strings.HasPrefix(relative, "other/mediawiki_content_history/")) &&
		strings.HasSuffix(relative, ".xml.bz2") && path.Base(path.Dir(relative)) == "bzip2"
}
