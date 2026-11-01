package wiki

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiapi"
)

type snakFlags struct{ datatype, value, valueJSON, snakType string }

func (v *snakFlags) flags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&v.datatype, "datatype", "", "Typed datatype: string, external-id, url, commonsMedia, wikibase-item, wikibase-property, monolingualtext, quantity, time, globe-coordinate")
	cmd.Flags().StringVar(&v.value, "value", "", "Scalar string/entity-id/quantity value")
	cmd.Flags().StringVar(&v.valueJSON, "value-json", "", "Native datavalue.value JSON file, or - for stdin (complex typed values)")
	_ = cmd.MarkFlagFilename("value-json")
	cmd.Flags().StringVar(&v.snakType, "snaktype", "value", "Snak type: value, somevalue or novalue")
	cmd.MarkFlagsMutuallyExclusive("value", "value-json")
}

func (v *snakFlags) read(cmd *cobra.Command, property string) (wikiapi.Snak, error) {
	var value json.RawMessage
	if v.valueJSON != "" {
		data, err := readEntityJSON(cmd, v.valueJSON)
		if err != nil {
			return nil, err
		}
		if !json.Valid(data) {
			return nil, fmt.Errorf("--value-json must contain valid JSON")
		}
		value = data
	}
	return wikiapi.NewSnak(property, v.datatype, v.snakType, v.value, value)
}

func newEntityStatementCmd(kind string) *cobra.Command {
	cmd := &cobra.Command{Use: "statement", Short: "Read or edit individual Wikibase statements"}
	cmd.AddCommand(newStatementReadCmd(kind, false), newStatementReadCmd(kind, true))
	for _, operation := range []string{"add", "update", "remove"} {
		cmd.AddCommand(newStatementEditCmd(kind, operation))
	}
	cmd.AddCommand(newStatementQualifierCmd(kind), newStatementReferenceCmd(kind))
	return cmd
}

func newStatementReadCmd(kind string, single bool) *cobra.Command {
	var property string
	out := apiOutput{}
	use := "list ID"
	args := cobra.ExactArgs(1)
	example := "  list " + entityExampleID(kind) + " --property P31"
	if single {
		use = "get ID STATEMENT_ID"
		args = cobra.ExactArgs(2)
		example = "  get " + entityExampleID(kind) + " STATEMENT_ID"
	}
	cmd := &cobra.Command{
		Use: use, Short: "Read native statement JSON including qualifiers and references", Args: args, Example: example, ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := out.validate(); err != nil {
				return err
			}
			if _, err := wikiapi.EntityID(kind, args[0]); err != nil {
				return err
			}
			if property != "" {
				if _, err := wikiapi.EntityID("property", property); err != nil {
					return err
				}
			}
			client, err := wikiAPIClient(false)
			if err != nil {
				return err
			}
			entity, err := client.Entity(cmd.Context(), kind, args[0], nil, "claims", false)
			if err != nil {
				return err
			}
			var result interface{}
			if single {
				result, err = wikiapi.FindStatement(entity, args[1])
			} else {
				result, err = wikiapi.EntityStatements(entity, property)
			}
			if err != nil {
				return err
			}
			return printEntityPlan(cmd, &out, result)
		},
	}
	if !single {
		cmd.Flags().StringVar(&property, "property", "", "Restrict statements to one property")
	}
	out.flags(cmd)
	return cmd
}

