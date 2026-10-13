package server

import (
	"encoding/csv"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

func safeCell(v string) string {
	trim := strings.TrimLeft(v, " \t\r\n")
	if strings.HasPrefix(trim, "=") || strings.HasPrefix(trim, "+") || strings.HasPrefix(trim, "-") || strings.HasPrefix(trim, "@") || strings.HasPrefix(v, "\t") || strings.HasPrefix(v, "\r") {
		return "'" + v
	}
	return v
}
func (s *Server) report(c *gin.Context) {
	if !require(c, "reports") {
		return
	}
	from, to, ok := dateRange(c)
	if !ok {
		return
	}
	i := who(c)
	rows, e := s.DB.Query(c.Request.Context(), `SELECT m.employee_code,u.name,a.work_date::text,a.check_in,a.check_out,a.worked_minutes,a.late_minutes,a.early_minutes,a.overtime_minutes,a.status,a.adjusted`+visibleSessions+` AND a.work_date BETWEEN $5::date AND $6::date ORDER BY a.work_date,u.name LIMIT 50001`, i.CompanyID, true, departmentScope(i), i.MembershipID, from, to)
	if e != nil {
		problem(c, e)
		return
	}
	defer rows.Close()
	data := [][]string{{"Devaraj Time attendance report", safeCell(i.Company), "From", from, "To", to}, {"Generated (UTC)", time.Now().UTC().Format(time.RFC3339), "Timezone", i.Timezone}, {"Code", "Employee", "Work date", "Effective check-in", "Effective check-out", "Worked minutes", "Late minutes beyond grace", "Early minutes", "Overtime minutes", "Status", "Adjusted"}}
	loc, _ := time.LoadLocation(i.Timezone)
	for rows.Next() {
		var code, name, date, status string
		var in time.Time
		var out *time.Time
		var worked, late, early, overtime int
		var adjusted bool
		if e = rows.Scan(&code, &name, &date, &in, &out, &worked, &late, &early, &overtime, &status, &adjusted); e != nil {
			problem(c, e)
			return
		}
		outText := ""
		if out != nil {
			outText = out.In(loc).Format(time.RFC3339)
		}
		data = append(data, []string{safeCell(code), safeCell(name), date, in.In(loc).Format(time.RFC3339), outText, fmt.Sprint(worked), fmt.Sprint(late), fmt.Sprint(early), fmt.Sprint(overtime), status, fmt.Sprint(adjusted)})
	}
	if e = rows.Err(); e != nil {
		problem(c, e)
		return
	}
	if len(data) > 50003 {
		fail(c, 422, "Report is too large; choose a smaller date range")
		return
	}
	if c.Query("format") == "xlsx" {
		f := excelize.NewFile()
		defer f.Close()
		_ = f.SetSheetName("Sheet1", "Attendance")
		for n, row := range data {
			for col, v := range row {
				cell, _ := excelize.CoordinatesToCellName(col+1, n+1)
				_ = f.SetCellStr("Attendance", cell, v)
			}
		}
		_ = f.SetColWidth("Attendance", "A", "K", 24)
		b, err := f.WriteToBuffer()
		if err != nil {
			problem(c, err)
			return
		}
		c.Header("Content-Disposition", "attachment; filename=devaraj-attendance.xlsx")
		c.Data(200, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", b.Bytes())
		return
	}
	c.Header("Content-Disposition", "attachment; filename=devaraj-attendance.csv")
	c.Header("Content-Type", "text/csv; charset=utf-8")
	w := csv.NewWriter(c.Writer)
	w.WriteAll(data)
}
