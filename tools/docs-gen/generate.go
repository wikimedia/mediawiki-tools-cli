package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	osexec "os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"gitlab.wikimedia.org/repos/releng/cli/cmd"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
)

const (
	configPath   = "docs/wiki.json"
	outputDir    = "_docs"
	markerPrefix = "<!-- a-wikimedia-cli sha256:"
)

type guide struct {
	Source string `json:"source"`
	Title  string `json:"title"`
}

type baseline struct {
	Revision int64  `json:"revision"`
	SHA256   string `json:"sha256"`
}

type configuration struct {
	API             string              `json:"api"`
	PageURL         string              `json:"page_url"`
	ReferenceRoot   string              `json:"reference_root"`
	LegacyPublisher string              `json:"legacy_publisher"`
	LegacySummary   string              `json:"legacy_summary"`
	Guides          []guide             `json:"guides"`
	Reviewed        map[string]baseline `json:"reviewed"`
}

type document struct {
	Title    string `json:"title"`
	Source   string `json:"source"`
	Markdown string `json:"markdown"`
	Wikitext string `json:"wikitext"`
}

func loadConfig() (configuration, error) {
	var config configuration
	err := readJSON(configPath, &config)
	if err != nil {
		return config, err
	}
	if config.API == "" || config.PageURL == "" || config.ReferenceRoot == "" {
		return config, fmt.Errorf("incomplete %s", configPath)
	}
	seen := map[string]bool{}
	sources := map[string]bool{}
	for _, guide := range config.Guides {
		if _, err := guidePath(guide.Source); err != nil {
			return config, err
		}
		if guide.Title == "" || seen[strings.ReplaceAll(guide.Title, "_", " ")] || sources[guide.Source] || guide.Title == config.ReferenceRoot || strings.HasPrefix(guide.Title, config.ReferenceRoot+"/") {
			return config, fmt.Errorf("invalid or duplicate guide title %q", guide.Title)
		}
		seen[strings.ReplaceAll(guide.Title, "_", " ")] = true
		sources[guide.Source] = true
	}
	return config, nil
}

func guidePath(source string) (string, error) {
	if (source != "index.md" && !strings.HasPrefix(source, "guides/")) || filepath.Ext(source) != ".md" || filepath.IsAbs(source) || filepath.Clean(source) != source || source == ".." || strings.HasPrefix(source, "../") {
		return "", fmt.Errorf("invalid guide source %q", source)
	}
	return filepath.Join("docs", source), nil
}

func readJSON(name string, target interface{}) error {
	data, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func writeJSON(name string, value interface{}) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(name, append(data, '\n'), 0o644)
}

func wikiURL(config configuration, title string) string {
	return strings.TrimRight(config.PageURL, "/") + "/" + strings.ReplaceAll(url.PathEscape(title), "%2F", "/")
}

func digest(text string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(text))) }
func normalize(text string) string {
	return strings.TrimRight(strings.ReplaceAll(text, "\r\n", "\n"), "\n") + "\n"
}

func marked(body string) string {
	body = normalize(body)
	// Keep #REDIRECT at the start of redirect pages.
	return body + markerPrefix + digest(body) + " -->\n"
}

func unmark(text string) (string, bool) {
	trimmed := strings.TrimRight(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	split := strings.LastIndex(trimmed, "\n")
	if split < 0 {
		return text, false
	}
	line, body := trimmed[split+1:], normalize(trimmed[:split])
	if !strings.HasPrefix(line, markerPrefix) {
		return text, false
	}
	expected := strings.TrimSuffix(strings.TrimPrefix(line, markerPrefix), " -->")
	return body, expected == digest(body)
}

func pandoc(input []byte, from, to string) ([]byte, error) {
	command := osexec.Command("pandoc", "--from", from, "--to", to, "--wrap=none")
	command.Stdin = bytes.NewReader(input)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("pandoc: %w: %s", err, stderr.String())
	}
	return output, nil
}

