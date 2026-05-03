package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// apiPage describes a single Gerrit REST API documentation page to scrape.
type apiPage struct {
	slug           string
	useName        string
	shortDesc      string
	resourcePrefix string
	primaryParam   string
}

var apiPages = []apiPage{
	{"rest-api-access", "access", "Access Rights Endpoints", "/access", ""},
	{"rest-api-accounts", "accounts", "Accounts Endpoints", "/accounts", "account-id"},
	{"rest-api-changes", "changes", "Changes Endpoints", "/changes", "change-id"},
	{"rest-api-config", "server", "Server Config Endpoints", "/config/server", ""},
	{"rest-api-groups", "groups", "Group Endpoints", "/groups", "group-id"},
	{"rest-api-plugins", "plugins", "Plugin Endpoints", "/plugins", "plugin-id"},
	{"rest-api-projects", "projects", "Project Endpoints", "/projects", "project-name"},
}

// endpoint is a parsed API endpoint from the documentation.
type endpoint struct {
	method string
	path   string
	title  string
}

// Command mirrors the YAML structure expected by tools/code-gen/main.go.
type Command struct {
	Use           string    `yaml:"use"`
	Short         string    `yaml:"short,omitempty"`
	StringFlags   []Flag    `yaml:"string-flags,omitempty"`
	SubCommands   []Command `yaml:"sub-commands,omitempty"`
	GerritPath    string    `yaml:"gerrit-path,omitempty"`
	HttpMethod    string    `yaml:"http-method,omitempty"`
	FixedJsonBody string    `yaml:"fixed-json-body,omitempty"`
}

// Flag mirrors the flag structure in the YAML spec.
type Flag struct {
	Name        string `yaml:"name"`
	Value       string `yaml:"value,omitempty"`
	Required    bool   `yaml:"required,omitempty"`
	Usage       string `yaml:"usage"`
	GerritParam string `yaml:"gerrit-param"`
}

// skipPaths lists endpoints to skip entirely.
var skipPaths = map[string]bool{
	"POST /accounts/{account-id}/sshkeys":    true,
	"GET /Documentation/":                     true,
	"GET /config/server/indexes/changes/versions/85": true,
	"POST /groups/{group-id}/members.add":     true,
	"POST /groups/{group-id}/members.delete":  true,
	"POST /groups/{group-id}/groups.add":      true,
	"POST /groups/{group-id}/groups.delete":   true,
	"GET /projects/{project-name}/commits:in": true,
}

// specialBody maps endpoint keys to fixed JSON bodies.
var specialBody = map[string]string{
	"PUT /accounts/{account-id}/password.http": `{"generate": true}`,
}

// queryParams maps resource prefixes to their query parameters.
var queryParams = map[string][]Flag{
	"/accounts": {
		{Name: "query", GerritParam: "q", Usage: "Query string to find accounts."},
		{Name: "limit", GerritParam: "n", Usage: "Maximum number of accounts to return."},
		{Name: "start", GerritParam: "S", Usage: "Number of accounts to skip."},
	},
	"/changes": {
		{Name: "query", GerritParam: "q", Usage: "Query string to find changes."},
		{Name: "limit", GerritParam: "n", Usage: "Maximum number of changes to return."},
		{Name: "start", GerritParam: "S", Usage: "Number of changes to skip."},
	},
	"/groups": {
		{Name: "query", GerritParam: "q", Usage: "Query string to find groups."},
		{Name: "limit", GerritParam: "n", Usage: "Maximum number of groups to return."},
		{Name: "start", GerritParam: "S", Usage: "Number of groups to skip."},
	},
	"/projects": {
		{Name: "query", GerritParam: "q", Usage: "Query string to find projects."},
		{Name: "limit", GerritParam: "n", Usage: "Maximum number of projects to return."},
		{Name: "start", GerritParam: "S", Usage: "Number of projects to skip."},
	},
	"/plugins": {
		{Name: "limit", GerritParam: "n", Usage: "Maximum number of plugins to return."},
		{Name: "skip", GerritParam: "S", Usage: "Number of plugins to skip."},
		{Name: "prefix", GerritParam: "p", Usage: "Prefix to filter plugins by."},
		{Name: "regex", GerritParam: "r", Usage: "Regex to filter plugins by."},
		{Name: "substring", GerritParam: "m", Usage: "Substring to filter plugins by."},
	},
}

