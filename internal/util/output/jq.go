package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/cli/cli/v2/pkg/jsoncolor"
	"github.com/itchyny/gojq"
	"github.com/sirupsen/logrus"
)

// printJQ executes a jq filter against objects and writes the result to writer.
// String values are printed raw (like jq -r); other values are printed as JSON.
// If format is empty it falls back to plain JSON output.
func printJQ(objects interface{}, format string, writer io.Writer) {
	if format == "" {
		NewJSON(objects, ".").Print(writer)
		return
	}

	query, err := gojq.Parse(format)
	if err != nil {
		logrus.Errorf("jq: parse error: %v", err)
		return
	}

	// Normalise input via JSON round-trip so gojq can handle all input types.
	var obj interface{}
	data, err := json.Marshal(normalizeInput(objects))
	if err != nil {
		logrus.Errorf("jq: marshal error: %v", err)
		return
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		logrus.Errorf("jq: unmarshal error: %v", err)
		return
	}

	iter := query.Run(obj)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, isErr := v.(error); isErr {
			logrus.Errorln(err)
			continue
		}
		// Print strings raw (like jq -r), objects/arrays as JSON.
		switch s := v.(type) {
		case string:
			fmt.Fprintln(writer, s)
		default:
			if shouldColor() {
				err := jsoncolor.Write(writer, strings.NewReader(interfaceToJSONString(v)), "  ")
				if err != nil {
					logrus.Errorln(err)
				}
			} else {
				fmt.Fprintf(writer, "%v\n", interfaceToJSONString(v))
			}
		}
	}
}

// normalizeInput recursively converts maps with non-string keys into
// map[string]interface{} so they can be marshaled to JSON.
func normalizeInput(in interface{}) interface{} {
	switch typed := in.(type) {
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(typed))
		for key, value := range typed {
			out[fmt.Sprintf("%v", key)] = normalizeInput(value)
		}
		return out
	case map[string]interface{}:
		out := make(map[string]interface{}, len(typed))
		for key, value := range typed {
			out[key] = normalizeInput(value)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(typed))
		for i, value := range typed {
			out[i] = normalizeInput(value)
		}
		return out
	default:
		return in
	}
}
