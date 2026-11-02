package output

import (
	"encoding/json"
	"io"
)

type JSON struct {
	Objects    interface{}
	Format     string
	parseError error
}

func NewJSON(objects interface{}, format string) *JSON {
	return &JSON{Objects: objects, Format: format}
}

func NewJSONFromString(objects, format string) *JSON {
	var value interface{}
	err := json.Unmarshal([]byte(objects), &value)
	return &JSON{Objects: value, Format: format, parseError: err}
}

func (j *JSON) Print(writer io.Writer) error {
	if j.parseError != nil {
		return j.parseError
	}
	return PrintJSONChecked(j.Objects, j.Format, false, writer)
}
