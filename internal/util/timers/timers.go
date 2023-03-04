package timers

import (
	"time"

	"github.com/sirupsen/logrus"
)

var clockOverride = ""

func NowUTC() time.Time {
	if clockOverride != "" {
		parsedOverride, err := Parse(clockOverride)
		if err != nil {
			logrus.Fatal(err)
		}
		return parsedOverride
	}
	return time.Now().UTC()
}

func HoursAgo(hours int) time.Time {
	return NowUTC().Add(-time.Duration(hours) * time.Hour)
}

func IsHoursAgo(t time.Time, hours float64) bool {
	return NowUTC().Sub(t).Hours() >= hours
}

func String(t time.Time) string {
	return t.Format(time.RFC3339)
}

func Parse(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}
