package strings

import "fmt"

func SplitToInterfaceSplit(in []string) []interface{} {
	out := make([]interface{}, len(in))
	for i, v := range in {
		out[i] = v
	}
	return out
}

func InterfaceSplitToSplit(in []interface{}) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = fmt.Sprintf("%s", v)
	}
	return out
}
