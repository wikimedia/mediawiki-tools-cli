package printers

/*
Creates output list this:

```
Repository                           File
Extension:FileImporter               tests/phpunit/Data/SourceUrlTest.php
Extension:Wikibase                   repo/tests/phpunit/includes/Api/EditEntityTest.php
SecurityCheckPlugin                  tests/integration/redos/test.php
SecurityCheckPlugin                  tests/integration/redos/test.php
```
*/
import (
	"github.com/fatih/color"
	"github.com/rodaine/table"
)

type Table struct {
	Headings []interface{}
	Rows     [][]interface{}
}

func (t *Table) AddHeadings(headings ...interface{}) {
	t.Headings = append(t.Headings, headings...)
}

func (t *Table) AddRow(rowValues ...interface{}) {
	var thisRow []interface{}
	t.Rows = append(t.Rows, append(thisRow, rowValues...))
}

func (t *Table) Print() {
	headerFmt := color.New(color.FgGreen, color.Underline).SprintfFunc()
	columnFmt := color.New(color.FgYellow).SprintfFunc()

	tbl := table.New(t.Headings...)
	tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)

	for _, row := range t.Rows {
		tbl.AddRow(row...)
	}

	tbl.Print()
}
