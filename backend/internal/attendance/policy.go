package attendance

import (
	"errors"
	"math"
	"time"
)

type Policy struct {
	Version     int    `json:"version"`
	Timezone    string `json:"timezone"`
	ShiftName   string `json:"shift_name"`
	Start       string `json:"start"`
	End         string `json:"end"`
	Grace       int    `json:"grace"`
	Expected    int    `json:"expected"`
	UnpaidBreak int    `json:"unpaid_break"`
	EarlyWindow int    `json:"early_window"`
	Overtime    bool   `json:"overtime"`
	Breaks      bool   `json:"breaks"`
	Workdays    []int  `json:"workdays"`
}
type Calculation struct {
	Worked       int    `json:"worked"`
	Late         int    `json:"late"`
	ArrivalDelay int    `json:"arrival_delay"`
	Early        int    `json:"early"`
	Overtime     int    `json:"overtime"`
	Status       string `json:"status"`
}

func Distance(lat1, lon1, lat2, lon2 float64) float64 {
	r := math.Pi / 180
	a := math.Pow(math.Sin((lat2-lat1)*r/2), 2) + math.Cos(lat1*r)*math.Cos(lat2*r)*math.Pow(math.Sin((lon2-lon1)*r/2), 2)
	return 6371000 * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(math.Max(0, 1-a)))
}
func VerifyLocation(now, measured time.Time, lat, lon, accuracy, workLat, workLon, radius, maxAccuracy float64) (float64, error) {
	for _, v := range []float64{lat, lon, accuracy, workLat, workLon, radius, maxAccuracy} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, errors.New("invalid location")
		}
	}
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 || accuracy <= 0 {
		return 0, errors.New("invalid location")
	}
	if now.Sub(measured) > 90*time.Second || measured.After(now.Add(10*time.Second)) {
		return 0, errors.New("location reading is stale; request a fresh position")
	}
	if accuracy > maxAccuracy {
		return 0, errors.New("low location accuracy; move outdoors and retry")
	}
	d := Distance(lat, lon, workLat, workLon)
	if d > radius {
		return d, errors.New("outside the workplace boundary")
	}
	return d, nil
}
func Transition(state, action string, breaks bool) (string, error) {
	switch {
	case state == "" && action == "check_in":
		return "checked_in", nil
	case state == "checked_in" && action == "check_out":
		return "checked_out", nil
	case breaks && state == "checked_in" && action == "start_break":
		return "on_break", nil
	case breaks && state == "on_break" && action == "end_break":
		return "checked_in", nil
	}
	return "", errors.New("invalid attendance transition")
}
func Bounds(date string, p Policy) (time.Time, time.Time, error) {
	loc, e := time.LoadLocation(p.Timezone)
	if e != nil {
		return time.Time{}, time.Time{}, e
	}
	s, e := time.ParseInLocation("2006-01-02 15:04", date+" "+p.Start, loc)
	if e != nil {
		return s, s, e
	}
	end, e := time.ParseInLocation("2006-01-02 15:04", date+" "+p.End, loc)
	if e != nil {
		return s, end, e
	}
	if !end.After(s) {
		end = end.AddDate(0, 0, 1)
	}
	return s, end, nil
}
func WorkDate(now time.Time, p Policy) (string, error) {
	loc, e := time.LoadLocation(p.Timezone)
	if e != nil {
		return "", e
	}
	local := now.In(loc)
	date := local.Format("2006-01-02")
	s, end, e := Bounds(date, p)
	if e != nil {
		return "", e
	}
	if end.Day() != s.Day() && local.Before(s.Add(-time.Duration(p.EarlyWindow)*time.Minute)) {
		date = local.AddDate(0, 0, -1).Format("2006-01-02")
	}
	return date, nil
}
func Calculate(date string, in time.Time, out *time.Time, breakSeconds int64, p Policy) (Calculation, error) {
	s, end, e := Bounds(date, p)
	if e != nil {
		return Calculation{}, e
	}
	c := Calculation{Status: "on_time"}
	c.ArrivalDelay = max(0, int(in.Sub(s).Minutes()))
	c.Late = max(0, c.ArrivalDelay-p.Grace)
	if c.Late > 0 {
		c.Status = "late"
	} else if in.After(s) {
		c.Status = "within_grace"
	}
	if out == nil {
		return c, nil
	}
	c.Worked = max(0, int(out.Sub(in).Seconds()-float64(breakSeconds))/60-p.UnpaidBreak)
	c.Early = max(0, int(end.Sub(*out).Minutes()))
	if p.Overtime {
		c.Overtime = max(0, c.Worked-p.Expected)
	}
	return c, nil
}
func Scheduled(date string, p Policy) bool {
	s, _, e := Bounds(date, p)
	if e != nil {
		return false
	}
	for _, d := range p.Workdays {
		if d == int(s.Weekday()) {
			return true
		}
	}
	return false
}
