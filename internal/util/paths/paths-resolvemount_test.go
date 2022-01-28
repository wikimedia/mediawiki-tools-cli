package paths

import (
	"os"
	"testing"
)

func TestResolveMountForCwd(t *testing.T) {
	cwd, _ := os.Getwd()

	app := "/app"

	type args struct {
		mountFrom string
		mountTo   string
	}
	tests := []struct {
		name string
		args args
		want *string
	}{
		{
			name: "no known path",
			args: args{
				mountFrom: "/a",
				mountTo:   "/b",
			},
			want: nil,
		},
		{
			name: "known path",
			args: args{
				mountFrom: cwd,
				mountTo:   "/app",
			},
			want: &app,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolveMountForCwd(tt.args.mountFrom, tt.args.mountTo); !pointerStringsMatch(got, tt.want) {
				t.Errorf("ResolveMountForCwd() = %v, want %v", pointerStringToString(got), pointerStringToString(tt.want))
			}
		})
	}
}

func Test_resolveMountForDirectory(t *testing.T) {
	type args struct {
		mountFrom string
		mountTo   string
		directory string
	}

	b := "/b"
	bFoo := "/b/foo"

	tests := []struct {
		name string
		args args
		want *string
	}{
		{
			name: "no known path",
			args: args{
				mountFrom: "/a",
				mountTo:   "/b",
				directory: "/c",
			},
			want: nil,
		},
		{
			name: "known path",
			args: args{
				mountFrom: "/a",
				mountTo:   "/b",
				directory: "/a",
			},
			want: &b,
		},
		{
			name: "known sub path",
			args: args{
				mountFrom: "/a",
				mountTo:   "/b",
				directory: "/a/foo",
			},
			want: &bFoo,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveMountForDirectory(tt.args.mountFrom, tt.args.mountTo, tt.args.directory); !pointerStringsMatch(got, tt.want) {
				t.Errorf("resolveMountForDirectory() = %v, want %v", pointerStringToString(got), pointerStringToString(tt.want))
			}
		})
	}
}

func pointerStringsMatch(a *string, b *string) bool {
	return pointerStringToString(a) == pointerStringToString(b)
}

func pointerStringToString(a *string) string {
	if a == nil {
		return "*nil*"
	}
	return *a
}
