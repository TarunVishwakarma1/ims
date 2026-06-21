package calendar

import (
	"time"

	"go.uber.org/zap"
)

var IST = time.FixedZone("IST", 5*3600+1800) // UTC+05:30

type Festival struct {
	Key  string
	Name string
	Date func(year int) time.Time
}

func fixedDate(month, day int) func(int) time.Time {
	return func(year int) time.Time {
		return time.Date(year, time.Month(month), day, 0, 0, 0, 0, IST)
	}
}

func lookupDate(table map[int]time.Time) func(int) time.Time {
	return func(year int) time.Time {
		if d, ok := table[year]; ok { return d }
		// fallback for missing years: Jan 1 (admin must manually update table; spec promises 2026-2030)
		zap.L().Warn("calendar: lunar festival year not in table, falling back to Jan 1",
			zap.Int("year", year))
		return time.Date(year, 1, 1, 0, 0, 0, 0, IST)
	}
}

func civilIST(y, m, d int) time.Time {
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, IST)
}

var diwaliDates = map[int]time.Time{
	2026: civilIST(2026, 11, 8),
	2027: civilIST(2027, 10, 28),
	2028: civilIST(2028, 11, 14),
	2029: civilIST(2029, 11, 5),
	2030: civilIST(2030, 10, 26),
}

var holiDates = map[int]time.Time{
	2026: civilIST(2026, 3, 3),
	2027: civilIST(2027, 3, 22),
	2028: civilIST(2028, 3, 11),
	2029: civilIST(2029, 3, 1),
	2030: civilIST(2030, 3, 20),
}

var eidDates = map[int]time.Time{
	2026: civilIST(2026, 3, 20),
	2027: civilIST(2027, 3, 9),
	2028: civilIST(2028, 2, 26),
	2029: civilIST(2029, 2, 14),
	2030: civilIST(2030, 2, 4),
}

var rakhiDates = map[int]time.Time{
	2026: civilIST(2026, 8, 28),
	2027: civilIST(2027, 8, 17),
	2028: civilIST(2028, 9, 4),
	2029: civilIST(2029, 8, 24),
	2030: civilIST(2030, 8, 13),
}

var ganeshDates = map[int]time.Time{
	2026: civilIST(2026, 9, 14),
	2027: civilIST(2027, 9, 4),
	2028: civilIST(2028, 8, 23),
	2029: civilIST(2029, 9, 11),
	2030: civilIST(2030, 8, 31),
}

var dussehraDates = map[int]time.Time{
	2026: civilIST(2026, 10, 20),
	2027: civilIST(2027, 10, 9),
	2028: civilIST(2028, 9, 27),
	2029: civilIST(2029, 10, 16),
	2030: civilIST(2030, 10, 6),
}

var onamDates = map[int]time.Time{
	2026: civilIST(2026, 8, 26),
	2027: civilIST(2027, 9, 14),
	2028: civilIST(2028, 9, 2),
	2029: civilIST(2029, 8, 23),
	2030: civilIST(2030, 9, 10),
}

var Festivals = []Festival{
	{Key: "diwali",            Name: "Diwali",            Date: lookupDate(diwaliDates)},
	{Key: "holi",              Name: "Holi",              Date: lookupDate(holiDates)},
	{Key: "eid_ul_fitr",       Name: "Eid-ul-Fitr",       Date: lookupDate(eidDates)},
	{Key: "rakhi",             Name: "Raksha Bandhan",    Date: lookupDate(rakhiDates)},
	{Key: "ganesh_chaturthi",  Name: "Ganesh Chaturthi",  Date: lookupDate(ganeshDates)},
	{Key: "dussehra",          Name: "Dussehra",          Date: lookupDate(dussehraDates)},
	{Key: "onam",              Name: "Onam",              Date: lookupDate(onamDates)},
	{Key: "pongal",            Name: "Pongal",            Date: fixedDate(1, 14)},
	{Key: "republic_day",      Name: "Republic Day",      Date: fixedDate(1, 26)},
	{Key: "independence_day",  Name: "Independence Day",  Date: fixedDate(8, 15)},
	{Key: "christmas",         Name: "Christmas",         Date: fixedDate(12, 25)},
	{Key: "new_year",          Name: "New Year",          Date: fixedDate(1, 1)},
}
