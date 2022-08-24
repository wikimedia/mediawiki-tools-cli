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
	// A bit of randomness so that we don't need to open a file for our non existent test
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
			name: "Path even if it doesn't exist",
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
