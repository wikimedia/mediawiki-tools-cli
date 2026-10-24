package cmd

import (
	"strings"

	"github.com/spf13/cobra"
	analyticscmd "gitlab.wikimedia.org/repos/releng/cli/commands/analytics"
	eventscmd "gitlab.wikimedia.org/repos/releng/cli/commands/events"
	"gitlab.wikimedia.org/repos/releng/cli/commands/wiki"
)

// isWikimediaWikiHost limits shared Wikimedia services to wiki hostnames.
// Other explicit MediaWiki hosts retain their ordinary Action API commands.
func isWikimediaWikiHost(host string) bool {
	language, family, ok := strings.Cut(host, ".")
	if !ok || language == "" || strings.ContainsAny(language, "/\\?#@:% \t\r\n") {
		return false
	}
	switch family {
	case "wikipedia.org", "wiktionary.org", "wikibooks.org", "wikinews.org", "wikiquote.org", "wikisource.org", "wikiversity.org", "wikivoyage.org":
		return true
	}
	switch host {
	case "commons.wikimedia.org", "meta.wikimedia.org", "species.wikimedia.org", "incubator.wikimedia.org", "outreach.wikimedia.org", "wikimania.wikimedia.org", "strategy.wikimedia.org", "foundation.wikimedia.org", "wikitech.wikimedia.org", "www.mediawiki.org", "www.wikidata.org", "www.wikifunctions.org":
		return true
	}
	return false
}

// newWikiTargetCmd assembles independent providers under one site context.
// Each provider keeps its own flags and implementation, so the Action API's
// --wiki URL cannot become an EventStreams database-name filter accidentally.
func newWikiTargetCmd(host string) *cobra.Command {
	target := &cobra.Command{
		Use: "@" + host, GroupID: "service",
		Short: "Wiki content, analytics and events for " + host,
	}
	content := wiki.NewWikiCmd()
	endpoint := content.PersistentFlags().Lookup("wiki")
	_ = endpoint.Value.Set("https://" + host + "/w/api.php")
	endpoint.DefValue = endpoint.Value.String()
	delete(endpoint.Annotations, cobra.BashCompOneRequiredFlag)
	for _, command := range content.Commands() {
		command.PersistentFlags().AddFlagSet(content.PersistentFlags())
		target.AddCommand(command)
	}
	for _, command := range analyticscmd.NewCmd().Commands() {
		project := command.Flags().Lookup("project")
		if project == nil {
			continue
		}
		_ = project.Value.Set(host)
		project.DefValue = host
		target.AddCommand(command)
	}
	eventsCommand := &cobra.Command{Use: "events", Short: "Follow events for " + host}
	tail := eventscmd.NewTailCmd()
	domain := tail.Flags().Lookup("domain")
	_ = domain.Value.Set(host)
	domain.DefValue = host
	eventsCommand.AddCommand(tail)
	target.AddCommand(eventsCommand)
	return target
}
