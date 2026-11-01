package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/cli/cli/v2/pkg/jsoncolor"
	"github.com/itchyny/gojq"
)

// ValidateJSONFormat checks filters before callers perform requests or edits.
func ValidateJSONFormat(format string) error {
	if format == "" {
		format = "."
	}
	query, err := gojq.Parse(format)
	if err != nil {
		return fmt.Errorf("jq: %w", err)
	}
	if _, err = gojq.Compile(query); err != nil {
		return fmt.Errorf("jq: %w", err)
	}
	return nil
}

// PrintJSONChecked preserves JSON versus jq raw-string output semantics while
// returning filter/encoding/write failures to the command. Results are buffered
// so a runtime filter error does not leave a misleading partial JSON stream.
// Existing Print callers retain their historical logging behavior.
func PrintJSONChecked(objects interface{}, format string, rawStrings bool, writer io.Writer) error {
	if format == "" {
		format = "."
	}
	query, err := gojq.Parse(format)
	if err != nil {
		return fmt.Errorf("jq: %w", err)
	}
	code, err := gojq.Compile(query)
	if err != nil {
		return fmt.Errorf("jq: %w", err)
	}
	data, err := json.Marshal(normalizeInput(objects))
	if err != nil {
		return err
	}
	var value interface{}
	if err = json.Unmarshal(data, &value); err != nil {
		return err
	}
	var buffer bytes.Buffer
	iter := code.Run(value)
	for {
		item, ok := iter.Next()
		if !ok {
			break
		}
		if failure, ok := item.(error); ok {
			return fmt.Errorf("jq: %w", failure)
		}
		if text, ok := item.(string); ok && rawStrings {
			fmt.Fprintln(&buffer, text)
			continue
		}
		encoded, err := json.Marshal(item)
		if err != nil {
			return err
		}
		if shouldColor() {
			if err = jsoncolor.Write(&buffer, bytes.NewReader(encoded), "  "); err != nil {
				return err
			}
		} else {
			buffer.Write(encoded)
		}
		buffer.WriteByte('\n')
	}
	_, err = buffer.WriteTo(writer)
	return err
}
