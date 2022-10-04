// Package lookpath provides wrapping functionality around os.exec.LookPath
package lookpath

import (
	"os/exec"
	"reflect"
	"testing"
)

func TestHasExecutable(t *testing.T) {
	whereIsGo, _ := exec.LookPath("go")
	type args struct {
		executable string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "No such binary",
			args: args{
				executable: "/sadiojjdsodsa",
			},
			want: false,
		},
		{
			name: "No such binary",
			args: args{
				executable: whereIsGo,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasExecutable(tt.args.executable); got != tt.want {
				t.Errorf("HasExecutable() = %v, want %v for executable = %v", got, tt.want, tt.args.executable)
			}
		})
	}
}

func TestNeedExecutables(t *testing.T) {
	type args struct {
		executables []string
	}
	tests := []struct {
		name        string
		args        args
		wantMissing []string
		wantErr     bool
	}{
		{
			name: "Want go, got go",
			args: args{
				executables: []string{"go"},
			},
			wantErr: false,
		},
		{
			name: "Want tttttt, not existing",
			args: args{
				executables: []string{"tttttt"},
			},
			wantMissing: []string{"tttttt"},
			wantErr:     true,
		},
		{
			name: "Want tttttt & go, not existing",
			args: args{
				executables: []string{"tttttt", "go"},
			},
			wantMissing: []string{"tttttt"},
			wantErr:     true,
		},
		{
			name: "Want tttttt & uuuuuu, not existing",
			args: args{
				executables: []string{"tttttt", "uuuuuu"},
			},
			wantMissing: []string{"tttttt", "uuuuuu"},
			wantErr:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMissing, err := NeedExecutables(tt.args.executables)
			if (err != nil) != tt.wantErr {
				t.Errorf("NeedExecutables() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotMissing, tt.wantMissing) {
				t.Errorf("NeedExecutables() = %v, want %v", gotMissing, tt.wantMissing)
			}
		})
	}
}
