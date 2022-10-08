package strings

import (
	"reflect"
	"testing"
)

func TestSplitToInterfaceSplitRoundtrip(t *testing.T) {
	tests := []struct {
		name string
		in   []string
	}{
		{
			"simple",
			[]string{"1", "2", "3"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitToInterfaceSplit(tt.in)
			back := InterfaceSplitToSplit(got)
			if !reflect.DeepEqual(tt.in, back) {
				t.Errorf("input = %v, SplitToInterfaceSplit() = %v, InterfaceSplitToSplit = %v", tt.in, got, back)
			}
		})
	}
}
