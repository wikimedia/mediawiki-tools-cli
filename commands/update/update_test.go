package update

import "testing"

func TestExecutableNameFromPath(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "unix path", in: "/usr/local/bin/mw", want: "mw"},
		{name: "windows path", in: `C:\\Users\\adam\\mw.exe`, want: "mw.exe"},
		{name: "plain name", in: "mw", want: "mw"},
		{name: "trailing slash unix", in: "/usr/local/bin/", want: "/usr/local/bin/"},
		{name: "trailing slash windows", in: `C:\\Users\\adam\\`, want: `C:\\Users\\adam\\`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := executableNameFromPath(tc.in)
			if got != tc.want {
				t.Fatalf("executableNameFromPath(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
