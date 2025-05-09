package output

/*
Output order is not guaranteed.
*/
import (
	"encoding/json"
	"fmt"
	"io"
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

func NewJSONFromString(objects string, format string) *JSON {
	var obj map[string]interface{}
	err := json.Unmarshal([]byte(objects), &obj)
	if err != nil {
		logrus.Panic(err)
	}

	convertedObjects := make(map[interface{}]interface{})
	for key, value := range obj {
		convertedObjects[key] = value
	}

	return &JSON{
		Objects: convertedObjects,
		Format:  format,
	}
}

func (j *JSON) Print(writer io.Writer) {
	printWithKeys(j, writer)
}

func printWithKeys(j *JSON, writer io.Writer) {
	query := parseFormatQueryOrPanic(j.Format)

	// Convert from interface => interface, to string => interface
	mapOfInterfaces := make(map[string]interface{}, len(j.Objects))
	for key, value := range j.Objects {
		mapOfInterfaces[fmt.Sprintf("%v", key)] = value
	}

	marshalAndPrint(mapOfInterfaces, query, writer)
}

func parseFormatQueryOrPanic(format string) *gojq.Query {
	query, err := gojq.Parse(format)
	if err != nil {
		logrus.Panic(err)
	}
	logrus.Trace(query.String())
	return query
}

func marshalAndPrint(in interface{}, query *gojq.Query, writer io.Writer) {
	// Convert to a map of interfaces so the j lib doesn't complain about our types
	var mapOfInterfaces map[string]interface{}
	data, _ := json.Marshal(in)
	err := json.Unmarshal(data, &mapOfInterfaces)
	if err != nil {
		panic(err)
	}

	iter := query.Run(mapOfInterfaces) // or query.RunWithContext
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, isErr := v.(error); isErr {
			logrus.Errorln(err)
		}

		if shouldColor() {
			err := jsoncolor.Write(writer, strings.NewReader(interfaceToJSONString(v)), "  ")
			if err != nil {
				panic(err)
			}
		} else {
			fmt.Fprintf(writer, "%v\n", interfaceToJSONString(v))
		}
	}
}

func interfaceToJSONString(v interface{}) string {
	byteSlice, _ := json.Marshal(v)
	return string(byteSlice)
}

func JSONStringToInterface(jsonString string) interface{} {
	var obj interface{}
	err := json.Unmarshal([]byte(jsonString), &obj)
	if err != nil {
		logrus.Panic(err)
	}
	return obj
}
