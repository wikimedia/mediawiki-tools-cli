package output

import (
	"os"
	"sort"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// Output holds the output configuration for a command.
type Output struct {
	Type          string
	Filter        []string
	Format        string
	TableBinding  *TableBinding
	PrettyBinding PrettyBinding
	// ttyDefault and pipeDefault are used when Type resolves to AutoType.
	ttyDefault  Type
	pipeDefault Type
}

// Type is the output format identifier.
type Type string

// These are the different output types.
const (
	JSONType   Type = "json"
	JQType     Type = "jq"
	TableType  Type = "table"
	PrettyType Type = "pretty"
	// AutoType selects ttyDefault or pipeDefault based on whether stdout is a TTY.
	// It is used as the flag default when WithDefaultTTY / WithDefaultPipe are set.
	AutoType Type = "auto"

	// WebType is a special type for opening a browser.
	// It is not included in AllTypes and must be passed as an additional type.
	WebType Type = "web"
)

// AllTypes lists the output types that every command supports.
var AllTypes = []Type{
	JSONType,
	JQType,
	TableType,
	PrettyType,
}

// TableBinding configures how objects are rendered as a table.
type TableBinding struct {
	Headings []string
	// ProcessObjects receives the full object collection and populates the table.
	// Use this when per-row extraction needs custom logic (e.g. nested maps).
	ProcessObjects func(interface{}, *Table)
	// RowExtractor is a simpler alternative to ProcessObjects.
	// When set (and ProcessObjects is nil) the library iterates over the
	// object collection and calls RowExtractor for each element.
	// Return nil to skip a row.
	RowExtractor func(interface{}) []string
	// ColumnMaxWidths holds the maximum character width for each column (0 = unlimited).
	ColumnMaxWidths []int
	// TrimSpace trims leading/trailing whitespace from every cell before rendering.
	TrimSpace bool
}

func (o *Output) ConfiguredOutputTypes() []string {
	outputTypes := []string{string(JSONType), string(JQType)}
	if o.TableBinding != nil {
		outputTypes = append(outputTypes, string(TableType))
	}
	if o.PrettyBinding != nil {
		outputTypes = append(outputTypes, string(PrettyType))
	}
	return outputTypes
}

func (o *Output) ConfiguredOutputTypesString() string {
	return strings.Join(o.ConfiguredOutputTypes(), ", ")
}

// ─── Functional options ───────────────────────────────────────────────────────

// AddFlagsOption is a functional option for AddFlagsWithOpts.
type AddFlagsOption func(*addFlagsConfig)

type addFlagsConfig struct {
	defaultOutput     Type
	defaultTTY        Type
	defaultPipe       Type
	additionalTypes   []Type
	disableFilterFlag bool
	tableBinding      *TableBinding
	prettyBinding     PrettyBinding
}

// WithDefaultOutput sets the fixed default output type.
func WithDefaultOutput(t Type) AddFlagsOption {
	return func(cfg *addFlagsConfig) {
		cfg.defaultOutput = t
	}
}

// WithDefaultTTY sets the output type used when stdout is a TTY and the user
// has not specified --output.  Implies WithDefaultPipe must also be set; together
// they cause the flag default to become "auto".
func WithDefaultTTY(t Type) AddFlagsOption {
	return func(cfg *addFlagsConfig) {
		cfg.defaultTTY = t
	}
}

// WithDefaultPipe sets the output type used when stdout is not a TTY and the
// user has not specified --output.
func WithDefaultPipe(t Type) AddFlagsOption {
	return func(cfg *addFlagsConfig) {
		cfg.defaultPipe = t
	}
}

// WithAdditionalTypes adds extra output types to the flag help text.
func WithAdditionalTypes(types ...Type) AddFlagsOption {
	return func(cfg *addFlagsConfig) {
		cfg.additionalTypes = types
	}
}

// WithFilterFlagDisabled disables the -f / --filter flag.
func WithFilterFlagDisabled() AddFlagsOption {
	return func(cfg *addFlagsConfig) {
		cfg.disableFilterFlag = true
	}
}

// WithTableBinding enables table output and sets its binding.
func WithTableBinding(tb *TableBinding) AddFlagsOption {
	return func(cfg *addFlagsConfig) {
		cfg.tableBinding = tb
	}
}

// WithPrettyBinding enables pretty output and sets its binding.
func WithPrettyBinding(pb PrettyBinding) AddFlagsOption {
	return func(cfg *addFlagsConfig) {
		cfg.prettyBinding = pb
	}
}

// ─── Flag registration ────────────────────────────────────────────────────────

// AddFlags adds output-related flags to cmd using a positional default and
// optional extra types.
//
// Deprecated: prefer AddFlagsWithOpts which uses the functional-option pattern
// and supports smart TTY/pipe defaults.
func (o *Output) AddFlags(cmd *cobra.Command, defaultOutput Type, additionalTypes ...Type) {
	allTypes := append(AllTypes, additionalTypes...)
	allowedTypes := make([]string, len(allTypes))
	for i, t := range allTypes {
		allowedTypes[i] = string(t)
	}
	sort.Strings(allowedTypes)
	cmd.Flags().StringVarP(&o.Type, "output", "o", string(defaultOutput), "Output format ("+strings.Join(allowedTypes, "|")+")")
	cmd.Flags().StringVarP(&o.Format, "format", "", "", "Format the specified output")
	cmd.Flags().StringSliceVarP(&o.Filter, "filter", "f", []string{}, "Filter output based on conditions provided")
}

// AddFlagsWithOpts adds output-related flags to cmd using the functional-option
// pattern.  When both WithDefaultTTY and WithDefaultPipe are provided the flag
// default becomes "auto", which resolves at print-time based on whether stdout
// is a terminal.
func (o *Output) AddFlagsWithOpts(cmd *cobra.Command, opts ...AddFlagsOption) {
	cfg := &addFlagsConfig{
		defaultOutput: JSONType,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	// Apply bindings from options.
	if cfg.tableBinding != nil {
		o.TableBinding = cfg.tableBinding
	}
	if cfg.prettyBinding != nil {
		o.PrettyBinding = cfg.prettyBinding
	}
	if cfg.defaultTTY != "" {
		o.ttyDefault = cfg.defaultTTY
	}
	if cfg.defaultPipe != "" {
		o.pipeDefault = cfg.defaultPipe
	}

	// Build the allowed-types list from what is actually configured.
	allTypes := []Type{JSONType, JQType}
	if o.TableBinding != nil {
		allTypes = append(allTypes, TableType)
	}
	if o.PrettyBinding != nil {
		allTypes = append(allTypes, PrettyType)
	}
	allTypes = append(allTypes, cfg.additionalTypes...)

	allowedTypes := make([]string, len(allTypes))
	for i, t := range allTypes {
		allowedTypes[i] = string(t)
	}
	sort.Strings(allowedTypes)

	// Choose the flag default.
	flagDefault := string(cfg.defaultOutput)
	if cfg.defaultTTY != "" && cfg.defaultPipe != "" {
		flagDefault = string(AutoType)
	}

	cmd.Flags().StringVarP(&o.Type, "output", "o", flagDefault, "Output format ("+strings.Join(allowedTypes, "|")+")")
	cmd.Flags().StringVarP(&o.Format, "format", "", "", "Format the specified output")
	if !cfg.disableFilterFlag {
		cmd.Flags().StringSliceVarP(&o.Filter, "filter", "f", []string{}, "Filter output based on conditions provided")
	}
}

// ─── Rendering ────────────────────────────────────────────────────────────────

// Print outputs objects using the configured output type.
func (o *Output) Print(cmd *cobra.Command, objects any) {
	// Filtering only applies to maps with interface{} keys.
	var filteredObjects interface{}
	switch objs := objects.(type) {
	case map[interface{}]interface{}:
		filteredObjects = Filter(objs, o.Filter)
	default:
		filteredObjects = objects
	}

	// Resolve "auto" to the appropriate concrete type.
	effectiveType := o.Type
	if effectiveType == string(AutoType) {
		if isTerminal(os.Stdout) && o.ttyDefault != "" {
			effectiveType = string(o.ttyDefault)
		} else if o.pipeDefault != "" {
			effectiveType = string(o.pipeDefault)
		} else {
			effectiveType = string(JSONType)
		}
	}

	writer := cmd.OutOrStdout()
	switch effectiveType {
	case string(JSONType):
		NewJSON(filteredObjects, o.Format).Print(writer)
	case string(JQType):
		printJQ(filteredObjects, o.Format, writer)
	case string(TableType):
		if o.TableBinding == nil {
			logrus.Error("Output type 'table' not supported for this command.")
			return
		}
		tbl := &Table{
			TrimSpace:       o.TableBinding.TrimSpace,
			ColumnMaxWidths: o.TableBinding.ColumnMaxWidths,
		}
		switch {
		case o.TableBinding.ProcessObjects != nil:
			o.TableBinding.ProcessObjects(filteredObjects, tbl)
		case o.TableBinding.RowExtractor != nil:
			iterateObjects(filteredObjects, func(v interface{}) {
				if row := o.TableBinding.RowExtractor(v); row != nil {
					tbl.AddRowS(row...)
				}
			})
		}
		tbl.Headings = []interface{}{}
		tbl.AddHeadingsS(o.TableBinding.Headings...)
		tbl.Print(writer)
	case string(PrettyType):
		if o.PrettyBinding == nil {
			logrus.Error("Output type 'pretty' not supported for this command.")
			return
		}
		pretty := Pretty{}
		o.PrettyBinding(filteredObjects, &pretty)
		pretty.Print(writer)
	default:
		logrus.Errorf("Unknown output type: %v. Allowed types: %v", effectiveType, AllTypes)
	}
}

// iterateObjects calls fn for each element in the collection.
// Handles map[interface{}]interface{}, map[string]interface{}, and single values.
func iterateObjects(objects interface{}, fn func(interface{})) {
	switch m := objects.(type) {
	case map[interface{}]interface{}:
		for _, v := range m {
			fn(v)
		}
	case map[string]interface{}:
		for _, v := range m {
			fn(v)
		}
	default:
		fn(objects)
	}
}
