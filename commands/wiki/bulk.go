package wiki

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiapi"
)

func newWikiBulkCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "bulk", Short: "Explicit JSONL edits with per-record outcomes"}
	var input string
	options := wikiapi.BulkOptions{}
	apply := &cobra.Command{
		Use: "apply", Short: "Apply explicit JSONL edits or preview them", Args: cobra.NoArgs, SilenceUsage: true,
		Long:    "Read one explicit edit object per line from --input FILE or --input -. Each record requires operation and summary; updates require a positive baserevid. Supported operations: page-put, entity-create, term-change, statement-add/update/remove, qualifier-add/remove, reference-add/remove. Page creation requires createonly=true and baserevid=0. Entity creation requires baserevid=0. Kind must be item or property for Wikibase operations.\n\nResults are streamed as NDJSON: line, optional record_id, operation, status and result/error. --dry-run validates and reads preconditions but fetches no CSRF tokens and writes nothing. Errors are reported per record and cause a failing final exit status. An ambiguous POST outcome is marked error.ambiguous=true: inspect the wiki before retrying that record. No failed writes are retried automatically. There is no implicit bridge from page-list output to edits, and no transaction across records.\n\nExample record: {\"operation\":\"term-change\",\"kind\":\"item\",\"entity\":\"Q42\",\"term\":\"label\",\"change\":\"set\",\"language\":\"en\",\"values\":[\"Example\"],\"baserevid\":123,\"summary\":\"Correct label\"}",
		Example: "  apply --input edits.jsonl --dry-run\n  apply --input - --stop-on-error",
		RunE: func(cmd *cobra.Command, args []string) error {
			if input == "" {
				return fmt.Errorf("provide --input FILE or --input -")
			}
			var reader io.Reader = cmd.InOrStdin()
			if input != "-" {
				file, err := os.Open(input)
				if err != nil {
					return err
				}
				defer file.Close()
				reader = file
			}
			client, err := wikiAPIClient(false)
			if err != nil {
				return err
			}
			_, err = client.RunBulk(cmd.Context(), reader, cmd.OutOrStdout(), options)
			return err
		},
	}
	apply.Flags().StringVar(&input, "input", "", "JSONL edit file, or - for stdin (required)")
	_ = apply.MarkFlagFilename("input")
	apply.Flags().BoolVar(&options.DryRun, "dry-run", false, "Preview validated records without CSRF requests or wiki writes")
	apply.Flags().BoolVar(&options.StopOnError, "stop-on-error", false, "Stop after the first failed record (earlier successes remain applied)")
	cmd.AddCommand(apply)
	return cmd
}
