package attendance

import (
	"math"
	"testing"
	"time"
)

func basePolicy() Policy {
	return Policy{Version: 2, Timezone: "Asia/Phnom_Penh", Start: "08:00", End: "17:00", Grace: 10, Expected: 480, EarlyWindow: 120, Overtime: true, Workdays: []int{1, 2, 3, 4, 5}}
}
func TestGeofence(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name               string
		lat, lon, accuracy float64
		age                time.Duration
		want               bool
	}{{"inside", 11.5564, 104.9282, 10, 0, true}, {"outside", 11.6, 104.95, 10, 0, false}, {"poor accuracy", 11.5564, 104.9282, 90, 0, false}, {"stale", 11.5564, 104.9282, 10, 2 * time.Minute, false}, {"future", 11.5564, 104.9282, 10, -time.Minute, false}, {"invalid", 91, 104, 10, 0, false}, {"nan", math.NaN(), 104, 10, 0, false}, {"no accuracy", 11.5564, 104.9282, 0, 0, false}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, e := VerifyLocation(now, now.Add(-tc.age), tc.lat, tc.lon, tc.accuracy, 11.5564, 104.9282, 150, 75)
			if (e == nil) != tc.want {
				t.Fatalf("error=%v", e)
			}
		})
	}
	if d := Distance(0, 0, 0, 1); math.Abs(d-111194.926) > 1 {
		t.Fatalf("distance %v", d)
	}
	d := Distance(11.5564, 104.9282, 11.557, 104.9282)
	if _, e := VerifyLocation(now, now, 11.557, 104.9282, 5, 11.5564, 104.9282, d, 75); e != nil {
		t.Fatal("exact boundary rejected")
	}
}
func TestTransitions(t *testing.T) {
	tests := []struct {
		state, action, want string
		breaks              bool
	}{{"", "check_in", "checked_in", false}, {"", "check_out", "", true}, {"checked_in", "check_in", "", true}, {"checked_in", "start_break", "on_break", true}, {"checked_in", "start_break", "", false}, {"on_break", "check_out", "", true}, {"on_break", "end_break", "checked_in", true}, {"checked_in", "check_out", "checked_out", true}, {"checked_out", "check_out", "", true}}
	for _, tc := range tests {
		got, e := Transition(tc.state, tc.action, tc.breaks)
		if got != tc.want || (e != nil) != (tc.want == "") {
			t.Fatalf("%+v got=%s err=%v", tc, got, e)
		}
	}
}
func TestGraceAndCalculations(t *testing.T) {
	p := basePolicy()
	start, end, _ := Bounds("2026-10-13", p)
	for _, tc := range []struct {
		delay, late int
		status      string
	}{{0, 0, "on_time"}, {7, 0, "within_grace"}, {10, 0, "within_grace"}, {15, 5, "late"}} {
		c, e := Calculate("2026-10-13", start.Add(time.Duration(tc.delay)*time.Minute), nil, 0, p)
		if e != nil || c.Late != tc.late || c.ArrivalDelay != tc.delay || c.Status != tc.status {
			t.Fatalf("%+v %v", c, e)
		}
	}
	c, _ := Calculate("2026-10-13", start, &end, 3600, p)
	if c.Worked != 480 || c.Overtime != 0 {
		t.Fatalf("%+v", c)
	}
	o := end.Add(time.Hour)
	c, _ = Calculate("2026-10-13", start, &o, 3600, p)
	if c.Worked != 540 || c.Overtime != 60 {
		t.Fatalf("%+v", c)
	}
	o = end.Add(-time.Hour)
	c, _ = Calculate("2026-10-13", start, &o, 3600, p)
	if c.Early != 60 || c.Worked != 420 {
		t.Fatalf("%+v", c)
	}
	p.UnpaidBreak = 30
	c, _ = Calculate("2026-10-13", start, &end, 3600, p)
	if c.Worked != 450 {
		t.Fatalf("%+v", c)
	}
}
func TestOvernightAndDST(t *testing.T) {
	p := basePolicy()
	p.Start = "22:00"
	p.End = "06:00"
	start, end, e := Bounds("2026-10-13", p)
	if e != nil || end.Sub(start) != 8*time.Hour {
		t.Fatal(start, end, e)
	}
	date, e := WorkDate(start.Add(4*time.Hour), p)
	if e != nil || date != "2026-10-13" {
		t.Fatal(date, e)
	}
	date, e = WorkDate(start.Add(-time.Hour), p)
	if e != nil || date != "2026-10-13" {
		t.Fatal(date, e)
	}
	p.Timezone = "America/New_York"
	p.Start = "22:00"
	p.End = "06:00"
	start, end, e = Bounds("2026-10-31", p)
	if e != nil || end.Sub(start) != 9*time.Hour {
		t.Fatal("DST elapsed time", start, end, e)
	}
	p.Start = "08:00"
	p.End = "17:00"
	if Scheduled("2026-10-11", p) {
		t.Fatal("Sunday incorrectly scheduled")
	}
	p.Workdays = []int{0}
	if !Scheduled("2026-10-11", p) {
		t.Fatal("custom Sunday ignored")
	}
}
func TestPolicySnapshot(t *testing.T) {
	p := basePolicy()
	snapshot := p
	p.Grace = 0
	start, _, _ := Bounds("2026-10-13", snapshot)
	c, _ := Calculate("2026-10-13", start.Add(7*time.Minute), nil, 0, snapshot)
	if c.Late != 0 || snapshot.Version != 2 {
		t.Fatal("historical policy changed")
	}
}
