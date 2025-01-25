package cobrautil

import (
	"testing"

	"github.com/spf13/cobra"
)

func threeFakeCmds() (*cobra.Command, *cobra.Command, *cobra.Command) {
	lv1Cmd := &cobra.Command{
		Use: "level1",
	}
	lv2Cmd := &cobra.Command{
		Use: "level2",
	}
	lv1Cmd.AddCommand(lv2Cmd)
	lv3Cmd := &cobra.Command{
		Use: "level3",
	}
	lv2Cmd.AddCommand(lv3Cmd)
	return lv1Cmd, lv2Cmd, lv3Cmd
}

func TestFullCommandString(t *testing.T) {
	lv1Cmd, lv2Cmd, lv3Cmd := threeFakeCmds()

	type args struct {
		cmd *cobra.Command
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "simple 1 level commands",
			args: args{
				cmd: lv1Cmd,
			},
			want: "level1",
		},
		{
			name: "level 2 command",
			args: args{
				cmd: lv2Cmd,
			},
			want: "level1 level2",
		},
		{
			name: "level 3 command",
			args: args{
				cmd: lv3Cmd,
			},
			want: "level1 level2 level3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FullCommandString(tt.args.cmd); got != tt.want {
				t.Errorf("FullCommandString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFullCommandStringWithoutPrefix(t *testing.T) {
	_, _, lv3Cmd := threeFakeCmds()

	type args struct {
		cmd    *cobra.Command
		prefix string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "no prefix trim",
			args: args{
				cmd:    lv3Cmd,
				prefix: "",
			},
			want: "level1 level2 level3",
		},
		{
			name: "no prefix trim",
			args: args{
				cmd:    lv3Cmd,
				prefix: "level1",
			},
			want: "level2 level3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FullCommandStringWithoutPrefix(tt.args.cmd, tt.args.prefix); got != tt.want {
				t.Errorf("FullCommandStringWithoutPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCommandIsSubCommandOf(t *testing.T) {
	_, _, lv3Cmd := threeFakeCmds()

	type args struct {
		cmd        *cobra.Command
		subCommand string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "not matching",
			args: args{
				cmd:        lv3Cmd,
				subCommand: "nothing",
			},
			want: false,
		},
		{
			name: "matching 1",
			args: args{
				cmd:        lv3Cmd,
				subCommand: "level3",
			},
			want: false,
		},
		{
			name: "matching all",
			args: args{
				cmd:        lv3Cmd,
				subCommand: "level3 level2 level1",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CommandIsSubCommandOf(tt.args.cmd, tt.args.subCommand); got != tt.want {
				t.Errorf("CommandIsSubCommandOf() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCommandIsSubCommandOfOneOrMore(t *testing.T) {
	_, _, lv3Cmd := threeFakeCmds()

	type args struct {
		cmd         *cobra.Command
		subCommands []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "empty list to check",
			args: args{
				cmd:         lv3Cmd,
				subCommands: []string{},
			},
			want: false,
		},
		{
			name: "matching 1 of 1",
			args: args{
				cmd:         lv3Cmd,
				subCommands: []string{"level3"},
			},
			want: false,
		},
		{
			name: "matching 1 of 2",
			args: args{
				cmd:         lv3Cmd,
				subCommands: []string{"foo", "level3"},
			},
			want: false,
		},
		{
			name: "matching 0 of 2",
			args: args{
				cmd:         lv3Cmd,
				subCommands: []string{"foo", "bar"},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CommandIsSubCommandOfOneOrMore(tt.args.cmd, tt.args.subCommands); got != tt.want {
				t.Errorf("CommandIsSubCommandOfOneOrMore() = %v, want %v", got, tt.want)
			}
		})
	}
}
