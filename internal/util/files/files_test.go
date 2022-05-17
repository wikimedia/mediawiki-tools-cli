package files

import (
	"io/ioutil"
	"math/rand"
	"os"
	"reflect"
	"strings"
	"testing"
)

func writeContentToTmpFile(content string) string {
	tmpFile, err := ioutil.TempFile(os.TempDir(), "mwcli-test-files-")
	if err != nil {
		panic(err)
	}
	tmpFile.WriteString(content)
	tmpFile.Close()
	return tmpFile.Name()
}

func randomString() string {
	// A bit of randomness so that we don't need to open a file for our non existent test
	chars := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	var b strings.Builder
	for i := 0; i < 10; i++ {
		b.WriteRune(chars[rand.Intn(len(chars))])
	}
	return b.String()
}

func TestAddLinesUnique(t *testing.T) {
	type args struct {
		lines    []string
		filename string
	}

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Non Existent",
			args: args{
				lines:    []string{"foo"},
				filename: "/tmp/mwcli-test-files-empty-" + randomString(),
			},
			want: "foo\n",
		},
		{
			name: "Empty",
			args: args{
				lines:    []string{},
				filename: writeContentToTmpFile(""),
			},
			want: "",
		},
		{
			name: "Keep one",
			args: args{
				lines:    []string{"foo"},
				filename: writeContentToTmpFile("foo\n"),
			},
			want: "foo\n",
		},
		{
			name: "Add one on empty, making one",
			args: args{
				lines:    []string{"foo"},
				filename: writeContentToTmpFile(""),
			},
			want: "foo\n",
		},
		{
			name: "Add one, making two",
			args: args{
				lines:    []string{"foo"},
				filename: writeContentToTmpFile("bar\n"),
			},
			want: "bar\nfoo\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AddLinesUnique(tt.args.lines, tt.args.filename)
			got, _ := ioutil.ReadFile(tt.args.filename)
			if string(got) != tt.want {
				t.Errorf(tt.args.filename+" AddLinesUnique() = %v, want %v", string(got), tt.want)
			}
		})
	}
}

func TestLines(t *testing.T) {
	type args struct {
		fileName string
	}

	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "Non Existent",
			args: args{
				fileName: "/tmp/mwcli-test-files-empty-" + randomString(),
			},
			want: []string{},
		},
		{
			name: "Empty",
			args: args{
				fileName: writeContentToTmpFile(""),
			},
			want: []string{},
		},
		{
			name: "One",
			args: args{
				fileName: writeContentToTmpFile("foo"),
			},
			want: []string{"foo"},
		},
		{
			name: "Two",
			args: args{
				fileName: writeContentToTmpFile("foo\nbar\n"),
			},
			want: []string{"foo", "bar"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Lines(tt.args.fileName); !reflect.DeepEqual(got, tt.want) {
				// Special case for empty splits which DeepEqual doesn't like
				if len(got) != 0 && len(tt.want) != 0 {
					t.Errorf("Lines() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}