// Rewrite links in Pandoc's syntax tree, leaving code examples alone.
func convertLinks(input []byte, from, to string, rewrite func(string, string) (string, string, error)) ([]byte, error) {
	ast, err := pandoc(input, from, "json")
	if err != nil {
		return nil, err
	}
	var tree interface{}
	if err := json.Unmarshal(ast, &tree); err != nil {
		return nil, err
	}
	var linkErr error
	var walk func(interface{})
	walk = func(node interface{}) {
		switch value := node.(type) {
		case map[string]interface{}:
			if value["t"] == "Link" {
				fields := value["c"].([]interface{})
				target := fields[len(fields)-1].([]interface{})
				title := target[1].(string)
				// Recent Pandoc releases use a class instead of the legacy
				// title marker for MediaWiki links. Remove it for Markdown.
				if from == "mediawiki" {
					attributes := fields[0].([]interface{})
					classes := []interface{}{}
					for _, class := range attributes[1].([]interface{}) {
						if class == "wikilink" {
							title = "wikilink"
						} else {
							classes = append(classes, class)
						}
					}
					attributes[1] = classes
				}
				raw, title, err := rewrite(target[0].(string), title)
				if err != nil {
					linkErr = err
					return
				}
				target[0], target[1] = raw, title
				if to == "mediawiki" && title == "wikilink" {
					attributes := fields[0].([]interface{})
					attributes[1] = append(attributes[1].([]interface{}), "wikilink")
					target[1] = ""
				}
			}
			for _, child := range value {
				walk(child)
			}
		case []interface{}:
			for _, child := range value {
				walk(child)
			}
		}
	}
	walk(tree)
	if linkErr != nil {
		return nil, linkErr
	}
	ast, err = json.Marshal(tree)
	if err != nil {
		return nil, err
	}
	return pandoc(ast, "json", to)
}

func convertMarkdown(input []byte, source, page string, links map[string]string, config configuration) ([]byte, error) {
	wiki, err := url.Parse(config.PageURL)
	if err != nil {
		return nil, err
	}
	return convertLinks(input, "gfm", "mediawiki", func(raw, title string) (string, string, error) {
		parsed, err := url.Parse(raw)
		if err != nil {
			return "", "", err
		}
		if parsed.Scheme == "" && parsed.Host == "" && strings.HasSuffix(parsed.Path, ".md") {
			key := path.Clean(path.Join(path.Dir(source), parsed.Path))
			mapped, found := links[key]
			if !found {
				return "", "", fmt.Errorf("%s: unmapped documentation link %s", source, raw)
			}
			if parsed.Fragment != "" {
				mapped += "#" + parsed.EscapedFragment()
			}
			raw = mapped
			parsed, err = url.Parse(mapped)
			if err != nil {
				return "", "", err
			}
		}
		prefix := strings.TrimRight(wiki.Path, "/") + "/"
		if wiki.Host != "" && parsed.Scheme == wiki.Scheme && strings.EqualFold(parsed.Host, wiki.Host) && parsed.User == nil && parsed.RawQuery == "" && !parsed.ForceQuery && strings.HasPrefix(parsed.Path, prefix) {
			target := strings.TrimPrefix(parsed.Path, prefix)
			if target != "" {
				// Keep the established sibling-link style in command reference
				// pages instead of rewriting every SEE ALSO entry on release.
				referencePrefix := strings.TrimRight(config.ReferenceRoot, "/") + "/"
				if config.ReferenceRoot != "" && target != page && strings.HasPrefix(page, referencePrefix) && strings.HasPrefix(target, referencePrefix) {
					relative, err := filepath.Rel(page, target)
					if err != nil {
						return "", "", err
					}
					target = filepath.ToSlash(relative)
					if !strings.HasPrefix(target, "../") && !strings.HasPrefix(target, "./") {
						target = "./" + target
					}
				}
				// Category and file links must remain navigation, not add a
				// category or embed a file in the generated page.
				namespace, _, _ := strings.Cut(target, ":")
				if strings.EqualFold(namespace, "Category") || strings.EqualFold(namespace, "File") || strings.EqualFold(namespace, "Image") {
					target = ":" + target
				}
				if parsed.Fragment != "" {
					target += "#" + parsed.Fragment
				}
				return target, "wikilink", nil
			}
		}
		return raw, title, nil
	})
}

func convertWiki(input []byte, source string, config configuration) ([]byte, error) {
	return convertLinks(input, "mediawiki", "gfm", func(raw, title string) (string, string, error) {
		if title != "wikilink" {
			return raw, title, nil
		}
		target, fragment, _ := strings.Cut(raw, "#")
		if target == "" {
			return raw, "", nil
		}
		mapped := wikiURL(config, strings.TrimPrefix(target, ":"))
		for _, guide := range config.Guides {
			if strings.ReplaceAll(guide.Title, "_", " ") == strings.ReplaceAll(target, "_", " ") {
				relative, err := filepath.Rel(filepath.Dir(source), guide.Source)
				if err != nil {
					return "", "", err
				}
				mapped = filepath.ToSlash(relative)
				break
			}
		}
		if fragment != "" {
			mapped += "#" + fragment
		}
		return mapped, "", nil
	})
}

