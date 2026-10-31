//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package wiki

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/creack/pty"
	"golang.org/x/term"
)

func TestOAuthScopePickerTerminalKeysAndRestoration(t *testing.T) {
	for _, test := range []struct {
		name, keys string
		cancel     bool
	}{
		{"risk expansion", "\x1b[C\x1b[C\x1b[C\r", false},
		{"cancel", "\x03", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			master, slave, err := pty.Open()
			if err != nil {
				t.Fatal(err)
			}
			defer master.Close()
			defer slave.Close()
			before, err := term.GetState(int(slave.Fd()))
			if err != nil {
				t.Fatal(err)
			}
			ready := make(chan struct{})
			readDone := make(chan struct{})
			go func() {
				defer close(readDone)
				var seen strings.Builder
				buffer := make([]byte, 4096)
				signalled := false
				for {
					n, err := master.Read(buffer)
					if !signalled {
						seen.Write(buffer[:n])
						if strings.Contains(seen.String(), "Wiki access:") {
							close(ready)
							signalled = true
						}
					}
					if err != nil {
						return
					}
				}
			}()
			type result struct {
				scopes []string
				err    error
			}
			done := make(chan result, 1)
			go func() {
				var scopes []string
				err := survey.AskOne(&oauthScopePrompt{selection: newOAuthSelection(nil)}, &scopes, survey.WithStdio(slave, slave, io.Discard))
				done <- result{scopes, err}
			}()
			select {
			case <-ready:
			case got := <-done:
				t.Fatalf("picker stopped before displaying: %v", got.err)
			case <-time.After(3 * time.Second):
				t.Fatal("picker did not display")
			}
			if _, err := master.Write([]byte(test.keys)); err != nil {
				t.Fatal(err)
			}
			var got result
			select {
			case got = <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("picker did not handle terminal input")
			}
			if test.cancel {
				if !errors.Is(got.err, terminal.InterruptErr) {
					t.Fatal("cancel did not abort permission selection", got.err)
				}
			} else {
				if got.err != nil {
					t.Fatal(got.err)
				}
				selected := map[string]bool{}
				for _, scope := range got.scopes {
					selected[scope] = true
				}
				if !selected["basic"] || !selected["delete"] || !selected["import"] || selected["oauthmanageownclient"] || selected["editmywatchlist"] {
					t.Fatal("real arrow keys selected the wrong risk levels")
				}
			}
			after, err := term.GetState(int(slave.Fd()))
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatal("picker did not restore terminal mode", err)
			}
			_ = slave.Close()
			select {
			case <-readDone:
			case <-time.After(time.Second):
				t.Fatal("terminal output reader did not stop")
			}
		})
	}
}
