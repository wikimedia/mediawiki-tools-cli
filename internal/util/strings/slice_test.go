package strings

import (
	"reflect"
	"testing"
)

func TestReplaceInAll(t *testing.T) {
	type args struct {
		list    []string
		find    string
		replace string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "replace in all simple",
			args: args{
				list:    []string{"a", "b", "c"},
				find:    "a",
				replace: "x",
			},
			want: []string{"x", "b", "c"},
		},
		{
			name: "replace in all complex",
			args: args{
				list:    []string{"abc", "123", "aaa"},
				find:    "a",
				replace: "x",
			},
			want: []string{"xbc", "123", "xxx"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ReplaceInAll(tt.args.list, tt.args.find, tt.args.replace); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReplaceInAll() = %v, want %v", got, tt.want)
			}
		})
	}
}
