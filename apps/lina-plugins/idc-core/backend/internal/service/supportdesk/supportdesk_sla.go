package supportdesk

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
)

type slaSnapshot struct {
	RuleNo              string  `orm:"rule_no"`
	RuleVersion         int64   `orm:"rule_version"`
	CalendarNo          string  `orm:"calendar_no"`
	CalendarVersion     int64   `orm:"calendar_version"`
	CalendarMode        string  `orm:"calendar_mode"`
	TimezoneName        *string `orm:"timezone_name"`
	Workdays            []int16 `orm:"workdays"`
	WorkStartMinute     *int    `orm:"work_start_minute"`
	WorkEndMinute       *int    `orm:"work_end_minute"`
	FirstResponseMinute *int    `orm:"first_response_minutes"`
	ResolutionMinute    *int    `orm:"resolution_minutes"`
	NearDueMinutes      []int   `orm:"near_due_minutes"`
	HolidayDates        []time.Time
}

type slaSnapshotRow struct {
	RuleNo              string  `orm:"rule_no"`
	RuleVersion         int64   `orm:"rule_version"`
	CalendarNo          string  `orm:"calendar_no"`
	CalendarVersion     int64   `orm:"calendar_version"`
	CalendarMode        string  `orm:"calendar_mode"`
	TimezoneName        *string `orm:"timezone_name"`
	WorkdaysJSON        string  `orm:"workdays_json"`
	WorkStartMinute     *int    `orm:"work_start_minute"`
	WorkEndMinute       *int    `orm:"work_end_minute"`
	FirstResponseMinute *int    `orm:"first_response_minutes"`
	ResolutionMinute    *int    `orm:"resolution_minutes"`
	NearDueMinutesJSON  string  `orm:"near_due_minutes_json"`
}

