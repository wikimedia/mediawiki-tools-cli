package output

import (
	"bytes"
	"encoding/json"
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

func TestCheckedJSONPreservesLargeIdentifiers(t *testing.T) {
	value := map[string]any{"identifier": json.Number("9007199254740993")}
	for _, format := range []string{".identifier", ".identifier + 1"} {
		var out bytes.Buffer
		if err := PrintJSONChecked(value, format, false, &out); err != nil {
			t.Fatal(err)
		}
		want := "9007199254740993\n"
		if strings.Contains(format, "+") {
			want = "9007199254740994\n"
		}
		if out.String() != want {
			t.Fatalf("identifier lost precision: %s", out.String())
		}
	}
}
