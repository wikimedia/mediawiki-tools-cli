package output

import (
	"fmt"
	"io"
)

func printJQ(objects interface{}, format string, writer io.Writer) error {
	return PrintJSONChecked(objects, format, true, writer)
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
