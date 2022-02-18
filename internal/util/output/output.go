package output

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"text/template"

	"github.com/sirupsen/logrus"
)

func OutputModern(objects []interface{}, outputFormat string, outputFilter []string) {

	// Filter
	if outputFilter != nil {
		getField := func(reflectedValue reflect.Value, filter string) string {
			fields := strings.Split(filter, ".")
			for _, field := range fields {
				reflectedValue = reflect.Indirect(reflectedValue).FieldByName(field)
			}
			return string(reflectedValue.String())
		}
		for _, filter := range outputFilter {
			filterSplit := strings.Split(filter, "=")
			filterKey := filterSplit[0]
			filterValue := filterSplit[1]
			for i := len(objects) - 1; i >= 0; i-- {
				change := objects[i]
				reflectedChange := reflect.ValueOf(change)
				fieldValue := getField(reflectedChange, filterKey)
				keep := true
				if filterValue[0:1] == "*" && filterValue[len(filterValue)-1:] == "*" {
					lookFor := filterValue[1 : len(filterValue)-1]
					if !strings.Contains(fieldValue, lookFor) {
						logrus.Tracef("Filtering out as '%s' not in '%s'", lookFor, fieldValue)
						keep = false
					}
				} else if fieldValue != filterValue {
					logrus.Tracef("Filtering out as '%s' doesn't match '%s'", filterValue, fieldValue)
					keep = false
				}

				if !keep {
					objects = append(objects[:i], objects[i+1:]...)
				}
			}
		}
	}

	// Output using format
	if outputFormat != "" {
		tmpl := template.Must(template.
			New("").
			Funcs(map[string]interface{}{
				"json": func(v interface{}) (string, error) {
					b, err := json.MarshalIndent(v, "", "  ")
					if err != nil {
						return "", err
					}
					return string(b), nil
				},
			}).
			Parse(outputFormat))
		for _, change := range objects {
			_ = tmpl.Execute(os.Stdout, change)
			fmt.Println()
		}
		return
	}

}
