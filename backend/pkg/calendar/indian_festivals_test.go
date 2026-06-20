package calendar_test

import (
	"testing"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/pkg/calendar"
)

func TestFestivals_Count(t *testing.T) {
	if len(calendar.Festivals) != 12 {
		t.Fatalf("expected 12 festivals, got %d", len(calendar.Festivals))
	}
}

func TestFestivals_FixedDates(t *testing.T) {
	cases := map[string]struct{ m, d int }{
		"republic_day":     {1, 26},
		"independence_day": {8, 15},
		"christmas":        {12, 25},
		"new_year":         {1, 1},
		"pongal":           {1, 14},
	}
	byKey := map[string]calendar.Festival{}
	for _, f := range calendar.Festivals { byKey[f.Key] = f }
	for key, want := range cases {
		f, ok := byKey[key]
		if !ok { t.Fatalf("missing %s", key); continue }
		d := f.Date(2026)
		if d.Month() != time.Month(want.m) || d.Day() != want.d {
			t.Fatalf("%s 2026: want %02d-%02d, got %s", key, want.m, want.d, d.Format("2006-01-02"))
		}
		if d.Location().String() != "IST" {
			t.Fatalf("%s: not IST: %s", key, d.Location())
		}
	}
}

func TestFestivals_LunarLookup_Diwali2026(t *testing.T) {
	var diwali calendar.Festival
	for _, f := range calendar.Festivals {
		if f.Key == "diwali" { diwali = f }
	}
	if diwali.Key == "" { t.Fatal("diwali not found") }
	d := diwali.Date(2026)
	// Diwali 2026: Nov 8 (verify the table)
	if d.Month() != time.November || d.Day() != 8 {
		t.Fatalf("diwali 2026 want 2026-11-08, got %s", d.Format("2006-01-02"))
	}
}
