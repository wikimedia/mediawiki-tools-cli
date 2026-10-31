package wiki

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/muesli/cancelreader"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiauth"
	"golang.org/x/term"
)

type callbackInputResult struct {
	URL string
	Err error
}
type callbackInput struct {
	results <-chan callbackInputResult
	stop    func()
}
type callbackTerminalIO struct {
	io.Reader
	io.Writer
}

// A cancellable terminal read is essential: after an HTTP callback wins, a
// leftover password reader must not consume the next password-manager prompt.
func startCallbackInput(cmd *cobra.Command, a wikiauth.Authorization) (*callbackInput, error) {
	file, ok := cmd.InOrStdin().(*os.File)
	if !ok || !term.IsTerminal(int(file.Fd())) {
		return nil, nil
	}
	state, err := term.MakeRaw(int(file.Fd()))
	if err != nil {
		return nil, fmt.Errorf("could not prepare hidden callback input")
	}
	reader, err := cancelreader.NewReader(file)
	if err != nil {
		_ = term.Restore(int(file.Fd()), state)
		return nil, fmt.Errorf("could not prepare cancellable callback input")
	}
	terminal := term.NewTerminal(callbackTerminalIO{Reader: reader, Writer: cmd.ErrOrStderr()}, "")
	_, _ = terminal.Write([]byte("Waiting for your browser...\nIf the redirect fails, copy the full URL from the address bar, paste it here, and press Enter.\nYour paste won't be displayed.\n"))
	results := make(chan callbackInputResult, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			line, err := terminal.ReadPassword("Callback URL (paste and press Enter): ")
			if err != nil && err != term.ErrPasteIndicator {
				results <- callbackInputResult{Err: fmt.Errorf("callback input was closed or cancelled")}
				return
			}
			line = strings.TrimSpace(line)
			if len(line) > 16<<10 {
				_, _ = terminal.Write([]byte("Callback URL is too long; paste it again.\n"))
				continue
			}
			if _, err := a.Code(line); err != nil {
				_, _ = terminal.Write([]byte("Callback URL does not match this login or authorization was declined; paste the correct URL or cancel.\n"))
				continue
			}
			results <- callbackInputResult{URL: line}
			return
		}
	}()
	return &callbackInput{results: results, stop: func() {
		_ = reader.Cancel()
		<-done
		_ = reader.Close()
		_ = term.Restore(int(file.Fd()), state)
		cmd.PrintErrln()
	}}, nil
}

func readManualCallback(cmd *cobra.Command, a wikiauth.Authorization) (string, error) {
	cmd.PrintErr("Paste the full callback URL: ")
	line, err := bufio.NewReader(io.LimitReader(cmd.InOrStdin(), (16<<10)+1)).ReadString('\n')
	if err != nil && !(err == io.EOF && strings.TrimSpace(line) != "") {
		return "", fmt.Errorf("could not read callback URL")
	}
	if len(line) > 16<<10 {
		return "", fmt.Errorf("callback URL is too long")
	}
	line = strings.TrimSpace(line)
	if _, err := a.Code(line); err != nil {
		return "", fmt.Errorf("callback URL is invalid for this login or authorization was declined")
	}
	return line, nil
}
