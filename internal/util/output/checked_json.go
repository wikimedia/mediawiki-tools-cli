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
func PrintJSONChecked(objects interface{}, format string, rawStrings bool, writer io.Writer) error {
	var buffer bytes.Buffer
	if err := printJSONStream(objects, format, rawStrings, &buffer); err != nil {
		return err
	}
	_, err := buffer.WriteTo(writer)
	return err
}

func printJSONStream(objects interface{}, format string, rawStrings bool, writer io.Writer) error {
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
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err = decoder.Decode(&value); err != nil {
		return err
	}
	color := shouldColor()
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
			if _, err := fmt.Fprintln(writer, text); err != nil {
				return err
			}
			continue
		}
		encoded, err := json.Marshal(item)
		if err != nil {
			return err
		}
		if color {
			if err := jsoncolor.Write(writer, bytes.NewReader(encoded), "  "); err != nil {
				return err
			}
			encoded = nil
		}
		if _, err := writer.Write(append(encoded, '\n')); err != nil {
			return err
		}
	}
	return nil
}
