package output

import (
	"bytes"
	"testing"
)

func TestPrintTemplate_MapInterfaceKeys(t *testing.T) {
	objects := map[interface{}]interface{}{
		"repo": map[string]interface{}{
			"Matches": []interface{}{1, 2, 3},
		},
	}

	var buf bytes.Buffer
	printTemplate(objects, `{{range $k, $v := .}}{{$k}}={{len $v.Matches}}{{"\n"}}{{end}}`, &buf)

	if got, want := buf.String(), "repo=3\n"; got != want {
		t.Fatalf("printTemplate() got %q, want %q", got, want)
	}
}

func TestPrintTemplate_RecursiveMapInterfaceKeys(t *testing.T) {
	objects := map[interface{}]interface{}{
		"outer": map[interface{}]interface{}{
			"inner": "ok",
		},
	}

	var buf bytes.Buffer
	printTemplate(objects, `{{index (index . "outer") "inner"}}`, &buf)

	if got, want := buf.String(), "ok"; got != want {
		t.Fatalf("printTemplate() got %q, want %q", got, want)
	}
}
