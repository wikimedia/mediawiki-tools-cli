package jsonfile

import (
	"testing"
)

func TestJSONFile_Clear(t *testing.T) {
	t.Run("clear clears json content", func(t *testing.T) {
		jEmpty, err := LoadFromDisk("./test_empty.json")
		if err != nil {
			t.Fatal(err)
		}
		jNonEmpty, err := LoadFromDisk("./test_nonEmpty.json")
		if err != nil {
			t.Fatal(err)
		}
		jNonEmpty.Clear()
		empty, err := jEmpty.String()
		if err != nil {
			t.Fatal(err)
		}
		cleared, err := jNonEmpty.String()
		if err != nil {
			t.Fatal(err)
		}
		if empty != cleared {
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
			j, err := LoadFromDisk(tt.loadFile)
			if err != nil {
				t.Fatal(err)
			}
			got, err := j.String()
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("JSONFile.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