var (
	reHTTP         = regexp.MustCompile(`\s+HTTP/\d\.\d.*$`)
	reQuery        = regexp.MustCompile(`\?.*$`)
	reAnchorRef    = regexp.MustCompile(`#[\w-]+\[\{([\w-]+)\}\]`)
	reSig          = regexp.MustCompile(`^(GET|PUT|POST|DELETE)\s+(/\S+)$`)
	rePathParam    = regexp.MustCompile(`\{([^}]+)\}`)
	reNonAlphaNum  = regexp.MustCompile(`[^a-zA-Z0-9]`)
	reSplitDotEtc  = regexp.MustCompile(`[.:~]`)
)

func fetchPage(baseURL, slug string) (*goquery.Document, error) {
	url := fmt.Sprintf("%s/%s.html", baseURL, slug)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fetching %s: status %d", url, resp.StatusCode)
	}
	return goquery.NewDocumentFromReader(resp.Body)
}

func scrapeEndpoints(doc *goquery.Document) []endpoint {
	var raw []endpoint

	doc.Find("p").Each(func(_ int, p *goquery.Selection) {
		text := strings.TrimSpace(p.Text())
		if !strings.HasPrefix(text, "'") {
			return
		}
		inner := strings.TrimPrefix(text, "'")
		hasMethod := false
		for _, m := range []string{"GET ", "PUT ", "POST ", "DELETE "} {
			if strings.HasPrefix(inner, m) {
				hasMethod = true
				break
			}
		}
		if !hasMethod {
			return
		}

		sig := strings.Trim(text, "'")
		sig = strings.TrimSpace(sig)
		sig = reHTTP.ReplaceAllString(sig, "")
		sig = reQuery.ReplaceAllString(sig, "")
		sig = reAnchorRef.ReplaceAllString(sig, "{${1}}")

		matches := reSig.FindStringSubmatch(sig)
		if matches == nil {
			return
		}
		method, path := matches[1], matches[2]

		// Find nearest previous heading for the title
		title := ""
		heading := p.PrevAll().Filter("h2,h3").First()
		if heading.Length() == 0 {
			heading = p.Parent().PrevAll().Filter("h2,h3").First()
		}
		if heading.Length() == 0 {
			// Walk up looking at siblings
			p.Parents().Each(func(_ int, parent *goquery.Selection) {
				if heading.Length() > 0 {
					return
				}
				parent.PrevAll().Filter("h2,h3").Each(func(_ int, h *goquery.Selection) {
					if heading.Length() == 0 || true {
						heading = h
					}
				})
			})
		}
		if heading.Length() > 0 {
			title = strings.TrimSpace(heading.Text())
		}

		raw = append(raw, endpoint{method, path, title})
	})

	// Deduplicate by (method, normalizedPath)
	seen := map[string]bool{}
	var result []endpoint
	for _, ep := range raw {
		key := ep.method + " " + normPath(ep.path)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, ep)
	}
	return result
}

func normPath(path string) string {
	return strings.TrimRight(path, "/")
}

func ensureSlash(path string) string {
	if strings.HasSuffix(path, "/") {
		return path
	}
	return path + "/"
}

func extractPathParams(path string) []string {
	matches := rePathParam.FindAllStringSubmatch(path, -1)
	seen := map[string]bool{}
	var params []string
	for _, m := range matches {
		p := m[1]
		if !seen[p] {
			seen[p] = true
			params = append(params, p)
		}
	}
	return params
}

func flagName(param string) string {
	name := param
	for _, sfx := range []string{"-id", "-name"} {
		if strings.HasSuffix(name, sfx) && len(name) > len(sfx) {
			name = name[:len(name)-len(sfx)]
			break
		}
	}
	name = reNonAlphaNum.ReplaceAllString(name, "")
	return strings.ToLower(name)
}

func buildFlags(pathParams []string) []Flag {
	var flags []Flag
	for _, param := range pathParams {
		f := Flag{
			Name:        flagName(param),
			GerritParam: param,
		}
		if param == "account-id" {
			f.Value = "self"
			f.Usage = "The account identifier."
		} else {
			f.Required = true
			f.Usage = fmt.Sprintf("The %s to operate on.", flagName(param))
		}
		flags = append(flags, f)
	}
	return flags
}

type phraseCheck struct {
	keywords []string
	verb     string
}

