package gerrit

import (
	_ "embed"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_jsonStringToInterface_roundtrip(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "simple",
			input: `{ "foo": "bar" }`,
		},
		{
			name:  "complex",
			input: `{ "foo": "bar", "baz": { "quux": "quuz" } }`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := jsonStringToInterface(tt.input)
			s, e := json.Marshal(i)
			if e != nil {
				t.Fatal(e)
			}
			// Assert json strings are the same
			assert.JSONEq(t, tt.input, string(s))
		})
	}
}
