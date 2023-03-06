package codegen

import (
	"testing"
)

func TestForFunctionName(t *testing.T) {
	type args struct {
		s       string
		isStart bool
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "empty",
			args: args{s: "", isStart: false},
			want: "",
		},
		{
			name: "hello",
			args: args{s: "hello", isStart: false},
			want: "Hello",
		},
		{
			name: "Hello",
			args: args{s: "Hello", isStart: false},
			want: "Hello",
		},
		{
			name: "Hello",
			args: args{s: "Hello", isStart: true},
			want: "hello",
		},
		{
			name: "get-change",
			args: args{s: "get-change", isStart: true},
			want: "getChange",
		},
		{
			name: "get-change",
			args: args{s: "get-change", isStart: false},
			want: "GetChange",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ForFunctionName(tt.args.s, tt.args.isStart); got != tt.want {
				t.Errorf("ForFunctionName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestForFileName(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "empty",
			args: args{s: ""},
			want: "",
		},
		{
			name: "hello",
			args: args{s: "hello"},
			want: "hello",
		},
		{
			name: "Hello",
			args: args{s: "Hello"},
			want: "hello",
		},
		{
			name: "some-file",
			args: args{s: "some-file"},
			want: "some_file",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ForFileName(tt.args.s); got != tt.want {
				t.Errorf("ForFileName() = %v, want %v", got, tt.want)
			}
		})
	}
}
