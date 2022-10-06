package output

import (
	"bytes"
	"testing"
)

func TestJSON_Print(t *testing.T) {
	type fields struct {
		Objects      map[interface{}]interface{}
		Format       string
		TopLevelKeys bool
	}
	tests := []struct {
		name       string
		fields     fields
		wantWriter string
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
			name: "Simple table has output",
			fields: fields{
				Objects:      provideMap("simpleTable"),
				Format:       "",
				TopLevelKeys: true,
			},
			wantWriter: `{"k1":"v1","k2":"v2"}` + "\n",
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
			j.Print(writer)
			if gotWriter := writer.String(); gotWriter != tt.wantWriter {
				t.Errorf("JSON.Print()...\n%v\n...want...\n%v\n...", gotWriter, tt.wantWriter)
			}
		})
	}
}
