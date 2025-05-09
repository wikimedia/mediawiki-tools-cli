package output

import (
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type Output struct {
	Type         string
	Filter       []string
	Format       string
	TableBinding *TableBinding
	AckBinding   AckBinding
}

var AllTypes = []Type{
	JSONType,
	GoTmplType,
	TableType,
	AckType,
}

// Type.
type Type string

// These are the different output types.
const (
	JSONType   Type = "json"
	GoTmplType Type = "template"
	TableType  Type = "table"
	AckType    Type = "ack"

	// WebType is a special type that is used to output to a web interface.
	// This is not available by default, and must be provided to additionalTypes, and handled by the caller.
	WebType Type = "web"
)

type TableBinding struct {
	Headings       []string
	ProcessObjects func(interface{}, *Table)
}

type AckBinding func(interface{}, *Ack)

func (o *Output) ConfiguredOutputTypes() []string {
	outputTypes := []string{string(JSONType), string(GoTmplType)}
	if o.TableBinding != nil {
		outputTypes = append(outputTypes, string(TableType))
	}
	if o.AckBinding != nil {
		outputTypes = append(outputTypes, string(AckType))
	}
	return outputTypes
}

func (o *Output) ConfiguredOutputTypesString() string {
	return strings.Join(o.ConfiguredOutputTypes(), ", ")
}

// AddFlagsOption defines a functional option for AddFlags.
type AddFlagsOption func(*addFlagsConfig)

type addFlagsConfig struct {
	defaultOutput     Type
	additionalTypes   []Type
	disableFilterFlag bool
	tableBinding      *TableBinding
	ackBinding        AckBinding
}

// WithDefaultOutput sets the default output type.
func WithDefaultOutput(t Type) AddFlagsOption {
	return func(cfg *addFlagsConfig) {
		cfg.defaultOutput = t
	}
}

// WithAdditionalTypes sets additional output types.
func WithAdditionalTypes(types ...Type) AddFlagsOption {
	return func(cfg *addFlagsConfig) {
		cfg.additionalTypes = types
	}
}

// WithFilterFlagDisabled disables the filter flag.
// As filter can only really be used with lists?
func WithFilterFlagDisabled() AddFlagsOption {
	return func(cfg *addFlagsConfig) {
		cfg.disableFilterFlag = true
	}
}

// WithTableBinding sets the TableBinding and enables table output type.
func WithTableBinding(tb *TableBinding) AddFlagsOption {
	return func(cfg *addFlagsConfig) {
		cfg.tableBinding = tb
	}
}

// WithAckBinding sets the AckBinding and enables ack output type.
func WithAckBinding(ab AckBinding) AddFlagsOption {
	return func(cfg *addFlagsConfig) {
		cfg.ackBinding = ab
	}
}

// AddFlags adds output-related flags to the command.
func (o *Output) AddFlags(cmd *cobra.Command, defaultOutput Type, additionalTypes ...Type) {
	allTypes := append(AllTypes, additionalTypes...)
	allowedTypes := make([]string, len(allTypes))
	for i, t := range allTypes {
		allowedTypes[i] = string(t)
	}
	cmd.Flags().StringVarP(&o.Type, "output", "", string(defaultOutput), "How to output the results "+strings.Join(allowedTypes, ", "))
	cmd.Flags().StringVarP(&o.Format, "format", "", "", "Format the specified output")
	cmd.Flags().StringSliceVarP(&o.Filter, "filter", "f", []string{}, "Filter output based on conditions provided")
}

// AddFlagsWithOpts adds output-related flags to the command using the options pattern.
func (o *Output) AddFlagsWithOpts(cmd *cobra.Command, opts ...AddFlagsOption) {
	// Set up config with defaults
	cfg := &addFlagsConfig{
		defaultOutput:     JSONType,
		additionalTypes:   nil,
		disableFilterFlag: false,
		tableBinding:      nil,
		ackBinding:        nil,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	// Set bindings if provided
	if cfg.tableBinding != nil {
		o.TableBinding = cfg.tableBinding
	}
	if cfg.ackBinding != nil {
		o.AckBinding = cfg.ackBinding
	}
	// Only enable output types that are configured
	allTypes := []Type{JSONType, GoTmplType}
	if o.TableBinding != nil {
		allTypes = append(allTypes, TableType)
	}
	if o.AckBinding != nil {
		allTypes = append(allTypes, AckType)
	}
	allTypes = append(allTypes, cfg.additionalTypes...)
	allowedTypes := make([]string, len(allTypes))
	for i, t := range allowedTypes {
		allowedTypes[i] = string(t)
	}
	cmd.Flags().StringVarP(&o.Type, "output", "", string(cfg.defaultOutput), "How to output the results "+strings.Join(allowedTypes, ", "))
	cmd.Flags().StringVarP(&o.Format, "format", "", "", "Format the specified output")
	if !cfg.disableFilterFlag {
		cmd.Flags().StringSliceVarP(&o.Filter, "filter", "f", []string{}, "Filter output based on conditions provided")
	}
}

// Print outputs the objects using the configured output type and writes to the provided cobra command's output.
func (o *Output) Print(cmd *cobra.Command, objects any) {
	// Filtering only applies to maps
	var filteredObjects interface{}
	switch objs := objects.(type) {
	case map[interface{}]interface{}:
		filteredObjects = Filter(objs, o.Filter)
	default:
		filteredObjects = objects
	}
	writer := cmd.OutOrStderr()
	switch o.Type {
	case string(JSONType):
		NewJSON(filteredObjects, o.Format).Print(writer)
	case string(GoTmplType):
		NewGoTmpl(filteredObjects, o.Format).Print(writer)
	case string(TableType):
		if o.TableBinding == nil {
			logrus.Trace("TableBinding is nil")
			logrus.Error("Output type not supported for current operation.")
			return
		}
		table := &Table{}
		o.TableBinding.ProcessObjects(filteredObjects, table)
		table.Headings = []interface{}{}
		table.AddHeadingsS(o.TableBinding.Headings...)
		table.Print(writer)
	case string(AckType):
		if o.AckBinding == nil {
			logrus.Trace("AckBinding is nil")
			logrus.Error("Output type not supported for current operation.")
			return
		}
		ack := Ack{}
		o.AckBinding(filteredObjects, &ack)
		ack.Print(writer)
	default:
		logrus.Errorf("Unknown output type: %v. Allowed types are: %v", o.Type, AllTypes)
	}
}
