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
	"os"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/fatih/color"
	"github.com/mattn/go-runewidth"
	"github.com/rodaine/table"
	stringsutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/strings"
)

type Table struct {
	Headings []interface{}
	Rows     [][]interface{}
	// TrimSpace trims leading and trailing whitespace from every cell before rendering.
	TrimSpace bool
	// ColumnMaxWidths holds the maximum character width for each column (0 = unlimited).
	ColumnMaxWidths []int
}

// StyledCell adds a semantic colour to a table cell while retaining its plain
// string representation for width calculations and redirected output.
type StyledCell struct {
	Text  string
	Color color.Attribute
}

func (c StyledCell) String() string { return c.Text }

func tableColorEnabled(writer io.Writer) bool {
	file, ok := writer.(*os.File)
	return ok && isTerminal(file) && os.Getenv("NO_COLOR") == "" && os.Getenv("CLICOLOR") != "0" && os.Getenv("TERM") != "dumb"
}

// StyledText uses the same terminal and colour preferences as table cells.
func StyledText(writer io.Writer, text string, attributes ...color.Attribute) string {
	if !tableColorEnabled(writer) {
		return text
	}
	style := color.New(attributes...)
	style.EnableColor()
	return style.Sprint(text)
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
	colors := tableColorEnabled(writer)
	if colors {
		headerColor := color.New(color.FgGreen, color.Underline)
		columnColor := color.New(color.FgYellow)
		headerColor.EnableColor()
		columnColor.EnableColor()
		headerFmt = headerColor.SprintfFunc()
		columnFmt = columnColor.SprintfFunc()
	} else {
		headerFmt = fmt.Sprintf
		columnFmt = fmt.Sprintf
	}

	tbl := table.New(t.Headings...)
	tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)

	links := supportsHyperlinks(writer)
	hasLinks := false
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
			if link, ok := cell.(Hyperlink); ok {
				linked := terminalHyperlink(s, link.URL, links)
				hasLinks = hasLinks || linked != s
				s = linked
			}
			if styled, ok := cell.(StyledCell); ok && colors {
				style := color.New(styled.Color)
				style.EnableColor()
				s = style.Sprint(s)
			}
			processed[i] = s
		}
		tbl.AddRow(processed...)
	}

	if hasLinks || colors {
		tbl.WithWidthFunc(func(s string) int { return runewidth.StringWidth(ansi.Strip(s)) })
	}
	tbl.WithWriter(writer)
	tbl.Print()
}
