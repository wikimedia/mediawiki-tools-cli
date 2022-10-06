package output

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type Output struct {
	Type             string
	Filter           []string
	Format           string
	JSONTopLevelKeys bool
	TableBinding     *TableBinding
	AckBinding       AckBinding
}

type TableBinding struct {
	Headings       []string
	ProcessObjects func(map[interface{}]interface{}, *Table)
}

type AckBinding func(map[interface{}]interface{}, *Ack)

func (o *Output) OutputTypes() []string {
	outputTypes := []string{"json", "template"}
	if o.TableBinding != nil {
		outputTypes = append(outputTypes, "table")
	}
	if o.AckBinding != nil {
		outputTypes = append(outputTypes, "ack")
	}
	return outputTypes
}

func (o *Output) OutputTypesString() string {
	return strings.Join(o.OutputTypes(), ", ")
}

func (o *Output) AddFlags(cmd *cobra.Command, defaultOutput string) {
	cmd.Flags().StringVarP(&o.Type, "output", "", defaultOutput, "How to output the results "+o.OutputTypesString())
	cmd.Flags().StringVarP(&o.Format, "format", "", "", "Format the specified output")
	cmd.Flags().StringSliceVarP(&o.Filter, "filter", "f", []string{}, "Filter output based on conditions provided")
}

func (o *Output) Print(objects map[interface{}]interface{}) {
	objects = Filter(objects, o.Filter)
	switch o.Type {
	case "json":
		NewJSON(objects, o.Format, o.JSONTopLevelKeys).Print(os.Stdout)
	case "template":
		NewGoTmpl(objects, o.Format).Print(os.Stdout)
	case "table":
		if o.TableBinding == nil {
			logrus.Panic("Table binding is nil")
		}
		TableFromObjects(
			objects,
			o.TableBinding.Headings,
			o.TableBinding.ProcessObjects,
		).Print(os.Stdout)
	case "ack":
		if o.AckBinding == nil {
			logrus.Panic("Ack binding is nil")
		}
		ack := Ack{}
		o.AckBinding(objects, &ack)
		ack.Print(os.Stdout)
	default:
		logrus.Panic("Unknown output method: " + o.Type)
	}
}
