package output

import (
	"bytes"
	"testing"
)

func TestGoTmpl_Print(t *testing.T) {
	type fields struct {
		Objects map[interface{}]interface{}
		Format  string
	}
	tests := []struct {
		name       string
		fields     fields
		wantWriter string
	}{
		{
			name: "emtpy everything is empty",
			fields: fields{
				Objects: provideMap("empty"),
				Format:  "",
			},
			wantWriter: "",
		},
		{
			name: "format that is just a string is returned per row",
			fields: fields{
				Objects: provideMap("test1.json"),
				Format:  "foo",
			},
			wantWriter: "foo\nfoo\n",
		},
		{
			name: "valid format is parsed and used",
			fields: fields{
				Objects: provideMap("test1.json"),
				Format:  "{{.TopLevelString}}",
			},
			wantWriter: "aString\nbString\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &GoTmpl{
				Objects: tt.fields.Objects,
				Format:  tt.fields.Format,
			}
			writer := &bytes.Buffer{}
			m.Print(writer)
			if gotWriter := writer.String(); gotWriter != tt.wantWriter {
				t.Errorf("GoTmpl.Print()...\n%v\n...want...\n%v\n...", gotWriter, tt.wantWriter)

			}
		})
	}
}
