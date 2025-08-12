package output

/*
Creates output list this:

```
Repository                           File
Extension:FileImporter               tests/phpunit/Data/SourceUrlTest.php
Extension:Wikibase                   repo/tests/phpunit/includes/Api/EditEntityTest.php
SecurityCheckPlugin                  tests/integration/redos/test.php
SecurityCheckPlugin                  tests/integration/redos/test.php
```.
*/
import (
	"fmt"
	"io"

	"github.com/fatih/color"
	"github.com/rodaine/table"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/strings"
)

type Table struct {
	Headings []interface{}
	Rows     [][]interface{}
}

func NewTable(headings []interface{}, rows [][]interface{}) *Table {
	return &Table{
		Headings: headings,
		Rows:     rows,
	}
}

func TableFromObjects(objects interface{}, headings []string, processObjects func(interface{}, *Table)) *Table {
	var thisTable Table
	thisTable.AddHeadingsS(headings...)
	processObjects(objects, &thisTable)
	return &thisTable
}

func (t *Table) AddHeadings(headings ...interface{}) {
	t.Headings = append(t.Headings, headings...)
}

func (t *Table) AddHeadingsS(headings ...string) {
	t.AddHeadings(strings.SplitToInterfaceSplit(headings)...)
}

func (t *Table) AddRow(rowValues ...interface{}) {
	var thisRow []interface{}
	t.Rows = append(t.Rows, append(thisRow, rowValues...))
}

func (t *Table) AddRowS(rowValues ...string) {
	t.AddRow(strings.SplitToInterfaceSplit(rowValues)...)
}

func (t *Table) Print(writer io.Writer) {
	var headerFmt table.Formatter
	var columnFmt table.Formatter
	if shouldColor() {
		headerFmt = color.New(color.FgGreen, color.Underline).SprintfFunc()
		columnFmt = color.New(color.FgYellow).SprintfFunc()
	} else {
		headerFmt = fmt.Sprintf
		columnFmt = fmt.Sprintf
	}

	tbl := table.New(t.Headings...)
	tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)

	for _, row := range t.Rows {
		tbl.AddRow(row...)
	}

	tbl.WithWriter(writer)
	tbl.Print()
}