func newStatementEditCmd(kind, operation string) *cobra.Command {
	out := apiOutput{}
	options := entityEditFlags{}
	values := snakFlags{}
	var input, rank string
	var rawJSON bool
	use := operation + " ID"
	args := cobra.RangeArgs(1, 2)
	if operation == "update" {
		use += " STATEMENT_ID [PROPERTY]"
		args = cobra.RangeArgs(2, 3)
	} else if operation == "remove" {
		use += " STATEMENT_ID"
		args = cobra.ExactArgs(2)
	} else {
		use += " [PROPERTY]"
	}
	cmd := &cobra.Command{
		Use: use, Short: "Apply one revision-guarded statement " + operation, Args: args, ValidArgsFunction: cobra.NoFileCompletions,
		Long: "Read the entity revision before editing one statement. Scalar edits preserve existing qualifiers/references and rank; --input supplies a complete native statement replacement. Statement IDs are opaque and checked against this entity. Supported typed datatypes are listed under --datatype; --raw-json with --input skips datatype-specific validation for other native types. Property/datatype compatibility is finally checked by the wiki. Dry-run reads state but fetches no CSRF token and writes nothing. Failed writes are never retried automatically.",
		Example: "  " + operation + " " + entityExampleID(kind) + func() string {
			if operation == "remove" {
				return " STATEMENT_ID --dry-run"
			}
			if operation == "update" {
				return " STATEMENT_ID P31 --datatype wikibase-item --value Q5 --dry-run"
			}
			return " P31 --datatype wikibase-item --value Q5 --dry-run"
		}(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := out.validate(); err != nil {
				return err
			}
			if _, err := wikiapi.EntityID(kind, args[0]); err != nil {
				return err
			}
			change := wikiapi.StatementChange{Operation: operation, Rank: rank}
			if operation != "add" {
				change.ID = args[1]
			}
			if operation != "remove" {
				typedCount := 2
				if operation == "update" {
					typedCount = 3
				}
				if input != "" {
					if len(args) != typedCount-1 || cmd.Flags().Changed("datatype") || cmd.Flags().Changed("value") || cmd.Flags().Changed("value-json") || cmd.Flags().Changed("snaktype") || rank != "" {
						return fmt.Errorf("--input cannot be combined with typed value/rank arguments")
					}
					data, err := readEntityJSON(cmd, input)
					if err != nil {
						return err
					}
					if err = json.Unmarshal(data, &change.Statement); err != nil {
						return err
					}
					if err = wikiapi.ValidateStatement(change.Statement, rawJSON); err != nil {
						return err
					}
				} else {
					if len(args) != typedCount {
						return fmt.Errorf("provide PROPERTY and typed value flags, or --input with native statement JSON")
					}
					if rawJSON {
						return fmt.Errorf("--raw-json requires --input")
					}
					snak, err := values.read(cmd, args[len(args)-1])
					if err != nil {
						return err
					}
					change.Snak = snak
				}
			}
			client, err := wikiAPIClient(false)
			if err != nil {
				return err
			}
			plan, err := client.PlanStatement(cmd.Context(), kind, args[0], change, options.BaseRevision, rawJSON)
			if err != nil {
				return err
			}
			return finishWikibasePlan(cmd, &out, &options, client, plan)
		},
	}
	if operation != "remove" {
		cmd.Flags().StringVar(&input, "input", "", "Complete native statement JSON file, or - for stdin")
		_ = cmd.MarkFlagFilename("input")
		cmd.Flags().BoolVar(&rawJSON, "raw-json", false, "Allow native JSON datatypes outside the typed subset (does not bypass revision/ownership checks)")
		cmd.Flags().StringVar(&rank, "rank", "", "Statement rank: normal, preferred or deprecated (default: normal for add; preserve for typed update)")
		values.flags(cmd)
	}
	options.flags(cmd)
	out.flags(cmd)
	return cmd
}

func finishWikibasePlan(cmd *cobra.Command, out *apiOutput, options *entityEditFlags, client *wikiapi.Client, plan *wikiapi.WikibaseEditPlan) error {
	if options.dryRun {
		return printEntityPlan(cmd, out, map[string]interface{}{"wiki": client.Endpoint, "plan": plan, "summary": options.Summary, "bot": options.Bot})
	}
	result, err := client.ExecuteWikibasePlan(cmd.Context(), plan, options.EditOptions)
	if err != nil {
		return err
	}
	return printEntityResponse(cmd, out, result)
}

func statementPartListCmd(kind, part string) *cobra.Command {
	out := apiOutput{}
	cmd := &cobra.Command{
		Use: "list ID STATEMENT_ID", Short: "Read native " + part + " from a statement", Args: cobra.ExactArgs(2), ValidArgsFunction: cobra.NoFileCompletions, Example: "  list " + entityExampleID(kind) + " STATEMENT_ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := out.validate(); err != nil {
				return err
			}
			client, err := wikiAPIClient(false)
			if err != nil {
				return err
			}
			entity, err := client.Entity(cmd.Context(), kind, args[0], nil, "claims", false)
			if err != nil {
				return err
			}
			statement, err := wikiapi.FindStatement(entity, args[1])
			if err != nil {
				return err
			}
			data := statement[part]
			if len(data) == 0 {
				if part == "references" {
					data = json.RawMessage(`[]`)
				} else {
					data = json.RawMessage(`{}`)
				}
			}
			return out.print(cmd, data)
		},
	}
	out.flags(cmd)
	return cmd
}

