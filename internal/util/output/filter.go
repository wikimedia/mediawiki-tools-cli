package output

import (
	"reflect"
	"strings"

	"github.com/sirupsen/logrus"
)

func Filter(objects []interface{}, outputFilter []string) []interface{} {
	if len(outputFilter) == 0 {
		return objects
	}
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
	return objects
}
