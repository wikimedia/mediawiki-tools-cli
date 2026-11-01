package output

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestCheckedJSONFormatsAndFailures(t *testing.T) {
	for _, test := range []struct {
		format string
		raw    bool
		want   string
	}{{".name", false, "\"Earth\"\n"}, {".name", true, "Earth\n"}, {".missing", false, "null\n"}, {"empty", true, ""}} {
		var out bytes.Buffer
		if err := PrintJSONChecked(map[string]interface{}{"name": "Earth"}, test.format, test.raw, &out); err != nil || out.String() != test.want {
			t.Fatalf("%q %v", out.String(), err)
		}
	}
	for _, format := range []string{"[", "unknown_function"} {
		if ValidateJSONFormat(format) == nil {
			t.Fatal(format)
		}
	}
	var out bytes.Buffer
	if err := PrintJSONChecked(map[string]interface{}{"name": "Earth"}, `.name, error("fail")`, true, &out); err == nil || out.Len() != 0 {
		t.Fatalf("%q %v", out.String(), err)
	}
	expected := errors.New("broken pipe")
	if err := PrintJSONChecked("Earth", ".", false, checkedFailWriter{expected}); !errors.Is(err, expected) {
		t.Fatal(err)
	}
	if err := PrintJSONChecked("Earth", ".", false, &out); err != nil || !strings.Contains(out.String(), "Earth") {
		t.Fatal(err)
	}
}

type checkedFailWriter struct{ err error }

func (w checkedFailWriter) Write([]byte) (int, error) { return 0, w.err }
