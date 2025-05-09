package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestJSON_Print(t *testing.T) {
	type fields struct {
		Objects map[interface{}]interface{}
		Format  string
	}
	tests := []struct {
		name                  string
		fields                fields
		wantWriter            string
		wantLogWriterContains string
	}{
		{
			name: "Simple table has output",
			fields: fields{
				Objects: provideMap("simpleTable"),
				Format:  "",
			},
			wantWriter: `{"k1":"v1","k2":"v2"}` + "\n",
		},
		{
			name: "Simple table has output, and format can be applied ..",
			fields: fields{
				Objects: provideMap("simpleTable"),
				Format:  ".[]",
			},
			wantWriter: "\"v1\"\n\"v2\"\n",
		},
		{
			name: "Simple table has output, bad format",
			fields: fields{
				Objects: provideMap("simpleTable"),
				Format:  "hdsa0jdsa",
			},
			wantWriter:            "{}\n",
			wantLogWriterContains: "function not defined: hdsa0jdsa",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := &JSON{
				Objects: tt.fields.Objects,
				Format:  tt.fields.Format,
			}
			writer := &bytes.Buffer{}
			logWriter := &bytes.Buffer{}
			logrus.SetOutput(logWriter)
			j.Print(writer)
			checkStringContainLinesInAnyOrder(t, writer.String(), tt.wantWriter)
			if gotLogWriterWriter := logWriter.String(); tt.wantLogWriterContains != "" && !strings.Contains(gotLogWriterWriter, tt.wantLogWriterContains) {
				t.Errorf("JSON.Print() log output...\n%v\n...should contain...\n%v\n...", gotLogWriterWriter, tt.wantLogWriterContains)
			}
		})
	}
}