func generate(config configuration) error {
	if err := os.Setenv("MWCLI_SKIP_RENDER_MARKDOWN", "true"); err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(outputDir, "guides")); err != nil {
		return err
	}
	refDir := filepath.Join(outputDir, "ref")
	if err := os.RemoveAll(refDir); err != nil {
		return err
	}
	if err := os.MkdirAll(refDir, 0o755); err != nil {
		return err
	}
	root := cmd.NewMwCliCmd()
	pruneReferenceChildren(root)
	cobrautil.VisitAllSubCommands(root, func(c *cobra.Command) { c.Example = cobrautil.CommandExamples(c) })
	root.DisableAutoGenTag = true
	if err := doc.GenMarkdownTreeCustom(root, refDir, func(string) string { return "" }, func(name string) string {
		return wikiURL(config, config.ReferenceRoot+"/"+strings.TrimSuffix(name, ".md"))
	}); err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(refDir, "*.md"))
	if err != nil {
		return err
	}
	var documents []document
	links := map[string]string{}
	for _, file := range files {
		title := config.ReferenceRoot + "/" + strings.TrimSuffix(filepath.Base(file), ".md")
		documents = append(documents, document{title, "commands/", file, strings.TrimSuffix(file, ".md") + ".wiki"})
	}
	// Keep the stable reference entry point following the default executable name.
	index := document{config.ReferenceRoot, "commands/", filepath.Join(refDir, "index.md"), filepath.Join(refDir, "index.wiki")}
	if err := os.WriteFile(index.Markdown, []byte("[Command reference]("+wikiURL(config, config.ReferenceRoot+"/"+root.Name())+")\n"), 0o644); err != nil {
		return err
	}
	documents = append(documents, index)
	for _, guide := range config.Guides {
		source, _ := guidePath(guide.Source)
		links[guide.Source] = wikiURL(config, guide.Title)
		file := filepath.Join(outputDir, guide.Source)
		documents = append(documents, document{guide.Title, source, file, strings.TrimSuffix(file, ".md") + ".wiki"})
		data, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(file, data, 0o644); err != nil {
			return err
		}
	}
	version := currentDocumentationVersion()
	if err := writeJSON(filepath.Join(outputDir, "version.json"), version); err != nil {
		return err
	}
	for _, document := range documents {
		data, err := os.ReadFile(document.Markdown)
		if err != nil {
			return err
		}
		source := strings.TrimPrefix(filepath.ToSlash(document.Source), "docs/")
		converted, err := convertMarkdown(data, source, document.Title, links, config)
		if err != nil {
			return fmt.Errorf("%s: %w", document.Title, err)
		}
		body := string(converted)
		if document.Title == config.ReferenceRoot {
			body = "#REDIRECT [[" + config.ReferenceRoot + "/" + root.Name() + "]]\n"
		}
		if strings.HasPrefix(document.Title, config.ReferenceRoot+"/") {
			body = strings.TrimRight(body, "\r\n") + "\n__NOTOC__\n"
		}
		if err := os.WriteFile(document.Wikitext, []byte(marked(versionedDocumentation(body, version))), 0o644); err != nil {
			return err
		}
	}
	// Preserve links to the published docker command reference after making dev canonical.
	prefix := config.ReferenceRoot + "/" + root.Name() + "_dev"
	for _, item := range documents {
		if item.Title != prefix && !strings.HasPrefix(item.Title, prefix+"_") {
			continue
		}
		title := config.ReferenceRoot + "/" + root.Name() + "_docker" + strings.TrimPrefix(item.Title, prefix)
		file := filepath.Join(refDir, strings.TrimPrefix(title, config.ReferenceRoot+"/")+".md")
		redirect := document{title, "commands/", file, strings.TrimSuffix(file, ".md") + ".wiki"}
		if err := os.WriteFile(redirect.Markdown, []byte("[Command reference]("+wikiURL(config, item.Title)+")\n"), 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(redirect.Wikitext, []byte(marked("#REDIRECT [["+item.Title+"]]\n")), 0o644); err != nil {
			return err
		}
		documents = append(documents, redirect)
	}
	if err := writeJSON(filepath.Join(outputDir, "pages.json"), documents); err != nil {
		return err
	}
	fmt.Printf("Built %d documentation pages in %s\n", len(documents), outputDir)
	return nil
}

// pruneReferenceChildren applies publishing exclusions to a disposable command
// tree. Runtime help and completion keep the complete upstream command tree.
func pruneReferenceChildren(command *cobra.Command) {
	if command.Annotations["mwcli-docs-skip-children"] != "" {
		command.RemoveCommand(command.Commands()...)
		return
	}
	for _, child := range command.Commands() {
		pruneReferenceChildren(child)
	}
}
