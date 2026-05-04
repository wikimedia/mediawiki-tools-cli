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
	"strings"

	"github.com/fatih/color"
	"github.com/rodaine/table"
	stringsutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/strings"
)

type Table struct {
	Headings        []interface{}
	Rows            [][]interface{}
	// TrimSpace trims leading and trailing whitespace from every cell before rendering.
	TrimSpace bool
	// ColumnMaxWidths holds the maximum character width for each column (0 = unlimited).
	ColumnMaxWidths []int
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
	t.AddHeadings(stringsutil.SplitToInterfaceSplit(headings)...)
}

func (t *Table) AddRow(rowValues ...interface{}) {
	var thisRow []interface{}
	t.Rows = append(t.Rows, append(thisRow, rowValues...))
}

func (t *Table) AddRowS(rowValues ...string) {
	t.AddRow(stringsutil.SplitToInterfaceSplit(rowValues)...)
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
		processed := make([]interface{}, len(row))
		for i, cell := range row {
			s := fmt.Sprintf("%v", cell)
			if t.TrimSpace {
				s = strings.TrimSpace(s)
			}
			if len(t.ColumnMaxWidths) > i {
				s = truncateCell(s, t.ColumnMaxWidths[i])
			}
			processed[i] = s
		}
		tbl.AddRow(processed...)
	}

	tbl.WithWriter(writer)
	tbl.Print()
}
