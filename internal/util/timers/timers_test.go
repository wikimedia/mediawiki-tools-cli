package timers

import (
	"reflect"
	"testing"
	"time"
)

func TestNowUTC(t *testing.T) {
	t.Run("Returns a time", func(t *testing.T) {
		got := NowUTC()
		diff := time.Now().UTC().Sub(got)
		if diff.Milliseconds() > 1000 {
			t.Errorf("NowUTC() seems to have got a non recent got = %v, diff %v", got, diff.Milliseconds())
		}
	})
}

func TestNowUTC_withOverride(t *testing.T) {
	clockOverride = "2020-01-01T10:00:00Z"
	overrideTime, _ := time.Parse(time.RFC3339, "2020-01-01T10:00:00Z")

	tests := []struct {
		name string
		want time.Time
	}{
		{
			name: "returns the current time in UTC",
			want: overrideTime,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NowUTC(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NowUTC() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHoursAgo(t *testing.T) {
	clockOverride = "2020-01-01T10:00:00Z"
	twoHoursBefore, _ := time.Parse(time.RFC3339, "2020-01-01T08:00:00Z")

	type args struct {
		hours int
	}
	tests := []struct {
		name string
		args args
		want time.Time
	}{
		{
			name: "2 hours before",
			args: args{
				hours: 2,
			},
			want: twoHoursBefore,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HoursAgo(tt.args.hours); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("HoursAgo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsHoursAgo(t *testing.T) {
	clockOverride = "2020-01-01T10:00:00Z"
	twoHoursBefore, _ := time.Parse(time.RFC3339, "2020-01-01T08:00:00Z")

	type args struct {
		t     time.Time
		hours float64
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "2 hours before is over 3 hours ago",
			args: args{
				t:     twoHoursBefore,
				hours: 3,
			},
			want: false,
		},
		{
			name: "2 hours before is exactly 2 hours ago",
			args: args{
				t:     twoHoursBefore,
				hours: 2,
			},
			want: true,
		},
		{
			name: "2 hours before is NOT 1 hour ago",
			args: args{
				t:     twoHoursBefore,
				hours: 1,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsHoursAgo(tt.args.t, tt.args.hours); got != tt.want {
				t.Errorf("IsHoursAgo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestString(t *testing.T) {
	type args struct {
		t time.Time
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "some times",
			args: args{
				t: time.Date(2022, 10, 4, 1, 2, 3, 4, time.UTC),
			},
			want: "2022-10-04T01:02:03Z",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := String(tt.args.t); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}
