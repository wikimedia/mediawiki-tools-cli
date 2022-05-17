package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/cli/cli/v2/pkg/jsoncolor"
	"github.com/itchyny/gojq"
	"github.com/sirupsen/logrus"
)

type JSON struct {
	Objects map[interface{}]interface{}
	Format  string
}

func NewJSON(objects map[interface{}]interface{}, format string) *JSON {
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
		// Convert to a map of interfaces so the j lib doesn't complain about our types
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

func interfaceToJSONString(v interface{}) string {
	byteSlice, _ := json.Marshal(v)
	return string(byteSlice)
}
