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

func TestCommandIsSubCommandOfString(t *testing.T) {
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
			if got := CommandIsSubCommandOfString(tt.args.cmd, tt.args.subCommand); got != tt.want {
				t.Errorf("CommandIsSubCommandOf() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCommandIsSubCommandOfOneOrMoreStrings(t *testing.T) {
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
			if got := CommandIsSubCommandOfOneOrMoreStrings(tt.args.cmd, tt.args.subCommands); got != tt.want {
				t.Errorf("CommandIsSubCommandOfOneOrMore() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFullCommandStrings(t *testing.T) {
	lv1Cmd := &cobra.Command{
		Use:     "level1",
		Aliases: []string{"l1", "one"},
	}
	lv2Cmd := &cobra.Command{
		Use:     "level2",
		Aliases: []string{"l2", "two"},
	}
	lv1Cmd.AddCommand(lv2Cmd)
	lv3Cmd := &cobra.Command{
		Use:     "level3",
		Aliases: []string{"l3", "three"},
	}
	lv2Cmd.AddCommand(lv3Cmd)

	tests := []struct {
		name string
		cmd  *cobra.Command
		want []string
	}{
		{
			name: "level 1 command with aliases",
			cmd:  lv1Cmd,
			want: []string{"level1", "l1", "one"},
		},
		{
			name: "level 2 command with aliases",
			cmd:  lv2Cmd,
			want: []string{
				"level1 level2",
				"level1 l2",
				"level1 two",
				"l1 level2",
				"l1 l2",
				"l1 two",
				"one level2",
				"one l2",
				"one two",
			},
		},
		{
			name: "level 3 command with aliases",
			cmd:  lv3Cmd,
			want: []string{
				"level1 level2 level3",
				"level1 level2 l3",
				"level1 level2 three",
				"level1 l2 level3",
				"level1 l2 l3",
				"level1 l2 three",
				"level1 two level3",
				"level1 two l3",
				"level1 two three",
				"l1 level2 level3",
				"l1 level2 l3",
				"l1 level2 three",
				"l1 l2 level3",
				"l1 l2 l3",
				"l1 l2 three",
				"l1 two level3",
				"l1 two l3",
				"l1 two three",
				"one level2 level3",
				"one level2 l3",
				"one level2 three",
				"one l2 level3",
				"one l2 l3",
				"one l2 three",
				"one two level3",
				"one two l3",
				"one two three",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FullCommandStrings(tt.cmd)
			gotMap := make(map[string]bool)
			wantMap := make(map[string]bool)
			for _, g := range got {
				gotMap[g] = true
			}
			for _, w := range tt.want {
				wantMap[w] = true
			}
			if len(gotMap) != len(wantMap) {
				t.Errorf("FullCommandStrings() = %v, want %v", got, tt.want)
				return
			}
			for w := range wantMap {
				if !gotMap[w] {
					t.Errorf("FullCommandStrings() = %v, want %v", got, tt.want)
					break
				}
			}
		})
	}
}
