package cobrautil

import (
	"testing"
)

func TestIndentExamples(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Single line",
			input:    "example",
			expected: "  example",
		},
		{
			name:     "Multiple lines",
			input:    "example1\nexample2",
			expected: "  example1\n  example2",
		},
		{
			name:     "Lines with leading and trailing spaces",
			input:    "  example1  \n  example2  ",
			expected: "  example1\n  example2",
		},
		{
			name:     "Empty lines",
			input:    "example1\n\nexample2",
			expected: "  example1\n  example2",
		},
		{
			name:     "All empty lines",
			input:    "\n\n",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeExample(tt.input)
			if result != tt.expected {
				t.Errorf("IndentExamples(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
