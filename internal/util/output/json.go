package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/cli/cli/v2/pkg/jsoncolor"
	"github.com/itchyny/gojq"
	"github.com/mattn/go-isatty"
	"github.com/sirupsen/logrus"
)

type JSON struct {
	Objects []interface{}
	Format  string
}

func NewJSON(objects []interface{}, format string) *JSON {
	return &JSON{
		Objects: objects,
		Format:  format,
	}
}

func (j *JSON) Print() {
	query, err := gojq.Parse(j.Format)
	if err != nil {
		logrus.Panic(err)
	}
	logrus.Trace(query.String())

	for _, obj := range j.Objects {
		// Convert to a map of interfaces so the j lib doesnt complain about our types
		var mapOfInterfaces map[string]interface{}
		data, _ := json.Marshal(obj)
		json.Unmarshal(data, &mapOfInterfaces)

		iter := query.Run(mapOfInterfaces) // or query.RunWithContext
		for {
			v, ok := iter.Next()
			if !ok {
				break
			}
			if err, isErr := v.(error); isErr {
				logrus.Fatalln(err)
			}

			if shouldColor() {
				jsoncolor.Write(os.Stdout, strings.NewReader(interfaceToJSONString(v)), "  ")
			} else {
				fmt.Printf("%v\n", interfaceToJSONString(v))
			}
		}
	}
}

func shouldColor() bool {
	// From https://github.com/cli/cli/blob/bf83c660a1ae486d582117e0a174f8e109b64775/pkg/iostreams/iostreams.go#L389
	stdoutIsTTY := isTerminal(os.Stdout)
	return stdoutIsTTY
}

func isTerminal(f *os.File) bool {
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}

func interfaceToJSONString(v interface{}) string {
	byteSlice, _ := json.Marshal(v)
	return string(byteSlice)
}
