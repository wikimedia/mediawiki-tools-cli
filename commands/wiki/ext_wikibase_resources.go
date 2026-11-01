package wiki

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiapi"
)

func addWikibaseResources(cmd *cobra.Command, kind string) {
	cmd.AddCommand(newEntityGetCmd(kind), newEntitySearchCmd(kind), newEntityCreateCmd(kind))
	for _, term := range []string{"label", "description", "alias"} {
		cmd.AddCommand(newEntityTermCmd(kind, term))
	}
}

func newEntityGetCmd(kind string) *cobra.Command {
	var languages []string
	var fallback bool
	out := apiOutput{}
	cmd := &cobra.Command{
		Use: "get ID", Short: "Read a Wikibase " + kind, Args: cobra.ExactArgs(1), Example: "  get " + entityExampleID(kind) + " --language en",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := out.validate(); err != nil {
				return err
			}
			if _, err := wikiapi.EntityID(kind, args[0]); err != nil {
				return err
			}
			if err := wikiapi.ValidateLanguages(languages); err != nil {
				return err
			}
			client, err := wikiAPIClient(false)
			if err != nil {
				return err
			}
			entity, err := client.Entity(cmd.Context(), kind, args[0], languages, "", fallback)
			if err != nil {
				return err
			}
			data, err := json.Marshal(entity)
			if err != nil {
				return err
			}
			return out.print(cmd, data)
		},
	}
	cmd.Flags().StringSliceVar(&languages, "language", nil, "Restrict terms to these language codes (repeatable or comma-separated; default: all)")
	cmd.Flags().BoolVar(&fallback, "language-fallback", false, "Allow the API's language fallback for returned terms")
	out.flags(cmd)
	return cmd
}

func entityExampleID(kind string) string {
	if kind == "property" {
		return "P31"
	}
	return "Q42"
}

func newEntitySearchCmd(kind string) *cobra.Command {
	var language string
	var limit int
	out := output.Output{}
	cmd := &cobra.Command{
		Use: "search QUERY", Short: "Search Wikibase " + kind + " labels and aliases", Args: cobra.ExactArgs(1), Example: "  search 'Douglas Adams' --language en --limit 10",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateResourceOutput(&out, false); err != nil {
				return err
			}
			if limit < 0 {
				return fmt.Errorf("--limit must be nonnegative")
			}
			if err := wikiapi.ValidateLanguages([]string{language}); err != nil {
				return err
			}
			client, err := wikiAPIClient(false)
			if err != nil {
				return err
			}
			results, err := client.SearchEntities(cmd.Context(), kind, args[0], language, limit)
			if err != nil {
				return err
			}
			out.Print(cmd, results)
			return nil
		},
	}
	cmd.Flags().StringVar(&language, "language", "en", "Language for matching and displaying terms")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum distinct results (0 means unlimited)")
	out.AddFlagsWithOpts(cmd, output.WithDefaultTTY(output.TableType), output.WithDefaultPipe(output.JSONType), output.WithFilterFlagDisabled(), output.WithTableBinding(&output.TableBinding{Headings: []string{"ID", "LABEL", "DESCRIPTION"}, ProcessObjects: func(objects interface{}, table *output.Table) {
		for _, entity := range objects.([]wikiapi.EntitySearchResult) {
			table.AddRowS(entity.ID, entity.Label, entity.Description)
		}
	}}))
	return cmd
}

type entityEditFlags struct {
	wikiapi.EditOptions
	dryRun bool
}

func (options *entityEditFlags) flags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&options.Summary, "summary", cli.DisplayName+": update Wikibase entity", "Edit summary")
	cmd.Flags().Int64Var(&options.BaseRevision, "baserevid", 0, "Expected revision for an existing entity (default: read current revision before editing)")
	cmd.Flags().BoolVar(&options.Bot, "bot", false, "Mark the edit as a bot edit")
	cmd.Flags().BoolVar(&options.dryRun, "dry-run", false, "Show the proposed edit without fetching a CSRF token or writing")
}

func readEntityJSON(cmd *cobra.Command, input string) ([]byte, error) {
	var reader io.Reader = cmd.InOrStdin()
	if input == "" {
		return nil, fmt.Errorf("provide --input FILE, or --input - for stdin")
	}
	if input != "-" {
		file, err := os.Open(input)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		reader = file
	}
	const maximum = 8 * 1024 * 1024
	data, err := io.ReadAll(io.LimitReader(reader, maximum+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maximum {
		return nil, fmt.Errorf("entity JSON exceeds 8 MiB")
	}
	return data, nil
}

func newEntityCreateCmd(kind string) *cobra.Command {
	var input string
	options := entityEditFlags{}
	out := apiOutput{}
	cmd := &cobra.Command{
		Use: "create", Short: "Create a Wikibase " + kind + " from JSON", Args: cobra.NoArgs, Example: "  create --input entity.json --summary 'Create example' --dry-run",
		Long: "Create a new entity from a JSON object. Input must not contain an existing entity id; properties require a datatype. Unlike the legacy put command, no labels or claims are invented. JSON can be read from stdin with --input -. This command creates one entity, not a batch.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := out.validate(); err != nil {
				return err
			}
			if options.BaseRevision != 0 {
				return fmt.Errorf("--baserevid applies to existing entities only")
			}
			data, err := readEntityJSON(cmd, input)
			if err != nil {
				return err
			}
			entity, err := wikiapi.ValidateEntityCreate(kind, data)
			if err != nil {
				return err
			}
			if options.dryRun {
				return printEntityPlan(cmd, &out, map[string]interface{}{"action": "wbeditentity", "new": kind, "data": entity, "summary": options.Summary, "bot": options.Bot, "wiki": wiki})
			}
			client, err := wikiAPIClient(false)
			if err != nil {
				return err
			}
			result, err := client.CreateEntity(cmd.Context(), kind, data, options.EditOptions)
			if err != nil {
				return err
			}
			return printEntityResponse(cmd, &out, result)
		},
	}
	cmd.Flags().StringVar(&input, "input", "", "Entity JSON file, or - for stdin")
	_ = cmd.MarkFlagFilename("input")
	options.flags(cmd)
	out.flags(cmd)
	return cmd
}

