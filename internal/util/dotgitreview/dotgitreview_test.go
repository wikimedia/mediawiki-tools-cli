package dotgitreview

import (
	"os"
	"reflect"
	"testing"
)

func TestForDirectory(t *testing.T) {
	cwd, _ := os.Getwd()
	type args struct {
		dir string
	}
	tests := []struct {
		name    string
		args    args
		want    *GitReview
		wantErr bool
	}{
		{
			name: "invalid",
			args: args{
				dir: cwd + "/testdata/invalid",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "valid",
			args: args{
				dir: cwd + "/testdata/valid",
			},
			want: &GitReview{
				Host:       "gerrit.wikimedia.org",
				Port:       "29418",
				Project:    "someProject/subProject",
				RawProject: "someProject/subProject.git",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ForDirectory(tt.args.dir)
			if (err != nil) != tt.wantErr {
				t.Errorf("ForDirectory() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ForDirectory() = %v, want %v", got, tt.want)
			}
		})
	}
}
