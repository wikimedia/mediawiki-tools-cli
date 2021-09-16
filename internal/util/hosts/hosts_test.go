/*Package hosts in internal utils is functionality for interacting with an etc hosts file

Copyright © 2020 Addshore

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/
package hosts

import (
	"io/ioutil"
	"os"
	"reflect"
	"testing"
)

var singleLocalHost = "127.0.0.1        iam.localhost\n"
var singleOtherHost = "123.123.111.111        iam.not.localhost\n"
var twoHostsToRemoveOnly = "127.0.0.1        1.mwcli.test 2.mwcli.test\n"
var twoHostsToRemoveFromLocal = "127.0.0.1        iam.localhost 1.mwcli.test 2.mwcli.test\n"

func writeContentToTmpFile(content string) string {
	tmpFile, err := ioutil.TempFile(os.TempDir(), hostsTmpPrefix+"test-")
	if err != nil {
		panic(err)
	}
	tmpFile.WriteString(content)
	tmpFile.Close()
	return tmpFile.Name()
}

func TestAddHosts(t *testing.T) {
	type args struct {
		toAdd []string
	}
	tests := []struct {
		startingContent string
		name            string
		args            args
		want            Save
	}{
		{
			name:            "Empty: Add single: 1.mwcli.test",
			startingContent: "",
			args: args{
				toAdd: []string{"1.mwcli.test"},
			},
			want: Save{
				Success: true,
				Content: "127.0.0.1        1.mwcli.test\n",
				TmpFile: "",
			},
		},
		{
			name:            "Empty: Add two: 1.mwcli.test, 2.mwcli.test",
			startingContent: "",
			args: args{
				toAdd: []string{"1.mwcli.test", "2.mwcli.test"},
			},
			want: Save{
				Success: true,
				Content: "127.0.0.1        1.mwcli.test 2.mwcli.test\n",
				TmpFile: "",
			},
		},
		{
			name:            "singleLocalHost: Add two: 1.mwcli.test, 2.mwcli.test",
			startingContent: singleLocalHost,
			args: args{
				toAdd: []string{"1.mwcli.test", "2.mwcli.test"},
			},
			want: Save{
				Success: true,
				Content: "127.0.0.1        iam.localhost 1.mwcli.test 2.mwcli.test\n",
				TmpFile: "",
			},
		},
		{
			name:            "singleOtherHost: Add two: 1.mwcli.test, 2.mwcli.test",
			startingContent: singleOtherHost,
			args: args{
				toAdd: []string{"1.mwcli.test", "2.mwcli.test"},
			},
			want: Save{
				Success: true,
				Content: "123.123.111.111  iam.not.localhost\n127.0.0.1        1.mwcli.test 2.mwcli.test\n",
				TmpFile: "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup a test file
			testFile := writeContentToTmpFile(tt.startingContent)
			hostsFile = testFile
			tt.want.TmpFile = testFile

			// Perform the test!
			if got := AddHosts(tt.args.toAdd); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AddHosts() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRemoveHostsWithSuffix(t *testing.T) {
	type args struct {
		hostSuffix string
	}
	tests := []struct {
		startingContent string
		name            string
		args            args
		want            Save
	}{
		{
			name:            "Remove mwcli.test suffix, resulting in same content",
			startingContent: singleLocalHost,
			args: args{
				hostSuffix: "mwcli.test",
			},
			want: Save{
				Success: true,
				Content: singleLocalHost,
				TmpFile: "",
			},
		},
		{
			name:            "Remove mwcli.test suffix, removing 2, resulting in nothing",
			startingContent: twoHostsToRemoveOnly,
			args: args{
				hostSuffix: "mwcli.test",
			},
			want: Save{
				Success: true,
				Content: "",
				TmpFile: "",
			},
		},
		{
			name:            "Remove mwcli.test suffix, removing 2, resulting in 1 left",
			startingContent: twoHostsToRemoveFromLocal,
			args: args{
				hostSuffix: "mwcli.test",
			},
			want: Save{
				Success: true,
				Content: singleLocalHost,
				TmpFile: "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup a test file
			testFile := writeContentToTmpFile(tt.startingContent)
			hostsFile = testFile
			tt.want.TmpFile = testFile

			// Perform the test!
			if got := RemoveHostsWithSuffix(tt.args.hostSuffix); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RemoveHostsWithSuffix() = %v, want %v", got, tt.want)
			}
		})
	}
}
