package cobrautil

import (
	"reflect"
	"testing"
)

func TestCommandAndEnvFromArgs(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		wantArgs []string
		wantEnvs []string
	}{
		{
			name:     "No env vars",
			input:    []string{"echo", "foo"},
			wantArgs: []string{"echo", "foo"},
			wantEnvs: []string{},
		},
		{
			name:     "Single env var",
			input:    []string{"FOO=bar", "echo", "foo"},
			wantArgs: []string{"echo", "foo"},
			wantEnvs: []string{"FOO=bar"},
		},
		{
			name:     "Multiple env vars",
			input:    []string{"FOO=bar", "BAR=baz", "echo", "foo"},
			wantArgs: []string{"echo", "foo"},
			wantEnvs: []string{"FOO=bar", "BAR=baz"},
		},
		{
			name:     "Mixed args and env vars",
			input:    []string{"echo", "FOO=bar", "foo", "BAR=baz"},
			wantArgs: []string{"echo", "foo"},
			wantEnvs: []string{"FOO=bar", "BAR=baz"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotArgs, gotEnvs := CommandAndEnvFromArgs(tt.input)
			if !reflect.DeepEqual(gotArgs, tt.wantArgs) {
				t.Errorf("CommandAndEnvFromArgs() gotArgs = %v, want %v", gotArgs, tt.wantArgs)
			}
			if !reflect.DeepEqual(gotEnvs, tt.wantEnvs) {
				t.Errorf("CommandAndEnvFromArgs() gotEnvs = %v, want %v", gotEnvs, tt.wantEnvs)
			}
		})
	}
}
