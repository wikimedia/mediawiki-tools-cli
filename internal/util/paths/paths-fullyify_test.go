package paths

import (
	"os"
	"os/user"
	"testing"
)

func TestFullifyUserProvidedPath(t *testing.T) {
	usr, _ := user.Current()
	usrDir := usr.HomeDir
	pwd, _ := os.Getwd()

	tests := []struct {
		name  string
		given string
		want  string
	}{
		{
			name:  "Full path",
			given: "/foo",
			want:  "/foo",
		},
		{
			name:  "User dir 1",
			given: "~",
			want:  usrDir,
		},
		{
			name:  "User dir 2",
			given: "~/",
			want:  usrDir,
		},
		{
			name:  "User sub dir",
			given: "~/foo",
			want:  usrDir + "/foo",
		},
		{
			name:  "pwd dir 1",
			given: ".",
			want:  pwd,
		},
		{
			name:  "pwd dir 2",
			given: "./",
			want:  pwd,
		},
		{
			name:  "pwd sub dir",
			given: "./foo",
			want:  pwd + "/foo",
		},
		{
			name:  "string is joined with pwd",
			given: "foo",
			want:  pwd + "/foo",
		},
		{
			name:  "string with paths is joined with pwd",
			given: "foo/bar",
			want:  pwd + "/foo/bar",
		},
		{
			name:  "string with paths is joined with pwd",
			given: `X:\Users\adam`,
			want:  `X:\Users\adam`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FullifyUserProvidedPath(tt.given); got != tt.want {
				t.Errorf("FullifyUserProvidedPath() = %v, want %v", got, tt.want)
			}
		})
	}
}
