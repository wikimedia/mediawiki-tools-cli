package jsonfile

import (
	"testing"
)

func TestJSONFile_Clear(t *testing.T) {
	t.Run("clear clears json content", func(t *testing.T) {
		jEmpty := LoadFromDisk("./test_empty.json")
		jNonEmpty := LoadFromDisk("./test_nonEmpty.json")
		jNonEmpty.Clear()
		if jEmpty.String() != jNonEmpty.String() {
			t.Error("Expected json to be empty")
		}
	})
}

func TestJSONFile_String(t *testing.T) {
	tests := []struct {
		name     string
		loadFile string
		want     string
	}{
		{
			name:     "empty looks empty",
			loadFile: "./test_empty.json",
			want:     "{}",
		},
		{
			name:     "content is printed",
			loadFile: "./test_nonEmpty.json",
			want: `{
  "foo": "bar"
}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := LoadFromDisk(tt.loadFile)
			if got := j.String(); got != tt.want {
				t.Errorf("JSONFile.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
