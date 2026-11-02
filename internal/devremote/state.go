package devremote

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

// Context records development location and capabilities, never service credentials.
type Context struct {
	Options
	Workspace  string   `json:"workspace"`
	Metadata   string   `json:"metadata"`
	Source     string   `json:"source"`
	SourceName string   `json:"source_name"`
	Directory  string   `json:"directory,omitempty"`
	Forward    []string `json:"forward,omitempty"`
	GitRemote  string   `json:"git_remote,omitempty"`
	WebPort    int      `json:"web_port,omitempty"`
}

type sessionState struct {
	PID           int       `json:"pid"`
	Control       string    `json:"control"`
	SSHControl    string    `json:"ssh_control,omitempty"`
	Executable    string    `json:"executable,omitempty"`
	Stopped       bool      `json:"stopped,omitempty"`
	BinaryRelease string    `json:"binary_release,omitempty"`
	Ready         bool      `json:"ready"`
	Error         string    `json:"error,omitempty"`
	Started       time.Time `json:"started"`
}

func executableSignature() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	info, err := os.Stat(executable)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s:%d:%d", executable, info.Size(), info.ModTime().UnixNano())
}

func contextDirectory(name string) (string, error) {
	if !safeName.MatchString(name) {
		return "", fmt.Errorf("invalid context name %q", name)
	}
	return filepath.Join(cli.UserDirectoryPathForCmd("remote"), "contexts", name), nil
}

// LoadContext finds a saved remote backend without initializing a local environment.
func LoadContext(name string) (Context, error) {
	directory, err := contextDirectory(name)
	if err != nil {
		return Context{}, err
	}
	var c Context
	data, err := os.ReadFile(filepath.Join(directory, "context.json"))
	if err == nil {
		err = json.Unmarshal(data, &c)
	}
	if err == nil && c.Context != name {
		err = errors.New("saved remote context name does not match its directory")
	}
	if err == nil && (len(c.SSHArgs) == 0 || ValidateHost(c.SSHArgs[len(c.SSHArgs)-1]) != nil) {
		err = errors.New("invalid saved SSH target")
	}
	if err == nil && c.Workspace != "" && (!strings.HasPrefix(c.Metadata, c.Workspace+"/") || !strings.HasPrefix(c.Source, c.Workspace+"/")) {
		err = errors.New("invalid saved workspace paths")
	}
	return c, err
}

// SaveContext atomically records a remote backend.
func SaveContext(c Context) error {
	directory, err := contextDirectory(c.Context)
	if err != nil {
		return err
	}
	root := filepath.Dir(directory)
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	lock := flock.New(filepath.Join(root, "registry.lock"))
	if err := lock.Lock(); err != nil {
		return err
	}
	defer lock.Unlock()
	if existing, err := LoadContext(c.Context); err == nil && !slices.Equal(existing.SSHArgs, c.SSHArgs) {
		return fmt.Errorf("context %s is already registered for another SSH target", c.Context)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	return atomicJSON(filepath.Join(directory, "context.json"), c)
}

func atomicJSON(filename string, value any) error {
	if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(filename), ".state-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(append(data, '\n')); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filename)
}

// NewContextName chooses a memorable name not already used locally.
func NewContextName() (string, error) {
	adjectives := []string{"amber", "bright", "calm", "clear", "coral", "cosmic", "gentle", "golden", "green", "happy", "indigo", "jolly", "lucky", "lunar", "mellow", "misty", "nimble", "noble", "olive", "orange", "quiet", "rapid", "silver", "small", "snowy", "solar", "steady", "sunny", "swift", "tidy", "warm", "wild"}
	nouns := []string{"badger", "beaver", "birch", "cedar", "comet", "crane", "dolphin", "falcon", "finch", "forest", "fox", "gecko", "heron", "island", "koala", "lake", "lark", "lynx", "maple", "meadow", "otter", "owl", "panda", "pine", "puffin", "raven", "river", "robin", "sparrow", "tiger", "willow", "wren"}
	for i := 0; i < 128; i++ {
		var b [2]byte
		if _, err := rand.Read(b[:]); err != nil {
			return "", err
		}
		name := adjectives[int(b[0])%len(adjectives)] + "-" + nouns[int(b[1])%len(nouns)]
		if _, err := os.Stat(filepath.Join(cli.UserDirectoryPathForCmd("mwdd"), name)); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return "", err
		}
		_, err := LoadContext(name)
		if os.IsNotExist(err) {
			return name, nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", errors.New("could not find an unused context name; specify --context")
}

// Contexts lists registered remote contexts.
func Contexts() ([]Context, error) {
	entries, err := os.ReadDir(filepath.Join(cli.UserDirectoryPathForCmd("remote"), "contexts"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var result []Context
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		c, err := LoadContext(entry.Name())
		if err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, nil
}

func loadSession(name string) (sessionState, error) {
	dir, err := contextDirectory(name)
	if err != nil {
		return sessionState{}, err
	}
	var s sessionState
	data, err := os.ReadFile(filepath.Join(dir, "session.json"))
	if err == nil {
		err = json.Unmarshal(data, &s)
	}
	return s, err
}

func saveSession(name string, s sessionState) error {
	dir, err := contextDirectory(name)
	if err != nil {
		return err
	}
	return atomicJSON(filepath.Join(dir, "session.json"), s)
}
