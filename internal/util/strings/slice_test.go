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

func TestStringInSlice(t *testing.T) {
	type args struct {
		find string
		list []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Does exist",
			args: args{
				find: "exist",
				list: []string{
					"lala",
					"exist",
					"foo",
				},
			},
			want: true,
		},
		{
			name: "Does not exist",
			args: args{
				find: "noexist",
				list: []string{
					"lala",
					"exist",
					"foo",
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StringInSlice(tt.args.find, tt.args.list); got != tt.want {
				t.Errorf("StringInSlice() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStringInRegexSlice(t *testing.T) {
	type args struct {
		s         string
		regexList []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Does exist",
			args: args{
				s: "exist",
				regexList: []string{
					"lala",
					"exist",
					"foo",
				},
			},
			want: true,
		},
		{
			name: "Does not exist",
			args: args{
				s: "somethingelse",
				regexList: []string{
					"lala",
					"exist",
					"foo",
				},
			},
			want: false,
		},
		{
			name: "Does exist regex",
			args: args{
				s: "exist",
				regexList: []string{
					"lala",
					"ex.*",
					"foo",
				},
			},
			want: true,
		},
		{
			name: "Does not exist regex",
			args: args{
				s: "somethingelse",
				regexList: []string{
					"lala",
					"ex.*",
					"foo",
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StringInRegexSlice(tt.args.s, tt.args.regexList); got != tt.want {
				t.Errorf("StringInRegexSlice() = %v, want %v", got, tt.want)
			}
		})
	}
}
