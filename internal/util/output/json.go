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

type JSON struct {
	Objects      map[interface{}]interface{}
	Format       string
	TopLevelKeys bool
}

func NewJSON(objects map[interface{}]interface{}, format string, topLevelKeys bool) *JSON {
	return &JSON{
		Objects:      objects,
		Format:       format,
		TopLevelKeys: topLevelKeys,
	}
}

func (j *JSON) Print(writer io.Writer) {
	if j.TopLevelKeys {
		printWithKeys(j, writer)
	} else {
		printIgnoringKeys(j, writer)
	}
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

func printIgnoringKeys(j *JSON, writer io.Writer) {
	query := parseFormatQueryOrPanic(j.Format)
	for _, obj := range j.Objects {
		marshalAndPrint(obj, query, writer)
	}
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
	json.Unmarshal(data, &mapOfInterfaces)

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
			jsoncolor.Write(writer, strings.NewReader(interfaceToJSONString(v)), "  ")
		} else {
			fmt.Fprintf(writer, "%v\n", interfaceToJSONString(v))
		}
	}
}

func interfaceToJSONString(v interface{}) string {
	byteSlice, _ := json.Marshal(v)
	return string(byteSlice)
}
