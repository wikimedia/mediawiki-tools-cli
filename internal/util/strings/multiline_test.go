package strings

import (
	"reflect"
	"testing"
)

func TestFilterMultiline(t *testing.T) {
	type args struct {
		s               string
		requiredMatches []string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "no match",
			args: args{
				s:               "foo",
				requiredMatches: []string{"bar"},
			},
			want: "",
		},
		{
			name: "match single",
			args: args{
				s:               "foo\nbar",
				requiredMatches: []string{"bar"},
			},
			want: "bar",
		},
		{
			name: "match multiple lines",
			args: args{
				s:               "foo\nbar\nbaz",
				requiredMatches: []string{"ba"},
			},
			want: "bar\nbaz",
		},
		{
			name: "match multiple search",
			args: args{
				s:               "foo\nbar\nbaz",
				requiredMatches: []string{"b", "a"},
			},
			want: "bar\nbaz",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FilterMultiline(tt.args.s, tt.args.requiredMatches); got != tt.want {
				t.Errorf("FilterMultiline() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSplitMultiline(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "Single line",
			args: args{
				s: "foo",
			},
			want: []string{"foo"},
		},
		{
			name: "Two lines",
			args: args{
				s: "foo\nbar",
			},
			want: []string{"foo", "bar"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SplitMultiline(tt.args.s); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SplitMultiline() = %v, want %v", got, tt.want)
			}
		})
	}
}
