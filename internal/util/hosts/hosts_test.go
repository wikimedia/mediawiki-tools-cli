package hosts

import (
	"os"
	"reflect"
	"testing"
)

var (
	singleLocalHost           = "127.0.0.1        iam.localhost\n"
	singleOtherHost           = "123.123.111.111        iam.not.localhost\n"
	twoHostsToRemoveOnly      = "127.0.0.1        1.mwcli.test 2.mwcli.test\n"
	twoHostsToRemoveFromLocal = "127.0.0.1        iam.localhost 1.mwcli.test 2.mwcli.test\n"
)

func writeContentToTmpFile(content string) string {
	tmpFile, err := os.CreateTemp(os.TempDir(), hostsTmpPrefix+"test-")
	if err != nil {
		panic(err)
	}
	_, err = tmpFile.WriteString(content)
	if err != nil {
		panic(err)
	}
	tmpFile.Close()
	return tmpFile.Name()
}

func TestAddHosts(t *testing.T) {
	type args struct {
		toAdd []string
		IP    string
	}
	tests := []struct {
		startingContent string
		name            string
		args            args
		want            ChangeResult
	}{
		{
			name:            "Empty: Add single: 1.mwcli.test",
			startingContent: "",
			args: args{
				IP:    "127.0.0.1",
				toAdd: []string{"1.mwcli.test"},
			},
			want: ChangeResult{
				Success:   true,
				Altered:   true,
				Content:   "127.0.0.1        1.mwcli.test\n",
				WriteFile: "",
			},
		},
		{
			name:            "Empty: Add single: 1.mwcli.test with alternative IP",
			startingContent: "",
			args: args{
				IP:    "1.2.3.4",
				toAdd: []string{"1.mwcli.test"},
			},
			want: ChangeResult{
				Success:   true,
				Altered:   true,
				Content:   "1.2.3.4          1.mwcli.test\n",
				WriteFile: "",
			},
		},
		{
			name:            "Empty: Add two: 1.mwcli.test, 2.mwcli.test",
			startingContent: "",
			args: args{
				IP:    "127.0.0.1",
				toAdd: []string{"1.mwcli.test", "2.mwcli.test"},
			},
			want: ChangeResult{
				Success:   true,
				Altered:   true,
				Content:   "127.0.0.1        1.mwcli.test 2.mwcli.test\n",
				WriteFile: "",
			},
		},
		{
			name:            "singleLocalHost: Add two: 1.mwcli.test, 2.mwcli.test",
			startingContent: singleLocalHost,
			args: args{
				IP:    "127.0.0.1",
				toAdd: []string{"1.mwcli.test", "2.mwcli.test"},
			},
			want: ChangeResult{
				Success:   true,
				Altered:   true,
				Content:   "127.0.0.1        iam.localhost 1.mwcli.test 2.mwcli.test\n",
				WriteFile: "",
			},
		},
		{
			name:            "singleOtherHost: Add two: 1.mwcli.test, 2.mwcli.test",
			startingContent: singleOtherHost,
			args: args{
				IP:    "127.0.0.1",
				toAdd: []string{"1.mwcli.test", "2.mwcli.test"},
			},
			want: ChangeResult{
				Success:   true,
				Altered:   true,
				Content:   "123.123.111.111  iam.not.localhost\n127.0.0.1        1.mwcli.test 2.mwcli.test\n",
				WriteFile: "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup a test file
			testFile := writeContentToTmpFile(tt.startingContent)
			hostsFile = testFile
			tt.want.WriteFile = testFile

			// Perform the test!
			if got := AddHosts(tt.args.IP, tt.args.toAdd, true); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AddHosts() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRemoveHostsWithSuffix(t *testing.T) {
	type args struct {
		hostSuffix string
		IP         string
	}
	tests := []struct {
		startingContent string
		name            string
		args            args
		want            ChangeResult
	}{
		{
			name:            "Remove mwcli.test suffix, resulting in same content",
			startingContent: singleLocalHost,
			args: args{
				IP:         "127.0.0.1",
				hostSuffix: "mwcli.test",
			},
			want: ChangeResult{
				Success:   true,
				Altered:   false,
				Content:   singleLocalHost,
				WriteFile: "",
			},
		},
		{
			name:            "Remove mwcli.test suffix, removing 2, resulting in nothing",
			startingContent: twoHostsToRemoveOnly,
			args: args{
				IP:         "127.0.0.1",
				hostSuffix: "mwcli.test",
			},
			want: ChangeResult{
				Success:   true,
				Altered:   true,
				Content:   "",
				WriteFile: "",
			},
		},
		{
			name:            "Remove mwcli.test suffix, removing 2, resulting in 1 left",
			startingContent: twoHostsToRemoveFromLocal,
			args: args{
				IP:         "127.0.0.1",
				hostSuffix: "mwcli.test",
			},
			want: ChangeResult{
				Success:   true,
				Altered:   true,
				Content:   singleLocalHost,
				WriteFile: "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup a test file
			testFile := writeContentToTmpFile(tt.startingContent)
			hostsFile = testFile
			tt.want.WriteFile = testFile

			// Perform the test!
			if got := RemoveHostsWithSuffix(tt.args.IP, tt.args.hostSuffix, true); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RemoveHostsWithSuffix() = %v, want %v", got, tt.want)
			}
		})
	}
}
