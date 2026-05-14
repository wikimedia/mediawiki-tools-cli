package output

import (
	"bytes"
	"testing"
)

func TestPretty_Print(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(*Pretty)
		wantOutput string
	}{
		{
			name:       "empty pretty",
			setup:      func(p *Pretty) {},
			wantOutput: "",
		},
		{
			name: "one section no items",
			setup: func(p *Pretty) {
				p.InitSection("header")
			},
			wantOutput: "header\n",
		},
		{
			name: "one section one item",
			setup: func(p *Pretty) {
				p.AddItem("header", "value")
			},
			wantOutput: "header\n  value\n",
		},
		{
			name: "one section two items",
			setup: func(p *Pretty) {
				p.AddItem("section", "first")
				p.AddItem("section", "second")
			},
			wantOutput: "section\n  first\n  second\n",
		},
		{
			name: "two sections",
			setup: func(p *Pretty) {
				p.AddItem("sectionA", "item1")
				p.AddItem("sectionB", "item2")
			},
			wantOutput: "sectionA\n  item1\n\nsectionB\n  item2\n",
		},
		{
			name: "line number prefix preserved",
			setup: func(p *Pretty) {
				p.AddItem("file.go", "42: some code")
			},
			wantOutput: "file.go\n  42: some code\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Pretty{}
			tt.setup(p)
			var buf bytes.Buffer
			p.Print(&buf)
			if got := buf.String(); got != tt.wantOutput {
				t.Errorf("Pretty.Print()\ngot:\n%q\nwant:\n%q", got, tt.wantOutput)
			}
		})
	}
}

func TestIsDigitsOnly(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"123", true},
		{"0", true},
		{"", false},
		{"12a", false},
		{"abc", false},
		{" 1", false},
	}
	for _, tt := range tests {
		if got := isDigitsOnly(tt.input); got != tt.want {
			t.Errorf("isDigitsOnly(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