type startCheck struct {
	prefix string
	verb   string
}

var phraseChecks = []phraseCheck{
	{[]string{"query account", "query group", "query changes", "query project"}, "list"},
	{[]string{"get changes with default star"}, "list"},
	{[]string{"get submitted together"}, "submitted-together"},
	{[]string{"get pure revert"}, "pure-revert"},
	{[]string{"get audit log"}, "get-audit-log"},
	{[]string{"get avatar change url"}, "get-change-url"},
	{[]string{"check consistency"}, "check-consistency"},
	{[]string{"confirm email"}, "confirm-email"},
	{[]string{"add/delete"}, "post"},
	{[]string{"add/update"}, "set"},
	{[]string{"set/generate", "generate http"}, "generate"},
	{[]string{"put default star"}, "star"},
	{[]string{"remove default star"}, "unstar"},
	{[]string{"revert submission"}, "revert-submission"},
	{[]string{"rebase chain"}, "rebase-chain"},
	{[]string{"unmark private", "remove private"}, "unprivate"},
	{[]string{"mark private"}, "private"},
	{[]string{"mark work in progress", "set work in progress"}, "wip"},
	{[]string{"unmark work in progress"}, "unset-wip"},
	{[]string{"mark ready", "set ready"}, "ready"},
	{[]string{"cherry-pick", "cherry pick", "cherrypick"}, "cherrypick"},
	{[]string{"set preferred email"}, "prefer"},
	{[]string{"batch update"}, "batch-update"},
}

var startChecks = []startCheck{
	{"list", "list"},
	{"query", "list"},
	{"retrieve", "get"},
	{"get", "get"},
	{"create", "create"},
	{"add", "add"},
	{"set", "set"},
	{"sign", "sign"},
	{"install", "install"},
	{"enable", "enable"},
	{"disable", "disable"},
	{"reload", "reload"},
	{"abandon", "abandon"},
	{"restore", "restore"},
	{"submit", "submit"},
	{"move", "move"},
	{"revert", "revert"},
	{"rebase", "rebase"},
	{"index", "index"},
	{"check", "check"},
	{"merge", "merge"},
	{"publish", "publish"},
	{"apply", "apply"},
	{"flush", "flush"},
	{"snapshot", "snapshot"},
	{"reindex", "reindex"},
	{"star", "star"},
	{"unstar", "unstar"},
	{"delete", "delete"},
	{"remove", "delete"},
	{"rename", "set"},
	{"put", "set"},
	{"run", "run"},
	{"confirm", "confirm"},
	{"change", "set"},
}

var methodFallback = map[string]string{
	"GET": "get", "PUT": "set", "POST": "create", "DELETE": "delete",
}

func deriveVerb(method, title string) string {
	t := strings.ToLower(title)

	for _, pc := range phraseChecks {
		for _, kw := range pc.keywords {
			if strings.Contains(t, kw) {
				return pc.verb
			}
		}
	}

	for _, sc := range startChecks {
		if strings.HasPrefix(t, sc.prefix) {
			return sc.verb
		}
	}

	if v, ok := methodFallback[method]; ok {
		return v
	}
	return "get"
}

func segmentKey(seg string) string {
	if strings.HasPrefix(seg, "{") {
		return ""
	}
	s := seg
	for _, old := range []string{".", ":", "_", "~", "%2f", "%2F"} {
		s = strings.ReplaceAll(s, old, "-")
	}
	return strings.Trim(s, "-")
}

func getSubResourceSegments(path, resourcePrefix, primaryParam string) []string {
	rel := path
	if strings.HasPrefix(path, resourcePrefix) {
		rel = path[len(resourcePrefix):]
	}
	parts := splitNonEmpty(rel, "/")

	// Strip leading primary param
	if len(parts) > 0 {
		if primaryParam != "" && parts[0] == "{"+primaryParam+"}" {
			parts = parts[1:]
		} else if strings.HasPrefix(parts[0], "{") {
			parts = parts[1:]
		}
	}

	// Split dot/colon/tilde-separated segments into sub-segments
	var expanded []string
	for _, p := range parts {
		if strings.HasPrefix(p, "{") {
			expanded = append(expanded, p)
		} else {
			subs := reSplitDotEtc.Split(p, -1)
			expanded = append(expanded, subs...)
		}
	}
	return expanded
}

