package dotenv

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

/*File location of the .env file to work on.*/
type File string

/*FileForDirectory returns the File for the given directory.*/
func FileForDirectory(directory string) File {
	return File(directory + string(os.PathSeparator) + ".env")
}

/*Path the path of the .env file.*/
func (f File) Path() string {
	return string(f)
}

/*EnsureExists ensures that the File exists, creating an empty one if not.*/
func (f File) EnsureExists() error {
	if _, err := os.Stat(f.Path()); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(f.Path()), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(f.Path(), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if os.IsExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return file.Close()
}

// Read loads the environment without creating a missing file.
func (f File) Read() (map[string]string, error) {
	data, err := os.ReadFile(f.Path())
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	values, err := godotenv.UnmarshalBytes(data)
	if err != nil {
		// Parser errors can quote values from the file.
		return nil, fmt.Errorf("invalid dotenv syntax in %q", f.Path())
	}
	return values, nil
}

func (f File) read() map[string]string {
	values, err := f.Read()
	if err != nil {
		slog.Error("read dotenv file", "path", f.Path(), "error", err)
		return map[string]string{}
	}
	return values
}

// Delete removes a setting, preserving other values.
func (f File) Delete(name string) error {
	if err := f.EnsureExists(); err != nil {
		return err
	}
	values, err := f.Read()
	if err != nil {
		return err
	}
	delete(values, name)
	return writeOverride(values, f.Path())
}

// Set changes a setting, preserving other values.
func (f File) Set(name string, value string) error {
	if err := f.EnsureExists(); err != nil {
		return err
	}
	values, err := f.Read()
	if err != nil {
		return err
	}
	values[name] = value
	return writeOverride(values, f.Path())
}

/*Get a value from the env.*/
func (f File) Get(name string) string {
	envMap := f.read()
	return envMap[name]
}

/*Has a value in the env.*/
func (f File) Has(name string) bool {
	envMap := f.read()
	_, ok := envMap[name]
	return ok
}

/*Missing a value in the env.*/
func (f File) Missing(name string) bool {
	return !f.Has(name)
}

/*List all values from the env.*/
func (f File) List() map[string]string {
	return f.read()
}
