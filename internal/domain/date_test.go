package domain

import (
	"testing"
	"time"
)

func TestDaysOverdue(t *testing.T) {
	for _, tc := range []struct {
		name, next, asOf string
		want             int
	}{
		{"today", "2026-10-21", "2026-10-21", 0},
		{"yesterday", "2026-10-20", "2026-10-21", 1},
		{"tomorrow", "2026-10-22", "2026-10-21", 0},
		{"leap day", "2024-02-28", "2024-03-01", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			next, _ := time.Parse("2006-01-02", tc.next)
			asOf, _ := time.Parse("2006-01-02", tc.asOf)
			if got := DaysOverdue(next, asOf); got != tc.want {
				t.Fatalf("days=%d want %d", got, tc.want)
			}
		})
	}
}

func TestCalendarDatePreservesLocalDay(t *testing.T) {
	zone := time.FixedZone("UTC+3", 3*3600)
	local := time.Date(2026, 10, 22, 0, 30, 0, 0, zone)
	if got := CalendarDate(local).Format("2006-01-02"); got != "2026-10-22" {
		t.Fatal(got)
	}
}
