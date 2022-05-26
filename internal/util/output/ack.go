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
*/
import (
	"fmt"

	"github.com/fatih/color"
)

type Ack struct {
	Sections map[string][]interface{}
}

func (a *Ack) initMap() {
	if a.Sections == nil {
		a.Sections = make(map[string][]interface{})
	}
}

func (a *Ack) InitSection(name string) {
	a.initMap()
	var emptyItems []interface{}
	a.Sections[name] = emptyItems
}

func (a *Ack) AddSection(name string, items []interface{}) {
	a.initMap()
	a.Sections[name] = items
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

func (a *Ack) Print() {
	headerFmt := color.New(color.FgGreen, color.Underline).SprintfFunc()

	firstOneDone := false
	for section, items := range a.Sections {
		if firstOneDone {
			fmt.Println("")
		}
		firstOneDone = true

		if shouldColor() {
			fmt.Print(headerFmt("%s:\n", section))
		} else {
			fmt.Printf("%s:\n", section)
		}

		for _, item := range items {
			fmt.Printf("%s\n", item)
		}
	}
}
