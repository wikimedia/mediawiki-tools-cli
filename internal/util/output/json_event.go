package output

import (
	"bytes"
	"encoding/json"
	"io"
	"os"

	"github.com/cli/cli/v2/pkg/jsoncolor"
)

// PrintJSONEvent writes one event immediately: indented JSON on a terminal,
// or a single compact JSON line when redirected. It preserves JSON numbers.
func PrintJSONEvent(writer io.Writer, data []byte) error {
	file, ok := writer.(*os.File)
	return printJSONEvent(writer, data, ok && isTerminal(file))
}

func printJSONEvent(writer io.Writer, data []byte, terminal bool) error {
	var compacted bytes.Buffer
	if err := json.Compact(&compacted, data); err != nil {
		return err
	}

	var rendered bytes.Buffer
	switch {
	case !terminal:
		_, _ = rendered.Write(compacted.Bytes())
	case os.Getenv("NO_COLOR") == "" && os.Getenv("CLICOLOR") != "0" && os.Getenv("TERM") != "dumb":
		// Render into a buffer because jsoncolor does not propagate writer errors.
		if err := jsoncolor.Write(&rendered, bytes.NewReader(data), "  "); err != nil {
			return err
		}
	default:
		if err := json.Indent(&rendered, bytes.TrimSpace(data), "", "  "); err != nil {
			return err
		}
	}
	if rendered.Len() == 0 || rendered.Bytes()[rendered.Len()-1] != '\n' {
		rendered.WriteByte('\n')
	}
	_, err := rendered.WriteTo(writer)
	return err
}
