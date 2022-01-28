package timers

import (
	"log"
	"time"
)

var clockOverride = ""

func NowUTC() time.Time {
	if clockOverride != "" {
		return Parse(clockOverride)
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

func Parse(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		log.Fatal(err)
	}
	return t
}
