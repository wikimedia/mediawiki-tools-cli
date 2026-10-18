package output

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestPrintJSONEvent(t *testing.T) {
	data := []byte("{\n\"id\": 9007199254740993, \"title\": \"Example\"\n}")
	for _, tt := range []struct {
		name     string
		terminal bool
		noColor  string
		cliColor string
		term     string
		colour   bool
	}{
		{name: "pipe"},
		{name: "terminal", terminal: true, colour: true},
		{name: "NO_COLOR", terminal: true, noColor: "1"},
		{name: "CLICOLOR", terminal: true, cliColor: "0"},
		{name: "dumb terminal", terminal: true, term: "dumb"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", tt.noColor)
			t.Setenv("CLICOLOR", tt.cliColor)
			t.Setenv("TERM", tt.term)
			var out bytes.Buffer
			if err := printJSONEvent(&out, data, tt.terminal); err != nil {
				t.Fatal(err)
			}
			got := out.String()
			if strings.Contains(got, "\x1b[") != tt.colour {
				t.Fatalf("unexpected colour: %q", got)
			}
			if !strings.Contains(got, "9007199254740993") {
				t.Fatalf("lost integer precision: %q", got)
			}
			if tt.colour {
				if !strings.Contains(got, "\n  ") {
					t.Fatalf("missing indentation: %q", got)
				}
			} else {
				want := "{\"id\":9007199254740993,\"title\":\"Example\"}\n"
				if tt.terminal {
					want = "{\n  \"id\": 9007199254740993,\n  \"title\": \"Example\"\n}\n"
				}
				if got != want {
					t.Fatalf("got %q, want %q", got, want)
				}
			}
		})
	}
}

func TestPrintJSONEventRedirected(t *testing.T) {
	var out bytes.Buffer
	for _, event := range []string{`{"id": 1}`, "{\n\"id\": 2\n}"} {
		if err := PrintJSONEvent(&out, []byte(event)); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := out.String(), "{\"id\":1}\n{\"id\":2}\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

type failingEventWriter struct{}

func (failingEventWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestPrintJSONEventErrors(t *testing.T) {
	for _, terminal := range []bool{false, true} {
		if err := printJSONEvent(failingEventWriter{}, []byte(`{"id":1}`), terminal); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("lost writer error: %v", err)
		}
		var out bytes.Buffer
		if err := printJSONEvent(&out, []byte(`{"id":`), terminal); err == nil || out.Len() != 0 {
			t.Fatalf("invalid JSON should fail without output: err=%v, output=%q", err, out.String())
		}
	}
}
