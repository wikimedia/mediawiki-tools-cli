package phabricator

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

var objectReferencePattern = regexp.MustCompile(`(?i)^[TP][1-9][0-9]*$`)

type objectReference struct {
	Ref string
	URL *url.URL
}

func parseObjectReference(raw string) (objectReference, error) {
	ref := raw
	var target *url.URL
	if strings.Contains(raw, "://") {
		var err error
		target, err = url.Parse(raw)
		if err != nil || (target.Scheme != "https" && target.Scheme != "http") || target.Hostname() == "" || target.User != nil || target.RawQuery != "" || target.RawPath != "" {
			return objectReference{}, fmt.Errorf("invalid Phabricator object URL %q", raw)
		}
		parts := strings.Split(strings.TrimRight(target.Path, "/"), "/")
		ref = parts[len(parts)-1]
	}
	if !objectReferencePattern.MatchString(ref) {
		return objectReference{}, fmt.Errorf("invalid object reference %q: use T123, P123 or an object URL", raw)
	}
	if _, err := strconv.Atoi(ref[1:]); err != nil {
		return objectReference{}, fmt.Errorf("object reference number is too large")
	}
	return objectReference{Ref: strings.ToUpper(ref), URL: target}, nil
}

func (ref objectReference) checkEndpoint(endpoint string) error {
	if ref.URL == nil {
		return nil
	}
	base, err := url.Parse(endpoint)
	if err != nil || !strings.EqualFold(base.Host, ref.URL.Host) || base.Scheme != ref.URL.Scheme || strings.TrimRight(ref.URL.Path, "/") != strings.TrimRight(base.Path, "/")+"/"+ref.Ref {
		return fmt.Errorf("object URL does not belong to the selected Phabricator profile (%s); choose the matching --profile", endpoint)
	}
	return nil
}

func newPhabricatorObjectViewCmd(site *string, pasteOnly bool) *cobra.Command {
	var raw bool
	out := output.Output{PrettyBinding: func(objects interface{}, pretty *output.Pretty) {
		paste := objects.(*phabPaste)
		pretty.AddItem(fmt.Sprintf("P%d — %s", paste.ID, paste.Fields.Title), paste.Fields.URI)
		pretty.AddItem("Content", paste.Attachments.Content.Content)
	}}
	cmd := &cobra.Command{
		Use:     "view <T123|P123|URL>",
		Aliases: []string{"v"},
		Short:   "View a Phabricator task or paste",
		Long: "View a task or paste by its prefixed identifier or URL. URLs must belong to the selected profile. " +
			"Use --raw to print paste content exactly, without a heading or added newline.",
		Example: "view T123\nview P123\nview https://phabricator.wikimedia.org/P123 --raw",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := parseObjectReference(args[0])
			if err != nil {
				return err
			}
			if pasteOnly && ref.Ref[0] != 'P' {
				return fmt.Errorf("expected a paste reference such as P123")
			}
			if raw && ref.Ref[0] != 'P' {
				return fmt.Errorf("--raw is only supported for pastes")
			}
			if raw && cmd.Flags().Changed("output") {
				return fmt.Errorf("--raw cannot be combined with --output")
			}
			if out.Type != "pretty" && out.Type != "json" && out.Type != "jq" {
				return fmt.Errorf("unsupported output format %q", out.Type)
			}
			if out.Type == "jq" && out.Format == "" {
				return fmt.Errorf("--output jq requires --format")
			}
			client, _, err := clientFromSite(*site)
			if err != nil {
				return err
			}
			if err := ref.checkEndpoint(client.baseURL); err != nil {
				return err
			}
			if ref.Ref[0] == 'T' {
				task, err := client.getTask(ref.Ref)
				if err != nil {
					return err
				}
				if out.Type == "pretty" {
					renderTask(task, client)
				} else {
					out.Print(cmd, task)
				}
				client.cache.flush()
				return nil
			}
			paste, err := client.getPaste(ref.Ref)
			if err != nil {
				return err
			}
			if raw {
				_, err = fmt.Fprint(cmd.OutOrStdout(), paste.Attachments.Content.Content)
				return err
			}
			out.Print(cmd, paste)
			return nil
		},
	}
	if pasteOnly {
		cmd.Use = "view <P123|URL>"
		cmd.Short = "View a Phabricator paste"
		cmd.Example = "view P123\nview P123 --raw\nview P123 --output json"
	}
	cmd.Flags().BoolVar(&raw, "raw", false, "Print only the exact paste content")
	out.AddFlagsWithOpts(cmd, output.WithDefaultOutput(output.PrettyType), output.WithFilterFlagDisabled())
	return cmd
}
