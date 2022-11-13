package cmdgloss

import "testing"

func TestThreePartBlock(t *testing.T) {
	type args struct {
		heading string
		details map[string]string
		footer  string
	}

	someMap := make(map[string]string)
	someMap["foo"] = "bar"

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "All provided",
			args: args{
				heading: "Hello",
				details: someMap,
				footer:  "Footer",
			},
			want: "***************************************\n" +
				"Hello\n" +
				"\n" +
				"foo: bar\n" +
				"\n" +
				"Footer\n" +
				"***************************************\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ThreePartBlock(tt.args.heading, tt.args.details, tt.args.footer); got != tt.want {
				t.Errorf("ThreePartBlock() = %v, want %v", got, tt.want)
			}
		})
	}
}