func printEntityPlan(cmd *cobra.Command, out *apiOutput, plan interface{}) error {
	data, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	return out.print(cmd, data)
}

func printEntityResponse(cmd *cobra.Command, out *apiOutput, data []byte) error {
	var response map[string]json.RawMessage
	if json.Unmarshal(data, &response) == nil {
		if entity, ok := response["entity"]; ok {
			return out.print(cmd, entity)
		}
	}
	return out.print(cmd, data)
}

func newEntityTermCmd(kind, term string) *cobra.Command {
	cmd := &cobra.Command{Use: term, Short: "Read or change " + kind + " " + term + " terms"}
	var languages []string
	var fallback bool
	out := apiOutput{}
	get := &cobra.Command{
		Use: "get ID", Short: "Read " + term + " terms by language", Args: cobra.ExactArgs(1), Example: "  get " + entityExampleID(kind) + " --language en",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := out.validate(); err != nil {
				return err
			}
			if _, err := wikiapi.EntityID(kind, args[0]); err != nil {
				return err
			}
			if err := wikiapi.ValidateLanguages(languages); err != nil {
				return err
			}
			client, err := wikiAPIClient(false)
			if err != nil {
				return err
			}
			property, _ := wikiapi.TermProperty(term)
			entity, err := client.Entity(cmd.Context(), kind, args[0], languages, property, fallback)
			if err != nil {
				return err
			}
			data := entity[property]
			if len(data) == 0 {
				data = json.RawMessage(`{}`)
			}
			return out.print(cmd, data)
		},
	}
	get.Flags().StringSliceVar(&languages, "language", nil, "Restrict returned language codes (repeatable or comma-separated; default: all)")
	get.Flags().BoolVar(&fallback, "language-fallback", false, "Allow the API's term language fallback where supported")
	out.flags(get)
	cmd.AddCommand(get)
	operations := []string{"set", "remove"}
	if term == "alias" {
		operations = []string{"set", "add", "remove"}
	}
	for _, operation := range operations {
		cmd.AddCommand(newEntityTermChangeCmd(kind, term, operation))
	}
	return cmd
}

func newEntityTermChangeCmd(kind, term, operation string) *cobra.Command {
	options := entityEditFlags{}
	out := apiOutput{}
	var language string
	use := "ID VALUE"
	args := cobra.ExactArgs(2)
	if term == "alias" {
		use = "ID VALUE..."
		args = cobra.MinimumNArgs(2)
	} else if operation == "remove" {
		use = "ID"
		args = cobra.ExactArgs(1)
	}
	cmd := &cobra.Command{
		Use: operation + " " + use, Short: strings.ToUpper(operation[:1]) + operation[1:] + " " + kind + " " + term + " terms", Args: args,
		Example: "  " + operation + " " + entityExampleID(kind) + func() string {
			if term != "alias" && operation == "remove" {
				return ""
			}
			return " 'Example'"
		}() + " --language en --summary 'Update term' --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEntityTermChange(cmd, &out, &options, kind, args[0], term, operation, language, args[1:])
		},
	}
	cmd.Flags().StringVar(&language, "language", "en", "Language code for the changed terms")
	options.flags(cmd)
	out.flags(cmd)
	return cmd
}

func runEntityTermChange(cmd *cobra.Command, out *apiOutput, options *entityEditFlags, kind, id, term, operation, language string, values []string) error {
	if err := out.validate(); err != nil {
		return err
	}
	id, err := wikiapi.EntityID(kind, id)
	if err != nil {
		return err
	}
	if err = wikiapi.ValidateTermChange(term, operation, language, values); err != nil {
		return err
	}
	if options.BaseRevision < 0 {
		return fmt.Errorf("--baserevid must be nonnegative")
	}
	client, err := wikiAPIClient(false)
	if err != nil {
		return err
	}
	entity, err := client.Entity(cmd.Context(), kind, id, nil, "info", false)
	if err != nil {
		return err
	}
	revision, err := entity.Revision()
	if err != nil {
		return err
	}
	edit := options.EditOptions
	if edit.BaseRevision == 0 {
		edit.BaseRevision = revision
	} else if edit.BaseRevision != revision {
		return fmt.Errorf("entity %s is at revision %d, expected %d", id, revision, edit.BaseRevision)
	}
	if options.dryRun {
		return printEntityPlan(cmd, out, map[string]interface{}{"entity": id, "term": term, "operation": operation, "language": language, "values": values, "baserevid": edit.BaseRevision, "summary": edit.Summary, "bot": edit.Bot, "wiki": client.Endpoint})
	}
	result, err := client.ChangeTerm(cmd.Context(), kind, id, term, operation, language, values, edit)
	if err != nil {
		return err
	}
	return printEntityResponse(cmd, out, result)
}
