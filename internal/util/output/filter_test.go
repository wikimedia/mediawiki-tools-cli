package output

import (
	"reflect"
	"testing"
)

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
				objects:      provideMap("test1.json"),
				outputFilter: []string{},
			},
			want: provideMap("test1.json"),
		},
		{
			name: "filter on non existent keys and value shows returns nothing",
			args: args{
				objects:      provideMap("test1.json"),
				outputFilter: []string{"xxx=yyy"},
			},
			want: emptyMap(),
		},
		{
			name: "filter single matching top level string",
			args: args{
				objects:      provideMap("test1.json"),
				outputFilter: []string{"TopLevelString=aString"},
			},
			wantTopKeys: []string{"EntryOne"},
		},
		{
			name: "filter single matching second level string",
			args: args{
				objects:      provideMap("test1.json"),
				outputFilter: []string{"TopLevelStruct.SecondLevelString=aString"},
			},
			wantTopKeys: []string{"EntryOne"},
		},
		{
			name: "filter simple *n* regex matching all top level strings",
			args: args{
				objects:      provideMap("test1.json"),
				outputFilter: []string{"TopLevelString=*String*"},
			},
			want: provideMap("test1.json"),
		},
		{
			name: "filter simple *n* regex matching one top level string",
			args: args{
				objects:      provideMap("test1.json"),
				outputFilter: []string{"TopLevelString=*aString*"},
			},
			wantTopKeys: []string{"EntryOne"},
		},
		{
			name: "filter simple n* regex matching one top level string",
			args: args{
				objects:      provideMap("test1.json"),
				outputFilter: []string{"TopLevelString=a*"},
			},
			wantTopKeys: []string{"EntryOne"},
		},
		{
			name: "filter simple *n regex matching one top level string",
			args: args{
				objects:      provideMap("test1.json"),
				outputFilter: []string{"TopLevelString=*String"},
			},
			want: provideMap("test1.json"),
		},
		{
			name: "filter all matching top level string",
			args: args{
				objects:      provideMap("test1.json"),
				outputFilter: []string{"TopLevelMatchingString=match"},
			},
			want: provideMap("test1.json"),
		},
		{
			name: "filter all matching top level int",
			args: args{
				objects:      provideMap("test1.json"),
				outputFilter: []string{"TopLevelMatchingInt=99"},
			},
			want: provideMap("test1.json"),
		},
		{
			name: "filter none matching top level int",
			args: args{
				objects:      provideMap("test1.json"),
				outputFilter: []string{"TopLevelMatchingInt=999"},
			},
			want: emptyMap(),
		},
		{
			name: "filter int with value that is not parseable",
			args: args{
				objects:      provideMap("test1.json"),
				outputFilter: []string{"TopLevelMatchingInt=xx"},
			},
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

func Test_simpleRegexStringMatch(t *testing.T) {
	type args struct {
		in      string
		matcher string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// All around matchers
		{name: "*n* no match", args: args{in: "aaa", matcher: "*b*"}, want: false},
		{name: "*n* match", args: args{in: "aaa", matcher: "*a*"}, want: true},
		// Starting matchers
		{name: "*n no match", args: args{in: "aaa", matcher: "*b"}, want: false},
		{name: "*n match", args: args{in: "aaa", matcher: "*a"}, want: true},
		// Ending matchers
		{name: "n* no match", args: args{in: "aaa", matcher: "b*"}, want: false},
		{name: "n* match", args: args{in: "aaa", matcher: "a*"}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := simpleRegexStringMatch(tt.args.in, tt.args.matcher); got != tt.want {
				t.Errorf("simpleRegexStringMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getObjectValueAtKey(t *testing.T) {
	type teststruct struct {
		astring string
		aint    int
	}
	ateststruct := teststruct{
		astring: "foo",
		aint:    99,
	}
	type args struct {
		object interface{}
		key    string
	}
	tests := []struct {
		name string
		args args
		want reflect.Value
	}{
		{
			name: "string",
			args: args{
				object: ateststruct,
				key:    "astring",
			},
			want: reflect.ValueOf("foo"),
		},
		{
			name: "int",
			args: args{
				object: ateststruct,
				key:    "aint",
			},
			want: reflect.ValueOf(99),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getValueAtKey(tt.args.object, tt.args.key); got.Type() != tt.want.Type() && got.String() != tt.want.String() {
				t.Errorf("getValueAtKey() = %v(%v), want %v(%v)", got, got.Type(), tt.want, tt.want.Type())
			}
		})
	}
}
