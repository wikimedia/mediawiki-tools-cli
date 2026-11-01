package ziki

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestInteractiveInputExitAndErrors(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		err        error
	}{{"quit", "quit", nil}, {"EOF", "", io.EOF}, {"failure", "", io.ErrUnexpectedEOF}} {
		t.Run(tc.name, func(t *testing.T) {
			g, _ := testGame("")
			g.readInteractive = func(string) (string, error) { return tc.text, tc.err }
			_, ok := g.readLine("prompt")
			if ok || !g.stopped {
				t.Fatal("interactive input did not stop")
			}
			if tc.name == "failure" {
				if !errors.Is(g.inputErr, tc.err) {
					t.Fatalf("error=%v", g.inputErr)
				}
			} else if g.inputErr != nil {
				t.Fatal(g.inputErr)
			}
		})
	}
}

func TestRedirectedInputDoesNotUseTerminalEditor(t *testing.T) {
	if interactiveReader(strings.NewReader("quit\n"), io.Discard) != nil {
		t.Fatal("terminal editing enabled for redirected input")
	}
}
