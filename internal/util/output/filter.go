package output

import (
	"reflect"
	"strings"

	"github.com/sirupsen/logrus"
)

func Filter(objects map[interface{}]interface{}, outputFilter []string) map[interface{}]interface{} {
	if len(outputFilter) == 0 {
		return objects
	}
	getField := func(reflectedValue reflect.Value, filter string) string {
		fields := strings.Split(filter, ".")
		for _, field := range fields {
			reflectedValue = reflect.Indirect(reflectedValue).FieldByName(field)
		}
		return reflectedValue.String()
	}
	for _, filter := range outputFilter {
		filterSplit := strings.Split(filter, "=")
		filterKey := filterSplit[0]
		filterValue := filterSplit[1]
		for i, object := range objects {
			reflectedValueOfObject := reflect.ValueOf(object)
			fieldValue := getField(reflectedValueOfObject, filterKey)
			keep := true

			if isSimpleRegexStringMatcher(filterValue) {
				if !simpleRegexStringMatch(fieldValue, filterValue) {
					keep = false
				}
			} else if fieldValue != filterValue {
				logrus.Tracef("Filtering out as '%s' doesn't match '%s'", filterValue, fieldValue)
				keep = false
			}

			if !keep {
				delete(objects, i)
			}
		}
	}
	return objects
}

func isSimpleRegexStringMatcher(matcher string) bool {
	return matcher[0:1] == "*" || matcher[len(matcher)-1:] == "*"
}

func simpleRegexStringMatch(in string, matcher string) bool {
	if matcher[0:1] == "*" && matcher[len(matcher)-1:] == "*" {
		// Both ends
		lookFor := matcher[1 : len(matcher)-1]
		return strings.Contains(in, lookFor)
	} else if matcher[0:1] == "*" {
		// Start only
		lookFor := matcher[1:]
		return strings.LastIndex(in, lookFor) == len(in)-len(lookFor)
	} else if matcher[len(matcher)-1:] == "*" {
		// End only
		lookFor := matcher[0 : len(matcher)-1]
		return strings.Index(in, lookFor) == 0
	}
	return false
	panic("simpleRegexStringMatch called wity invalid matcher")
}
