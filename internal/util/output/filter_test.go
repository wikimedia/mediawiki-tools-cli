package output

import (
	"encoding/json"
	"io/ioutil"
	"reflect"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

func filterableMapFromFile(file string) map[interface{}]interface{} {
	type ComplexObject struct {
		TopLevelString         string `json:"TopLevelString"`
		TopLevelMatchingInt    int    `json:"TopLevelMatchingInt"`
		TopLevelMatchingString string `json:"TopLevelMatchingString"`
		TopLevelStruct         struct {
			SecondLevelString     string `json:"SecondLevelString"`
			SecondLevelStructList []struct {
				ThirdLevelString string   `json:"ThirdLevelString"`
				ThirdLevelInt    int      `json:"ThirdLevelInt"`
				ThirdLevelList   []string `json:"ThirdLevelList"`
			} `json:"SecondLevelStructList"`
		} `json:"TopLevelStruct"`
	}
	type ComplexObjects map[string]ComplexObject

	// Get a filterable map from the 1 test file
	filterable := ComplexObjects{}
	bytes, err := ioutil.ReadFile(file)
	if err != nil {
		logrus.Fatal(err)
	}
	if err = json.NewDecoder(strings.NewReader(string(bytes))).Decode(&filterable); err != nil {
		logrus.Fatal(err)
	}
	filterableMap := make(map[interface{}]interface{}, len(filterable))
	for key, value := range filterable {
		filterableMap[key] = value
	}

	return filterableMap
}

func emptyMap() map[interface{}]interface{} {
	return make(map[interface{}]interface{})
}

func TestFilter(t *testing.T) {

	type args struct {
		objects      map[interface{}]interface{}
		outputFilter []string
	}
	tests := []struct {
		name        string
		file        string
		args        args
		want        map[interface{}]interface{}
		wantTopKeys []string
	}{
		{
			name: "no filters means no change",
			args: args{
				objects:      filterableMapFromFile("filter_test_1.json"),
				outputFilter: []string{},
			},
			want: filterableMapFromFile("filter_test_1.json"),
		},
		{
			name: "filter on non existant keys and value shows returns nothing",
			args: args{
				objects:      filterableMapFromFile("filter_test_1.json"),
				outputFilter: []string{"xxx=yyy"},
			},
			want: emptyMap(),
		},
		{
			name: "filter single matching top level string",
			args: args{
				objects:      filterableMapFromFile("filter_test_1.json"),
				outputFilter: []string{"TopLevelString=aString"},
			},
			wantTopKeys: []string{"EntryOne"},
		},
		{
			name: "filter single matching second level string",
			args: args{
				objects:      filterableMapFromFile("filter_test_1.json"),
				outputFilter: []string{"TopLevelStruct.SecondLevelString=aString"},
			},
			wantTopKeys: []string{"EntryOne"},
		},
		{
			name: "filter simple *n* regex matching all top level strings",
			args: args{
				objects:      filterableMapFromFile("filter_test_1.json"),
				outputFilter: []string{"TopLevelString=*String*"},
			},
			want: filterableMapFromFile("filter_test_1.json"),
		},
		{
			name: "filter simple *n* regex matching one top level string",
			args: args{
				objects:      filterableMapFromFile("filter_test_1.json"),
				outputFilter: []string{"TopLevelString=*aString*"},
			},
			wantTopKeys: []string{"EntryOne"},
		},
		// TODO FIXME on *n* regex is currently supported
		{
			name: "TODO FIXME filter simple n* regex matching one top level string",
			args: args{
				objects:      filterableMapFromFile("filter_test_1.json"),
				outputFilter: []string{"TopLevelString=a*"},
			},
			// Should be this
			// wantTopKeys: []string{"EntryOne"},
			want: emptyMap(),
		},
		// TODO FIXME on *n* regex is currently supported
		{
			name: "TODO FIXME filter simple *n regex matching one top level string",
			args: args{
				objects:      filterableMapFromFile("filter_test_1.json"),
				outputFilter: []string{"TopLevelString=*String"},
			},
			// Should be this
			// want: filterableMapFromFile("filter_test_1.json"),
			want: emptyMap(),
		},
		{
			name: "filter all matching top level string",
			args: args{
				objects:      filterableMapFromFile("filter_test_1.json"),
				outputFilter: []string{"TopLevelMatchingString=match"},
			},
			want: filterableMapFromFile("filter_test_1.json"),
		},
		// TODO FIXME Filtering on ints doesn't currently work
		{
			name: "TODO FIXME filter all matching top level int",
			args: args{
				objects:      filterableMapFromFile("filter_test_1.json"),
				outputFilter: []string{"TopLevelMatchingInt=99"},
			},
			// Should be this
			// want: filterableMapFromFile("filter_test_1.json"),
			want: emptyMap(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Filter(tt.args.objects, tt.args.outputFilter)
			if tt.want != nil && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Filter() = %v, want %v", got, tt.want)
			}
			if tt.wantTopKeys != nil {
				if len(got) != len(tt.wantTopKeys) {
					t.Errorf("Filter() returned %v keys, wanted %v keys", len(got), len(tt.wantTopKeys))
				}
				for _, wantedKey := range tt.wantTopKeys {
					if got[wantedKey] == nil {
						t.Errorf("Filter() = %v, wanted a number of keys %v, not found %v", got, tt.wantTopKeys, wantedKey)
					}
				}
			}
		})
	}
}
