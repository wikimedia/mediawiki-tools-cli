package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

type Data struct {
	Images []struct {
		Image        string `yaml:"image"`
		RequireRegex string `yaml:"requireRegex,omitempty"`
		NoCheck      bool   `yaml:"noCheck,omitempty"`
	} `yaml:"images"`
	Directories []string `yaml:"directories"`
	Files       []string `yaml:"files"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || len(args)%2 != 0 {
		return fmt.Errorf("usage: update OLD_IMAGE NEW_IMAGE [OLD_IMAGE NEW_IMAGE ...]")
	}
	data, err := getData()
	if err != nil {
		return err
	}
	for i := 0; i < len(args); i += 2 {
		find, replacement := args[i], args[i+1]
		if find == "" {
			return fmt.Errorf("image to replace cannot be empty")
		}
		fmt.Fprintf(os.Stderr, "Replacing %s with %s in files and directories defined in data.yml\n", find, replacement)
		for _, directory := range data.Directories {
			if err := replaceInDirectory(directory, find, replacement); err != nil {
				return err
			}
		}
		for _, file := range data.Files {
			if err := replaceInFile(file, find, replacement); err != nil {
				return err
			}
		}
		if err := replaceInFile("tools/image-update/data.yml", find, replacement); err != nil {
			return err
		}
	}
	return nil
}

func replaceInDirectory(directory, find, replacement string) error {
	files, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("read directory %s: %w", directory, err)
	}
	for _, file := range files {
		path := filepath.Join(directory, file.Name())
		if file.IsDir() {
			if err := replaceInDirectory(path, find, replacement); err != nil {
				return err
			}
		} else if err := replaceInFile(path, find, replacement); err != nil {
			return err
		}
	}
	return nil
}

func replaceInFile(path, find, replacement string) error {
	content, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	text := strings.ReplaceAll(string(content), find, replacement)
	if text == string(content) {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(text), info.Mode().Perm()); err != nil { // #nosec G703 -- path comes from repository-maintained image update metadata.
		return fmt.Errorf("write %s: %w", path, err)
	}
	fmt.Fprintln(os.Stderr, "Updated", path)
	return nil
}

func getData() (Data, error) {
	var data Data
	content, err := os.ReadFile("tools/image-update/data.yml")
	if err != nil {
		return data, fmt.Errorf("read image data: %w", err)
	}
	if err := yaml.Unmarshal(content, &data); err != nil {
		return data, fmt.Errorf("parse image data: %w", err)
	}
	return data, nil
}
