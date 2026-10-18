package cli

import "testing"

func TestInvocationName(t *testing.T) {
	for _, tc := range []struct{ path, want string }{
		{"mw", "mw"},
		{"./bin/wm", "wm"},
		{"/opt/wiki-tool", "wiki-tool"},
		{`C:\tools\wm.exe`, "wm"},
		{"wm.EXE", "wm"},
		{"", DefaultCommandName},
		{"/", DefaultCommandName},
		{"bad\nname", DefaultCommandName},
	} {
		if got := InvocationName(tc.path); got != tc.want {
			t.Errorf("%q: got %q want %q", tc.path, got, tc.want)
		}
	}
	if APPNAME != "mwcli" {
		t.Fatal("executable rename changed config identity")
	}
}