func newStatementQualifierCmd(kind string) *cobra.Command {
	cmd := &cobra.Command{Use: "qualifier", Short: "Read, add or remove qualifier snaks"}
	cmd.AddCommand(statementPartListCmd(kind, "qualifiers"))
	for _, operation := range []string{"add", "remove"} {
		out := &apiOutput{}
		options := &entityEditFlags{}
		values := &snakFlags{}
		var input string
		var rawJSON bool
		use := operation + " ID STATEMENT_ID HASH"
		args := cobra.ExactArgs(3)
		if operation == "add" {
			use = operation + " ID STATEMENT_ID [PROPERTY]"
			args = cobra.RangeArgs(2, 3)
		}
		leaf := &cobra.Command{
			Use: use, Short: "Apply one revision-guarded qualifier " + operation, Args: args, ValidArgsFunction: cobra.NoFileCompletions,
			Example: "  " + operation + " " + entityExampleID(kind) + " STATEMENT_ID " + func() string {
				if operation == "remove" {
					return "HASH --dry-run"
				}
				return "P1 --datatype string --value Example --dry-run"
			}(),
			RunE: func(cmd *cobra.Command, args []string) error {
				if err := out.validate(); err != nil {
					return err
				}
				var snak wikiapi.Snak
				hash := ""
				if operation == "add" {
					if input != "" {
						if len(args) != 2 || cmd.Flags().Changed("datatype") || cmd.Flags().Changed("value") || cmd.Flags().Changed("value-json") || cmd.Flags().Changed("snaktype") {
							return fmt.Errorf("choose native snak input or typed value flags")
						}
						data, err := readEntityJSON(cmd, input)
						if err != nil {
							return err
						}
						if err = json.Unmarshal(data, &snak); err != nil {
							return err
						}
						if err = wikiapi.ValidateSnak(snak, rawJSON); err != nil {
							return err
						}
					} else {
						if len(args) != 3 || rawJSON {
							return fmt.Errorf("provide PROPERTY with typed flags, or --input [--raw-json]")
						}
						var err error
						snak, err = values.read(cmd, args[2])
						if err != nil {
							return err
						}
					}
				} else {
					hash = args[2]
				}
				client, err := wikiAPIClient(false)
				if err != nil {
					return err
				}
				plan, err := client.PlanQualifier(cmd.Context(), kind, args[0], args[1], operation, hash, snak, options.BaseRevision, rawJSON)
				if err != nil {
					return err
				}
				return finishWikibasePlan(cmd, out, options, client, plan)
			},
		}
		if operation == "add" {
			leaf.Flags().StringVar(&input, "input", "", "Native snak JSON file, or - for stdin")
			_ = leaf.MarkFlagFilename("input")
			leaf.Flags().BoolVar(&rawJSON, "raw-json", false, "Allow native datatypes outside the typed subset")
			values.flags(leaf)
		}
		options.flags(leaf)
		out.flags(leaf)
		cmd.AddCommand(leaf)
	}
	return cmd
}

func newStatementReferenceCmd(kind string) *cobra.Command {
	cmd := &cobra.Command{Use: "reference", Short: "Read, add or remove statement references"}
	cmd.AddCommand(statementPartListCmd(kind, "references"))
	for _, operation := range []string{"add", "remove"} {
		out := &apiOutput{}
		options := &entityEditFlags{}
		var input string
		var rawJSON bool
		use := operation + " ID STATEMENT_ID HASH"
		args := cobra.ExactArgs(3)
		if operation == "add" {
			use = operation + " ID STATEMENT_ID"
			args = cobra.ExactArgs(2)
		}
		leaf := &cobra.Command{
			Use: use, Short: "Apply one revision-guarded reference " + operation, Args: args, ValidArgsFunction: cobra.NoFileCompletions,
			Example: "  " + operation + " " + entityExampleID(kind) + " STATEMENT_ID " + func() string {
				if operation == "remove" {
					return "HASH --dry-run"
				}
				return "--input reference.json --dry-run"
			}(),
			RunE: func(cmd *cobra.Command, args []string) error {
				if err := out.validate(); err != nil {
					return err
				}
				var reference wikiapi.Reference
				hash := ""
				if operation == "add" {
					data, err := readEntityJSON(cmd, input)
					if err != nil {
						return err
					}
					reference, err = wikiapi.ParseReference(data)
					if err != nil {
						return err
					}
					if err = wikiapi.ValidateReference(reference, rawJSON); err != nil {
						return err
					}
				} else {
					hash = args[2]
				}
				client, err := wikiAPIClient(false)
				if err != nil {
					return err
				}
				plan, err := client.PlanReference(cmd.Context(), kind, args[0], args[1], operation, hash, reference, options.BaseRevision, rawJSON)
				if err != nil {
					return err
				}
				return finishWikibasePlan(cmd, out, options, client, plan)
			},
		}
		if operation == "add" {
			leaf.Flags().StringVar(&input, "input", "", "Native reference JSON (snaks map), or - for stdin")
			_ = leaf.MarkFlagFilename("input")
			leaf.Flags().BoolVar(&rawJSON, "raw-json", false, "Allow native datatypes outside the typed subset")
		}
		options.flags(leaf)
		out.flags(leaf)
		cmd.AddCommand(leaf)
	}
	return cmd
}
