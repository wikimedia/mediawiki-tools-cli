package output

import (
	"reflect"
	"strconv"
	"strings"
)

func Filter(objects map[interface{}]interface{}, filterList []string) map[interface{}]interface{} {
	if len(filterList) == 0 {
		return objects
	}
	for _, filter := range filterList {
		filterKey, filterValue := filterKeyAndValue(filter)
		for i, object := range objects {
			reflectValue := getValueAtKey(object, filterKey)
			keep := reflectValue.IsValid()
			if keep {
				switch reflectValue.Type().String() {
				case "string":
					stringValue := reflectValue.String()
					if isSimpleRegexStringMatcher(filterValue) {
						if !simpleRegexStringMatch(stringValue, filterValue) {
							keep = false
						}
					} else if stringValue != filterValue {
						keep = false
					}
				case "int":
					intFilter, _ := strconv.ParseInt(filterValue, 0, 64)
					if reflectValue.Int() != intFilter {
						keep = false
					}
				}
			}

			if !keep {
				delete(objects, i)
			}
		}
	}
	return objects
}

func getValueAtKey(object interface{}, key string) reflect.Value {
	reflectedValue := reflect.ValueOf(object)
	fields := strings.Split(key, ".")
	// Look down through each key split from . recursively
	for _, field := range fields {
		reflectedValue = reflect.Indirect(reflectedValue).FieldByName(field)
	}
	return reflectedValue
}

func filterKeyAndValue(userInput string) (key string, value string) {
	split := strings.Split(userInput, "=")
	return split[0], split[1]
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
	panic("simpleRegexStringMatch called wity invalid matcher")
}
