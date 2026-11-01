package ziki

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

type terminalIO struct {
	io.Reader
	io.Writer
}

// Keep history in memory for this game, and leave redirected input line-oriented.
func interactiveReader(input io.Reader, output io.Writer) func(string) (string, error) {
	in, ok := input.(*os.File)
	if !ok || !term.IsTerminal(int(in.Fd())) || os.Getenv("TERM") == "dumb" {
		return nil
	}
	out, ok := output.(*os.File)
	if !ok || !term.IsTerminal(int(out.Fd())) {
		return nil
	}
	terminal := term.NewTerminal(terminalIO{Reader: in, Writer: out}, "")
	return func(prompt string) (string, error) {
		state, err := term.MakeRaw(int(in.Fd()))
		if err != nil {
			return "", err
		}
		defer term.Restore(int(in.Fd()), state)
		if width, height, err := term.GetSize(int(out.Fd())); err == nil && width > 0 && height > 0 {
			terminal.SetSize(width, height)
		}
		if strings.HasPrefix(prompt, "\n") {
			fmt.Fprint(out, "\r\n")
			prompt = strings.TrimPrefix(prompt, "\n")
		}
		terminal.SetPrompt(prompt)
		return terminal.ReadLine()
	}
}