func splitNonEmpty(s, sep string) []string {
	parts := strings.Split(s, sep)
	var result []string
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func buildLeaf(ep endpoint, resourcePrefix, primaryParam string) Command {
	verb := deriveVerb(ep.method, ep.title)
	gerritPath := ensureSlash(ep.path)
	params := extractPathParams(gerritPath)
	flags := buildFlags(params)

	// Add query params for list endpoints
	if verb == "list" {
		if qps, ok := queryParams[normPath(ep.path)]; ok {
			flags = append(flags, qps...)
		}
	}

	cmd := Command{
		Use:        verb,
		Short:      ep.title,
		GerritPath: gerritPath,
	}
	if len(flags) > 0 {
		cmd.StringFlags = flags
	}
	if ep.method != "GET" {
		cmd.HttpMethod = ep.method
	}

	key := ep.method + " " + normPath(ep.path)
	if body, ok := specialBody[key]; ok {
		cmd.FixedJsonBody = body
	}

	return cmd
}

// treeItem represents an endpoint with its remaining sub-resource segments.
type treeItem struct {
	ep   endpoint
	rest []string
}

func buildTree(endpoints []endpoint, resourcePrefix, primaryParam string) ([]endpoint, map[string][]treeItem) {
	var rootEps []endpoint
	groups := map[string][]treeItem{}
	groupOrder := []string{}

	for _, ep := range endpoints {
		key := ep.method + " " + normPath(ep.path)
		if skipPaths[key] {
			continue
		}

		segs := getSubResourceSegments(ep.path, resourcePrefix, primaryParam)
		var nonParamSegs []string
		for _, s := range segs {
			if !strings.HasPrefix(s, "{") {
				nonParamSegs = append(nonParamSegs, s)
			}
		}

		if len(nonParamSegs) == 0 {
			rootEps = append(rootEps, ep)
		} else {
			gk := segmentKey(nonParamSegs[0])
			if gk == "" {
				rootEps = append(rootEps, ep)
			} else {
				if _, exists := groups[gk]; !exists {
					groupOrder = append(groupOrder, gk)
				}
				groups[gk] = append(groups[gk], treeItem{ep, nonParamSegs[1:]})
			}
		}
	}

	// Preserve insertion order
	orderedGroups := map[string][]treeItem{}
	for _, gk := range groupOrder {
		orderedGroups[gk] = groups[gk]
	}

	return rootEps, orderedGroups
}

func deduplicateUses(cmds []Command) {
	counts := map[string]int{}
	for _, c := range cmds {
		counts[c.Use]++
	}

	idxMap := map[string]int{}
	for i := range cmds {
		u := cmds[i].Use
		if counts[u] <= 1 {
			continue
		}
		idxMap[u]++
		idx := idxMap[u]

		gp := cmds[i].GerritPath
		lastSeg := ""
		if gp != "" {
			parts := splitNonEmpty(strings.TrimRight(gp, "/"), "/")
			if len(parts) > 0 {
				lastSeg = parts[len(parts)-1]
			}
		}
		if strings.HasPrefix(lastSeg, "{") && idx == 1 {
			continue
		}
		cmds[i].Use = fmt.Sprintf("%s-%d", cmds[i].Use, idx)
	}
}

func buildGroup(gk string, items []treeItem, resourcePrefix, primaryParam string) Command {
	deeper := map[string][]treeItem{}
	deeperOrder := []string{}
	var direct []endpoint

	for _, item := range items {
		if len(item.rest) > 0 {
			dk := segmentKey(item.rest[0])
			if dk != "" {
				if _, exists := deeper[dk]; !exists {
					deeperOrder = append(deeperOrder, dk)
				}
				deeper[dk] = append(deeper[dk], treeItem{item.ep, item.rest[1:]})
			} else {
				direct = append(direct, item.ep)
			}
		} else {
			direct = append(direct, item.ep)
		}
	}

	var subCmds []Command

	for _, ep := range direct {
		leaf := buildLeaf(ep, resourcePrefix, primaryParam)
		subCmds = append(subCmds, leaf)
	}

	for _, dk := range deeperOrder {
		ditems := deeper[dk]
		if len(ditems) == 1 {
			leaf := buildLeaf(ditems[0].ep, resourcePrefix, primaryParam)
			leaf.Use = dk
			subCmds = append(subCmds, leaf)
		} else {
			subGroup := buildGroup(dk, ditems, resourcePrefix, primaryParam)
			subCmds = append(subCmds, subGroup)
		}
	}

	deduplicateUses(subCmds)

	// If only one direct endpoint and no deeper groups, return flat command
	if len(items) == 1 && len(deeper) == 0 && len(direct) == 1 {
		leaf := buildLeaf(direct[0], resourcePrefix, primaryParam)
		leaf.Use = gk
		return leaf
	}

	group := Command{
		Use:   gk,
		Short: titleCase(strings.ReplaceAll(gk, "-", " ")),
	}
	if len(subCmds) > 0 {
		group.SubCommands = subCmds
	}
	return group
}

func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

func buildPageSpec(doc *goquery.Document, page apiPage) Command {
	endpoints := scrapeEndpoints(doc)
	fmt.Fprintf(os.Stderr, "  Found %d endpoints\n", len(endpoints))

	rootEps, groups := buildTree(endpoints, page.resourcePrefix, page.primaryParam)

	top := Command{
		Use:   page.useName,
		Short: page.shortDesc,
	}

	var subCommands []Command

	for _, ep := range rootEps {
		leaf := buildLeaf(ep, page.resourcePrefix, page.primaryParam)
		subCommands = append(subCommands, leaf)
	}

	// Collect group keys in order
	groupKeys := make([]string, 0, len(groups))
	for gk := range groups {
		groupKeys = append(groupKeys, gk)
	}
	// Sort to ensure deterministic output
	sort.Strings(groupKeys)

	// Actually, we want insertion order from buildTree. The groups map loses order.
	// Let's rebuild the order from the endpoints.
	groupKeys = nil
	seen := map[string]bool{}
	for _, ep := range endpoints {
		key := ep.method + " " + normPath(ep.path)
		if skipPaths[key] {
			continue
		}
		segs := getSubResourceSegments(ep.path, page.resourcePrefix, page.primaryParam)
		var nonParamSegs []string
		for _, s := range segs {
			if !strings.HasPrefix(s, "{") {
				nonParamSegs = append(nonParamSegs, s)
			}
		}
		if len(nonParamSegs) > 0 {
			gk := segmentKey(nonParamSegs[0])
			if gk != "" && !seen[gk] {
				seen[gk] = true
				if _, exists := groups[gk]; exists {
					groupKeys = append(groupKeys, gk)
				}
			}
		}
	}

	for _, gk := range groupKeys {
		items := groups[gk]
		if len(items) == 1 && len(items[0].rest) == 0 {
			leaf := buildLeaf(items[0].ep, page.resourcePrefix, page.primaryParam)
			leaf.Use = gk
			subCommands = append(subCommands, leaf)
		} else {
			grp := buildGroup(gk, items, page.resourcePrefix, page.primaryParam)
			subCommands = append(subCommands, grp)
		}
	}

	deduplicateUses(subCommands)

	if len(subCommands) > 0 {
		top.SubCommands = subCommands
	}

	return top
}

func main() {
	baseURL := flag.String("base-url", "https://gerrit.wikimedia.org/r/Documentation", "Base URL for Gerrit REST API docs")
	output := flag.String("output", "tools/code-gen/gerrit.yml", "Output YAML file path")
	flag.Parse()

	var spec []Command

	for _, page := range apiPages {
		fmt.Fprintf(os.Stderr, "Scraping %s...\n", page.slug)
		doc, err := fetchPage(*baseURL, page.slug)
		if err != nil {
			logrus.Fatalf("Failed to fetch %s: %v", page.slug, err)
		}
		pageSpec := buildPageSpec(doc, page)
		spec = append(spec, pageSpec)
	}

	yamlBytes, err := yaml.Marshal(spec)
	if err != nil {
		logrus.Fatalf("Failed to marshal YAML: %v", err)
	}

	header := "# This file is auto-generated by tools/gerrit-scraper/main.go\n" +
		fmt.Sprintf("# Source: %s/rest-api.html\n", *baseURL) +
		"# DO NOT EDIT manually - regenerate with:\n" +
		"#   go run tools/gerrit-scraper/main.go\n\n"

	err = os.WriteFile(*output, []byte(header+string(yamlBytes)), 0644)
	if err != nil {
		logrus.Fatalf("Failed to write %s: %v", *output, err)
	}

	fmt.Fprintf(os.Stderr, "\nWrote %s\n", *output)
}
