package output

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type Output struct {
	Type   string
	Filter []string
	Format string
	// TopLevelKeys is only used for json and gotmpl currently
	TopLevelKeys bool
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
	ProcessObjects func(map[interface{}]interface{}, *Table)
}

type AckBinding func(map[interface{}]interface{}, *Ack)

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

func (o *Output) Print(objects map[interface{}]interface{}) {
	objects = Filter(objects, o.Filter)
	switch o.Type {
	case string(JSONType):
		NewJSON(objects, o.Format, o.TopLevelKeys).Print(os.Stdout)
	case string(GoTmplType):
		NewGoTmpl(objects, o.Format, o.TopLevelKeys).Print(os.Stdout)
	case string(TableType):
		if o.TableBinding == nil {
			logrus.Trace("TableBinding is nil")
			logrus.Error("Output type not supported for current operation.")
		}
		TableFromObjects(
			objects,
			o.TableBinding.Headings,
			o.TableBinding.ProcessObjects,
		).Print(os.Stdout)
	case string(AckType):
		if o.AckBinding == nil {
			logrus.Trace("AckBinding is nil")
			logrus.Error("Output type not supported for current operation.")
		}
		ack := Ack{}
		o.AckBinding(objects, &ack)
		ack.Print(os.Stdout)
	default:
		logrus.Errorf("Unknown output type: %v. Allowed types are: %v", o.Type, AllTypes)
	}
}
