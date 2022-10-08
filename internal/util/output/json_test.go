package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestJSON_Print(t *testing.T) {
	type fields struct {
		Objects      map[interface{}]interface{}
		Format       string
		TopLevelKeys bool
	}
	tests := []struct {
		name                  string
		fields                fields
		wantWriter            string
		wantLogWriterContains string
	}{
		{
			name: "Empty map is empty output",
			fields: fields{
				Objects:      provideMap("empty"),
				Format:       "",
				TopLevelKeys: false,
			},
			wantWriter: "",
		},
		{
			name: "Interesting map",
			fields: fields{
				Objects:      provideMap("test1.json"),
				Format:       "",
				TopLevelKeys: false,
			},
			wantWriter: `{"TopLevelMatchingInt":99,"TopLevelMatchingString":"match","TopLevelString":"aString","TopLevelStruct":{"SecondLevelString":"aString","SecondLevelStructList":[{"ThirdLevelInt":1,"ThirdLevelList":["foo","bar"],"ThirdLevelString":"aString"}]}}` + "\n" +
				`{"TopLevelMatchingInt":99,"TopLevelMatchingString":"match","TopLevelString":"bString","TopLevelStruct":{"SecondLevelString":"bString","SecondLevelStructList":[{"ThirdLevelInt":69,"ThirdLevelList":["cat","goat"],"ThirdLevelString":"bString"}]}}` + "\n",
		},
		{
			name: "Interesting map, with format applied",
			fields: fields{
				Objects:      provideMap("test1.json"),
				Format:       ".[\"TopLevelString\"]",
				TopLevelKeys: false,
			},
			wantWriter: "\"aString\"\n\"bString\"\n",
		},
		{
			name: "Simple table has output",
			fields: fields{
				Objects:      provideMap("simpleTable"),
				Format:       "",
				TopLevelKeys: true,
			},
			wantWriter: `{"k1":"v1","k2":"v2"}` + "\n",
		},
		{
			name: "Simple table has output, and format can be applied ..",
			fields: fields{
				Objects:      provideMap("simpleTable"),
				Format:       ".[]",
				TopLevelKeys: true,
			},
			wantWriter: "\"v1\"\n\"v2\"\n",
		},
		{
			name: "Simple table has output, bad format",
			fields: fields{
				Objects:      provideMap("simpleTable"),
				Format:       "hdsa0jdsa",
				TopLevelKeys: true,
			},
			wantWriter:            "{}\n",
			wantLogWriterContains: "function not defined: hdsa0jdsa",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := &JSON{
				Objects:      tt.fields.Objects,
				Format:       tt.fields.Format,
				TopLevelKeys: tt.fields.TopLevelKeys,
			}
			writer := &bytes.Buffer{}
			logWriter := &bytes.Buffer{}
			logrus.SetOutput(logWriter)
			j.Print(writer)
			checkStringContainnLinesInAnyOrder(t, writer.String(), tt.wantWriter)
			if gotLogWriterWriter := logWriter.String(); tt.wantLogWriterContains != "" && !strings.Contains(gotLogWriterWriter, tt.wantLogWriterContains) {
				t.Errorf("JSON.Print() log output...\n%v\n...should contain...\n%v\n...", gotLogWriterWriter, tt.wantLogWriterContains)
			}
		})
	}
}
