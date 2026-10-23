package help

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/docs"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

// AddGuides extends Cobra's help command without replacing command-reference help.
func AddGuides(root *cobra.Command) {
	manifest, err := docs.Load()
	if err != nil {
		panic(fmt.Sprintf("load bundled guides: %v", err))
	}
	root.InitDefaultHelpCmd()
	help, _, err := root.Find([]string{"help"})
	if err != nil {
		panic(err)
	}
	if help.Annotations == nil {
		help.Annotations = make(map[string]string)
	}
	help.Annotations["mwcli-lint-skip"] = "Cobra help accepts a multi-word command path"
	// Reading documentation must not run configuration or authentication wizards.
	help.Short = "Help about commands and workflow guides (" + root.Name() + " help guides)"
	help.PersistentPreRun = func(*cobra.Command, []string) {}
	showTopics := func(c *cobra.Command, _ []string) error {
		web, _ := c.Flags().GetBool("web")
		if web {
			return fmt.Errorf("choose a guide to open on the wiki: %s [guide] --web", c.CommandPath())
		}
		return c.Help()
	}
	guides := &cobra.Command{
		Use: "guides", Short: "Read workflow guides offline or open them on the wiki",
		Args: cobra.NoArgs,
	}
	guides.PersistentFlags().Bool("web", false, "Open the guide on MediaWiki.org")
	guides.RunE = showTopics
	help.AddCommand(guides)
	for _, guide := range manifest.Guides {
		current := guides
		for _, part := range guide.Topic() {
			var next *cobra.Command
			for _, child := range current.Commands() {
				if child.Name() == part {
					next = child
					break
				}
			}
			if next == nil {
				next = &cobra.Command{Use: part, Short: "Guides for " + part, Args: cobra.NoArgs}
				next.RunE = showTopics
				current.AddCommand(next)
			}
			current = next
		}
		source, err := docs.Files.ReadFile(guide.Source)
		if err != nil {
			panic(fmt.Sprintf("read bundled guide: %v", err))
		}
		for _, line := range strings.Split(string(source), "\n") {
			if strings.HasPrefix(line, "# ") {
				current.Short = strings.TrimPrefix(line, "# ")
				break
			}
		}
		current.RunE = func(c *cobra.Command, _ []string) error {
			web, _ := c.Flags().GetBool("web")
			if web {
				return browser.OpenURL(guideURL(manifest, guide))
			}
			markdown := guideMarkdown(string(source), guide, manifest, c.Root().Name())
			out := c.OutOrStdout()
			fmt.Fprint(out, cli.RenderMarkdown(markdown))
			// Folder guides can also have an introductory page, such as GitLab.
			if c.HasSubCommands() {
				fmt.Fprintln(out, "\nGuide topics:")
				for _, child := range c.Commands() {
					fmt.Fprintf(out, "  %s  %s\n", child.CommandPath(), child.Short)
				}
			}
			return nil
		}
	}
}

func guideURL(manifest docs.Manifest, guide docs.Guide) string {
	return strings.TrimRight(manifest.PageURL, "/") + "/" + strings.ReplaceAll(url.PathEscape(guide.Title), "%2F", "/")
}

// Preserve code examples while replacing prose links with offline help commands.
var markdownTokens = regexp.MustCompile("(?m)(?s:```[^\\n]*\\n.*?\\n```|~~~[^\\n]*\\n.*?\\n~~~|`[^`\\n]+`|<!--.*?-->)|\\[([^]\\n]+)\\]\\(([^)\\n]+)\\)")

func guideMarkdown(source string, guide docs.Guide, manifest docs.Manifest, executable string) string {
	result := markdownTokens.ReplaceAllStringFunc(source, func(token string) string {
		if strings.HasPrefix(token, "<!--") {
			return ""
		}
		match := markdownTokens.FindStringSubmatch(token)
		if len(match) < 3 || match[2] == "" {
			return token
		}
		parsed, err := url.Parse(match[2])
		if err != nil || parsed.IsAbs() || parsed.Host != "" || !strings.HasSuffix(parsed.Path, ".md") {
			return token
		}
		target := path.Clean(path.Join(path.Dir(guide.Source), parsed.Path))
		for _, candidate := range manifest.Guides {
			if candidate.Source == target {
				command := strings.TrimSpace(executable + " help guides " + strings.Join(candidate.Topic(), " "))
				return match[1] + " (`" + command + "`)"
			}
		}
		return token
	})
	return strings.ReplaceAll(result, "mw ", executable+" ")
}
