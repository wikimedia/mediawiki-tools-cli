package output

/*
Creates output list this:

```
Extension:UIFeedback resources/ext.uiFeedback.js
550:            mw.log( 'fooooo' );

Extension:wikihiero tests/parser/tests.txt
82:fooooo -->1</hiero>

Extension:AbuseFilter tests/phpunit/AbuseFilterSaveTest.php
256:            yield 'valid' => [ [ 'fooooobar', 'foooobaz' ], null ];
```.

Sections and Items will be output in the order that they were initialized.
*/
import (
	"fmt"
	"io"

	"github.com/fatih/color"
)

type Ack struct {
	Sections     map[string][]interface{}
	SectionOrder []string
}

func (a *Ack) initMap() {
	if a.Sections == nil {
		a.Sections = make(map[string][]interface{})
	}
}

func (a *Ack) InitSection(name string) {
	var emptyItems []interface{}
	a.AddSection(name, emptyItems)
}

func (a *Ack) AddSection(name string, items []interface{}) {
	a.initMap()
	a.Sections[name] = items
	a.SectionOrder = append(a.SectionOrder, name)
}

func (a *Ack) ensureSection(name string) {
	if _, ok := a.Sections[name]; !ok {
		a.InitSection(name)
	}
}

func (a *Ack) AddItem(section string, item string) {
	a.ensureSection(section)
	a.Sections[section] = append(a.Sections[section], item)
}

func (a *Ack) Print(writer io.Writer) {
	headerFmt := color.New(color.FgGreen, color.Underline).SprintfFunc()

	firstOneDone := false
	for _, section := range a.SectionOrder {
		items := a.Sections[section]
		if firstOneDone {
			fmt.Fprintln(writer, "")
			fmt.Fprintln(writer, "")
		}
		firstOneDone = true

		if shouldColor() {
			fmt.Fprint(writer, headerFmt("%s:\n", section))
		} else {
			fmt.Fprintf(writer, "%s:\n", section)
		}

		for _, item := range items {
			fmt.Fprintf(writer, "%s\n", item)
		}
	}
}
