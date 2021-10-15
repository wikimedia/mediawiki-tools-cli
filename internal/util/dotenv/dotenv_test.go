/*Package dotenv for interacting with a .env file

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
package dotenv

import (
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"
)

func randomString() string {
	// A bit of randomness so that we dont need to open a file for our non existent test
	rand.Seed(time.Now().UnixNano())
	chars := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890")
	var b strings.Builder
	for i := 0; i < 10; i++ {
		b.WriteRune(chars[rand.Intn(len(chars))])
	}
	return b.String()
}

func TestFile_Path(t *testing.T) {
	tests := []struct {
		name string
		f    File
		want string
	}{
		{
			name: "Path even if it doesnt exist",
			f:    File("/tmp/mwcli-test-dotenv-not-created"),
			want: "/tmp/mwcli-test-dotenv-not-created",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.f.Path(); got != tt.want {
				t.Errorf("File.Path() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFile_EnsureExists(t *testing.T) {
	emptyDotEnvPath := "/tmp/mwcli-test-dotenv-" + randomString()
	emptyDotEnv, err := os.Create(emptyDotEnvPath)
	if err != nil {
		panic(err)
	}
	emptyDotEnv.Close()

	tests := []struct {
		name string
		f    File
	}{
		{
			name: "EnsureExists creates file",
			f:    File("/tmp/mwcli-test-dotenv-" + randomString()),
		},
		{
			name: "EnsureExists works with existing file",
			f:    File(emptyDotEnvPath),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.f.EnsureExists()

			// Check it exists
			_, err := os.Stat(tt.f.Path())
			if err != nil {
				t.Errorf("File.EnsureExists() failed to create file: %v", err)
			}

			// And is empty
			b, err := ioutil.ReadFile(tt.f.Path())
			if string(b) != "" {
				t.Errorf("File.EnsureExists() failed to create empty file: %v", err)
			}
		})
	}
}

func TestFile_GeneralIntegration(t *testing.T) {
	envFile := File("/tmp/mwcli-test-dotenv-" + randomString())
	envFile.EnsureExists()

	tests := []struct {
		name           string
		f              File
		applyAndIsGood func(f File) bool
		expectList     map[string]string
	}{
		{
			name: "Has no non existing value",
			applyAndIsGood: func(f File) bool {
				return f.Has("FOO") == false
			},
		},
		{
			name: "So it is missing",
			applyAndIsGood: func(f File) bool {
				return f.Missing("FOO") == true
			},
		},
		{
			name: "Set a single value, now has it",
			applyAndIsGood: func(f File) bool {
				f.Set("FOO", "AVALUE")
				return f.Has("FOO") == true
			},
		},
		{
			name: "Set another, now we have 2",
			applyAndIsGood: func(f File) bool {
				f.Set("BAR", "BVALUE")
				return f.Has("FOO") == true && f.Get("BAR") == "BVALUE"
			},
		},
		{
			name: "Set one that already exists to a new value, it changes",
			applyAndIsGood: func(f File) bool {
				f.Set("FOO", "CVALUE")
				return f.Get("FOO") == "CVALUE"
			},
		},
		{
			name: "They appear in List too",
			applyAndIsGood: func(f File) bool {
				return f.List()["FOO"] == "CVALUE" && f.List()["BAR"] == "BVALUE" && len(f.List()) == 2
			},
		},
		{
			name: "Delete one, and we have 1",
			applyAndIsGood: func(f File) bool {
				f.Delete("BAR")
				return f.Has("FOO") == true && f.Has("BAR") == false
			},
		},
		{
			name: "Delete the last one, and it is empty",
			applyAndIsGood: func(f File) bool {
				f.Delete("FOO")
				return len(f.List()) == 0
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isGood := tt.applyAndIsGood(envFile)
			if !isGood {
				t.Errorf("%q failed "+envFile.Path(), tt.name)
			}
		})
	}
}
