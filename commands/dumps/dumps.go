package dumps

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	ossignal "os/signal"
	"strings"
	"syscall"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

type options struct {
	mirror        string
	approvedHosts []string
}

func NewCmd() *cobra.Command {
	return newCmd(confirm)
}

func newCmd(ask func(*cobra.Command, string) error) *cobra.Command {
	opts := &options{}
	cmd := &cobra.Command{
		Use:     "dumps",
		GroupID: "service",
		Short:   "Explore and download Wikimedia dumps",
		Long: `Explore Wikimedia dump directories and download individual files without
decompressing or interpreting their format. Start with datasets to find content
exports, SQL tables, Wikidata JSON/RDF, Commons structured data and other datasets.
Use list with a shortcut or any relative path from dumps.wikimedia.org.

The current/history shortcuts use the replacement MediaWiki Content File
Exports. Legacy XML content dumps are scheduled to retire in January 2027;
this does not mean SQL tables or Wikidata JSON/RDF dumps are being retired.
File availability, formats and mirror coverage vary by dataset and date.
Known mirror directory layouts are mapped automatically, including Umea's
dumps/ directory and mirrored Wikibase entity directories. Paths shown by list
can be passed to list, download or mirrors test. Unavailable or unreliable
latest aliases are rejected with guidance to select a dated path; dates are
never silently substituted, and mirrors need not contain the same snapshots.

Wikimedia is the default source. Non-Wikimedia hosts require confirmation before
contact, including redirects. --approve-host grants explicit per-invocation
consent to an exact host (including its port when nonstandard); no trust is saved.
Use mirrors test to compare a bounded sample only after approval. No server
is selected automatically and no files are downloaded just by listing mirrors.

See https://dumps.wikimedia.org/ and
https://wikitech.wikimedia.org/wiki/MediaWiki_Content_File_Exports`,
	}
	cmd.PersistentFlags().StringVar(&opts.mirror, "mirror", "wikimedia", "Known mirror name or HTTPS base URL")
	cmd.PersistentFlags().StringSliceVar(&opts.approvedHosts, "approve-host", nil, "Explicitly approve contacting a non-Wikimedia host (repeatable)")
	cmd.AddCommand(newDatasetsCmd(), newListCmd(opts, ask), newDownloadCmd(opts, ask), newMirrorsCmd(opts, ask))
	return cmd
}

func confirm(cmd *cobra.Command, message string) error {
	if !cli.CanAskForInput() {
		return fmt.Errorf("%s: approval required; use an interactive terminal or the explicit approval flag", message)
	}
	approved := false
	input, inOK := cmd.InOrStdin().(*os.File)
	output, outOK := cmd.ErrOrStderr().(*os.File)
	if !inOK || !outOK {
		return fmt.Errorf("approval requires terminal input and error output")
	}
	err := survey.AskOne(&survey.Confirm{Message: message, Default: false}, &approved,
		survey.WithStdio(input, output, output))
	if err != nil {
		return err
	}
	if !approved {
		return fmt.Errorf("cancelled: permission was not granted")
	}
	return nil
}

func (o *options) client(cmd *cobra.Command, ask func(*cobra.Command, string) error) *dumpClient {
	approved := make(map[string]bool)
	for _, host := range o.approvedHosts {
		approved[strings.ToLower(host)] = true
	}
	return newClient(func(u *url.URL) error {
		if u.User != nil || u.Fragment != "" {
			return fmt.Errorf("dump URLs must not contain credentials or fragments")
		}
		host := strings.ToLower(u.Host)
		name := strings.ToLower(u.Hostname())
		if (name == "wikimedia.org" || strings.HasSuffix(name, ".wikimedia.org")) && (u.Port() == "" || u.Port() == "443") {
			return nil
		}
		if approved[host] {
			return nil
		}
		if err := ask(cmd, fmt.Sprintf("Contact non-Wikimedia host %s? (or explicitly pass --approve-host %s)", host, host)); err != nil {
			return err
		}
		approved[host] = true
		return nil
	})
}

func newDatasetsCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use: "datasets", Short: "List starting points for Wikimedia dump datasets",
		Long:    "List built-in discovery shortcuts. Other dump paths and formats remain accessible through list and download. This catalog is not an exhaustive file-format list and does not make network requests.",
		Example: cobrautil.NormalizeExample("datasets\ndatasets --output json"),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			rows := make([][]string, len(datasets))
			for i, d := range datasets {
				rows[i] = []string{d.Name, d.Path, d.Description}
			}
			return printRows(cmd, format, datasets, []string{"Shortcut", "Path", "Description"}, rows)
		},
	}
	cmd.Flags().StringVar(&format, "output", "table", "Output format: table or json")
	return cmd
}

func newListCmd(opts *options, ask func(*cobra.Command, string) error) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use: "list [path]", Short: "Browse a dump directory",
		Long: `List linked files, directories and dataset pages at the selected source. Omit the
path to browse the root; use datasets for shortcuts. Paths use Wikimedia's
layout, for example current/enwiki/ or wikidata/; known mirror-specific
directories are mapped automatically. Listing does not download
the dump files. HTML landing pages such as other/pageviews/readme.html can also
be explored; links outside the selected source root are omitted. A directory
can be incomplete while an export is being published;
content exports are ready only when their SHA256SUMS file exists.`,
		Example:           cobrautil.NormalizeExample("list\nlist current/enwiki\nlist wikidata\nlist other --output json"),
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateFormat(format); err != nil {
				return err
			}
			relative := ""
			if len(args) > 0 {
				relative = args[0]
			}
			relative, err := dumpPath(relative)
			if err != nil {
				return err
			}
			base, err := mirrorURL(opts.mirror)
			if err != nil {
				return err
			}
			client := opts.client(cmd, ask)
			defer client.http.CloseIdleConnections()
			entries, err := client.list(cmd.Context(), base, relative)
			if err != nil {
				return err
			}
			rows := make([][]string, len(entries))
			for i, e := range entries {
				rows[i] = []string{e.Name, e.Path}
			}
			return printRows(cmd, format, entries, []string{"Name", "Path"}, rows)
		},
	}
	cmd.Flags().StringVar(&format, "output", "table", "Output format: table or json")
	return cmd
}

func validateFormat(format string) error {
	if format != "table" && format != "json" {
		return fmt.Errorf("--output must be table or json")
	}
	return nil
}

func printRows(cmd *cobra.Command, format string, data interface{}, headings []string, rows [][]string) error {
	if err := validateFormat(format); err != nil {
		return err
	}
	if format == "json" {
		return output.PrintJSONChecked(data, ".", false, cmd.OutOrStdout())
	}
	table := &output.Table{}
	table.AddHeadingsS(headings...)
	for _, row := range rows {
		table.AddRowS(row...)
	}
	table.Print(cmd.OutOrStdout())
	return nil
}

func runCancelable(cmd *cobra.Command, run func(context.Context) error) error {
	ctx, stop := ossignal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	err := run(ctx)
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("operation cancelled: %w", err)
	}
	return err
}