func (s *serviceImpl) createSLARound(ctx context.Context, ticketNo string, round int, departmentNo, categoryNo, priority string, startedAt time.Time) error {
	if departmentNo == "" {
		_, err := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_sla_round (tenant_id,ticket_no,round_no,created_at) VALUES (0,?,?,?)`, ticketNo, round, startedAt)
		return err
	}
	var rows []slaSnapshotRow
	err := s.database.GetScan(ctx, &rows, `SELECT r.rule_no,r.version AS rule_version,c.calendar_no,c.version AS calendar_version,c.mode_code AS calendar_mode,c.timezone_name,array_to_json(c.workdays)::TEXT AS workdays_json,c.work_start_minute,c.work_end_minute,r.first_response_minutes,r.resolution_minutes,array_to_json(r.near_due_minutes)::TEXT AS near_due_minutes_json FROM plugin_idc_core_support_sla_rule r JOIN plugin_idc_core_support_sla_calendar c ON c.tenant_id=0 AND c.calendar_no=r.calendar_no AND c.status_code='active' WHERE r.tenant_id=0 AND r.department_no=? AND r.priority_code=? AND r.status_code='active' AND (r.category_no=? OR r.category_no IS NULL) ORDER BY CASE WHEN r.category_no=? THEN 0 ELSE 1 END,r.id LIMIT 1`, departmentNo, priority, nullable(categoryNo), nullable(categoryNo))
	if err != nil {
		return gerror.Wrap(err, "load support SLA rule snapshot")
	}
	if len(rows) == 0 {
		_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_sla_round (tenant_id,ticket_no,round_no,created_at) VALUES (0,?,?,?)`, ticketNo, round, startedAt)
		return err
	}
	snapshot, err := rows[0].snapshot()
	if err != nil {
		return gerror.Wrap(err, "decode support SLA rule snapshot")
	}
	if snapshot.CalendarMode == "business_hours" {
		if err = s.database.GetScan(ctx, &snapshot.HolidayDates, `SELECT holiday_date FROM plugin_idc_core_support_sla_holiday WHERE tenant_id=0 AND calendar_no=? ORDER BY holiday_date`, snapshot.CalendarNo); err != nil {
			return err
		}
	}
	firstDue, err := slaDueAt(startedAt, snapshot, snapshot.FirstResponseMinute)
	if err != nil {
		return err
	}
	resolutionDue, err := slaDueAt(startedAt, snapshot, snapshot.ResolutionMinute)
	if err != nil {
		return err
	}
	_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_sla_round (tenant_id,ticket_no,round_no,rule_no,rule_version,calendar_no,calendar_version,calendar_mode,timezone_name,workdays,work_start_minute,work_end_minute,holiday_dates,first_response_minutes,resolution_minutes,near_due_minutes,first_response_due_at,resolution_due_at,created_at) VALUES (0,?,?,?,?,?,?,?,?,?::SMALLINT[],?,?,?::DATE[],?,?,?::INT[],?,?,?)`, ticketNo, round, snapshot.RuleNo, snapshot.RuleVersion, snapshot.CalendarNo, snapshot.CalendarVersion, snapshot.CalendarMode, snapshot.TimezoneName, postgresSmallintArray(snapshot.Workdays), snapshot.WorkStartMinute, snapshot.WorkEndMinute, postgresDateArray(snapshot.HolidayDates), snapshot.FirstResponseMinute, snapshot.ResolutionMinute, postgresIntArray(snapshot.NearDueMinutes), firstDue, resolutionDue, startedAt)
	return gerror.Wrap(err, "insert support SLA rule snapshot")
}

func (row slaSnapshotRow) snapshot() (slaSnapshot, error) {
	var workdays []int16
	if err := json.Unmarshal([]byte(row.WorkdaysJSON), &workdays); err != nil {
		return slaSnapshot{}, err
	}
	var nearDueMinutes []int
	if err := json.Unmarshal([]byte(row.NearDueMinutesJSON), &nearDueMinutes); err != nil {
		return slaSnapshot{}, err
	}
	if row.CalendarMode == "continuous" {
		workdays = nil
	}
	return slaSnapshot{
		RuleNo: row.RuleNo, RuleVersion: row.RuleVersion,
		CalendarNo: row.CalendarNo, CalendarVersion: row.CalendarVersion,
		CalendarMode: row.CalendarMode, TimezoneName: row.TimezoneName,
		Workdays: workdays, WorkStartMinute: row.WorkStartMinute, WorkEndMinute: row.WorkEndMinute,
		FirstResponseMinute: row.FirstResponseMinute, ResolutionMinute: row.ResolutionMinute,
		NearDueMinutes: nearDueMinutes,
	}, nil
}

func postgresSmallintArray(values []int16) any {
	if values == nil {
		return nil
	}
	parts := make([]string, len(values))
	for index, value := range values {
		parts[index] = strconv.FormatInt(int64(value), 10)
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func postgresDateArray(values []time.Time) any {
	if values == nil {
		return nil
	}
	parts := make([]string, len(values))
	for index, value := range values {
		parts[index] = value.Format("2006-01-02")
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func slaDueAt(start time.Time, snapshot slaSnapshot, minutes *int) (any, error) {
	if minutes == nil {
		return nil, nil
	}
	if snapshot.CalendarMode == "continuous" {
		return start.Add(time.Duration(*minutes) * time.Minute), nil
	}
	if snapshot.TimezoneName == nil || snapshot.WorkStartMinute == nil || snapshot.WorkEndMinute == nil {
		return nil, bizInvalid()
	}
	location, err := time.LoadLocation(*snapshot.TimezoneName)
	if err != nil {
		return nil, bizInvalid()
	}
	remaining := *minutes
	cursor := start.In(location)
	holidays := make(map[string]struct{}, len(snapshot.HolidayDates))
	for _, day := range snapshot.HolidayDates {
		holidays[day.In(location).Format("2006-01-02")] = struct{}{}
	}
	workdays := make(map[int]struct{}, len(snapshot.Workdays))
	for _, day := range snapshot.Workdays {
		workdays[int(day)] = struct{}{}
	}
	for days := 0; days < 3660; days++ {
		date := time.Date(cursor.Year(), cursor.Month(), cursor.Day(), 0, 0, 0, 0, location)
		isoDay := int(date.Weekday())
		if isoDay == 0 {
			isoDay = 7
		}
		_, workingDay := workdays[isoDay]
		_, holiday := holidays[date.Format("2006-01-02")]
		if workingDay && !holiday {
			windowStart := date.Add(time.Duration(*snapshot.WorkStartMinute) * time.Minute)
			windowEnd := date.Add(time.Duration(*snapshot.WorkEndMinute) * time.Minute)
			if cursor.Before(windowStart) {
				cursor = windowStart
			}
			if cursor.Before(windowEnd) {
				available := int(windowEnd.Sub(cursor) / time.Minute)
				if remaining <= available {
					return cursor.Add(time.Duration(remaining) * time.Minute).UTC(), nil
				}
				remaining -= available
			}
		}
		cursor = date.AddDate(0, 0, 1)
	}
	return nil, bizInvalid()
}
