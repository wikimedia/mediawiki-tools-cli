package dotenv

import (
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

// Read returns environment values and reports filesystem or parse failures.
func (f File) Read() (map[string]string, error) {
	if err := f.EnsureExists(); err != nil {
		return nil, err
	}
	return godotenv.Read(f.Path())
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
	values, err := f.Read()
	if err != nil {
		return err
	}
	delete(values, name)
	return writeOverride(values, f.Path())
}

// Set changes a setting, preserving other values.
func (f File) Set(name string, value string) error {
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
