package files

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// AddLineUnique appends a complete line only if it is not already present.
func AddLineUnique(line, fileName string) error {
	lines, err := Lines(fileName)
	if err != nil {
		return err
	}
	for _, existing := range lines {
		if existing == line {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(fileName), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Clean(fileName), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := file.WriteString(line + "\n")
	return errors.Join(writeErr, file.Close())
}

// RemoveAllLinesMatching removes complete matching lines, preserving other text.
func RemoveAllLinesMatching(line, fileName string) error {
	data, err := os.ReadFile(filepath.Clean(fileName))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	parts := strings.Split(string(data), "\n")
	kept := parts[:0]
	for _, part := range parts {
		if part != line {
			kept = append(kept, part)
		}
	}
	return os.WriteFile(filepath.Clean(fileName), []byte(strings.Join(kept, "\n")), 0o600)
}

// Lines returns complete lines. An absent file represents an empty list.
func Lines(fileName string) ([]string, error) {
	file, err := os.Open(filepath.Clean(fileName))
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	lines := []string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func Exists(fileName string) bool {
	_, err := os.Stat(fileName)
	return !os.IsNotExist(err)
}

func RemoveIfExists(fileName string) error {
	err := os.Remove(fileName)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func AddLinesUnique(lines []string, filename string) error {
	for _, line := range lines {
		if err := AddLineUnique(line, filename); err != nil {
			return err
		}
	}
	return nil
}
