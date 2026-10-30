package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fatih/color"
)

func TestStyledTableIsPlainWhenRedirected(t *testing.T) {
	var out bytes.Buffer
	table := NewTable([]interface{}{"Credential", "Comparison"}, [][]interface{}{{"first", StyledCell{Text: "match", Color: color.FgGreen}}, {"second", StyledCell{Text: "different", Color: color.FgRed}}})
	table.Print(&out)
	if strings.Contains(out.String(), "\x1b") || !strings.Contains(out.String(), "match") || !strings.Contains(out.String(), "different") {
		t.Fatal("redirected table lost text or contained terminal escapes")
	}
}
