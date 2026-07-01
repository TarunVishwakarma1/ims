package shop

import "time"

// istLoc is the shop wall clock. Business hours are interpreted in IST.
var istLoc = func() *time.Location {
	l, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		return time.FixedZone("IST", 5*3600+1800) // +05:30 fallback
	}
	return l
}()

// ShopOpen reports whether a shop is open at instant `now`, given optional
// "HH:MM" open/close times (IST). A nil on either side means always open.
// A close time earlier than the open time wraps past midnight (e.g. 18:00–02:00).
// Malformed times fail open (a shop is never hidden by bad data).
func ShopOpen(opensAt, closesAt *string, now time.Time) bool {
	if opensAt == nil || closesAt == nil {
		return true
	}
	openM, ok1 := parseHHMM(*opensAt)
	closeM, ok2 := parseHHMM(*closesAt)
	if !ok1 || !ok2 || openM == closeM {
		return true
	}
	t := now.In(istLoc)
	cur := t.Hour()*60 + t.Minute()
	if openM < closeM {
		return cur >= openM && cur < closeM
	}
	return cur >= openM || cur < closeM // wraps midnight
}

func parseHHMM(s string) (int, bool) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, false
	}
	return t.Hour()*60 + t.Minute(), true
}
