package output

import (
	"github.com/fatih/color"
	"github.com/rodaine/table"
)

func Table(objects []interface{}, headings []string, objectToRow func(interface{}) []string) {

	// Default table output below
	headerFmt := color.New(color.FgGreen, color.Underline).SprintfFunc()
	columnFmt := color.New(color.FgYellow).SprintfFunc()

	tbl := table.New(stringSplitToInterfaceSplit(headings)...)
	tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)

	for _, object := range objects {
		tbl.AddRow(stringSplitToInterfaceSplit(objectToRow(object))...)
	}
	tbl.Print()
}

func stringSplitToInterfaceSplit(in []string) []interface{} {
	out := make([]interface{}, len(in))
	for i, v := range in {
		out[i] = v
	}
	return out
}
