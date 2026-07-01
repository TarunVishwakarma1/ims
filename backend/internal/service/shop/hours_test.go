package shop

import (
	"testing"
	"time"
)

func atIST(t *testing.T, hhmm string) time.Time {
	t.Helper()
	loc, _ := time.LoadLocation("Asia/Kolkata")
	tm, err := time.ParseInLocation("15:04", hhmm, loc)
	if err != nil {
		t.Fatalf("parse %q: %v", hhmm, err)
	}
	// anchor on an arbitrary date; only the clock matters
	return time.Date(2026, 7, 1, tm.Hour(), tm.Minute(), 0, 0, loc)
}

func ptr(s string) *string { return &s }

func TestShopOpen(t *testing.T) {
	cases := []struct {
		name             string
		opens, closes    *string
		nowHHMM          string
		want             bool
	}{
		{"nil = always open", nil, nil, "03:00", true},
		{"open nil only", nil, ptr("18:00"), "03:00", true},
		{"within day window", ptr("09:00"), ptr("21:00"), "12:30", true},
		{"before open", ptr("09:00"), ptr("21:00"), "08:59", false},
		{"at close is closed", ptr("09:00"), ptr("21:00"), "21:00", false},
		{"after close", ptr("09:00"), ptr("21:00"), "22:00", false},
		{"overnight — late night open", ptr("18:00"), ptr("02:00"), "23:30", true},
		{"overnight — early morning open", ptr("18:00"), ptr("02:00"), "01:00", true},
		{"overnight — daytime closed", ptr("18:00"), ptr("02:00"), "12:00", false},
		{"equal times = 24h", ptr("00:00"), ptr("00:00"), "12:00", true},
		{"malformed fails open", ptr("nope"), ptr("21:00"), "23:00", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ShopOpen(c.opens, c.closes, atIST(t, c.nowHHMM)); got != c.want {
				t.Fatalf("ShopOpen=%v want %v", got, c.want)
			}
		})
	}
}
