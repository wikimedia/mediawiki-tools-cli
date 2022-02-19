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

type TableBinding struct {
	Headings []string
	ToRow    func(interface{}) []string
}

type AckBinding func(interface{}) (section string, stringVal string)

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

func (o *Output) Print(objects []interface{}) {
	objects = Filter(objects, o.Filter)
	switch o.Type {
	case "json":
		NewJSON(objects, o.Format).Print()
	case "template":
		NewGoTmpl(objects, o.Format).Print()
	case "table":
		if o.TableBinding == nil {
			logrus.Panic("Table binding is nil")
		}
		TableFromObjects(
			objects,
			o.TableBinding.Headings,
			o.TableBinding.ToRow,
		).Print()
	case "ack":
		if o.AckBinding == nil {
			logrus.Panic("Ack binding is nil")
		}
		ack := Ack{}
		for _, obj := range objects {
			section, objString := o.AckBinding(obj)
			ack.AddItem(section, objString)
		}
		ack.Print()
	default:
		logrus.Panic("Unknown output method: " + o.Type)
	}
}
