package supportdesk

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
)

const maxSupportSettings = 500

func (s *serviceImpl) DepartmentSettings(ctx context.Context) ([]DepartmentOption, error) {
	a, err := s.staffActor(ctx, PermissionConfigure)
	if err != nil {
		return nil, err
	}
	if a.DataScope >= 3 {
		return nil, bizNotAccessible()
	}
	var rows []DepartmentOption
	err = s.database.GetScan(ctx, &rows, `SELECT department_no,org_dept_id,name_zh_cn,name_en_us,COALESCE(parent_department_no,'') AS parent_department_no,assignment_mode,status_code AS status,version FROM plugin_idc_core_support_department WHERE tenant_id=0 ORDER BY name_zh_cn,department_no LIMIT ?`, maxSupportSettings)
	if err != nil {
		return nil, settingReadResult(err, "load support department settings")
	}
	return rows, s.projectOrganizationNames(ctx, rows)
}

func (s *serviceImpl) CreateDepartmentSetting(ctx context.Context, input DepartmentSettingInput) (*DepartmentOption, error) {
	a, err := s.staffActor(ctx, PermissionConfigure)
	if err != nil {
		return nil, err
	}
	input, ok := normalizeDepartmentSetting(input, true)
	if !ok {
		return nil, bizInvalid()
	}
	departmentNo, err := s.newNo("DEP_")
	if err != nil {
		return nil, wrapUnavailable(err, "generate support department number")
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if err := s.requireOrganizationBinding(ctx, input.OrgDeptID); err != nil {
			return err
		}
		if err := s.lockSupportSettings(ctx); err != nil {
			return err
		}
		if err := s.requireDepartmentParent(ctx, departmentNo, input.ParentDepartmentNo); err != nil {
			return err
		}
		_, err := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_department (tenant_id,department_no,org_dept_id,name_zh_cn,name_en_us,parent_department_no,assignment_mode,status_code,version,created_by,updated_by) VALUES (0,?,?,?,?,?,?,?,1,?,?)`, departmentNo, input.OrgDeptID, input.NameZhCN, input.NameEnUS, nullable(input.ParentDepartmentNo), input.AssignmentMode, input.Status, a.UserID, a.UserID)
		return err
	})
	if err = settingWriteResult(err, "create support department setting"); err != nil {
		return nil, err
	}
	return s.departmentSetting(ctx, departmentNo)
}

func (s *serviceImpl) UpdateDepartmentSetting(ctx context.Context, departmentNo string, input DepartmentSettingInput) (*DepartmentOption, error) {
	a, err := s.staffActor(ctx, PermissionConfigure)
	if err != nil {
		return nil, err
	}
	input, ok := normalizeDepartmentSetting(input, false)
	if !ok || !businessNoPattern.MatchString(departmentNo) || input.ExpectedVersion <= 0 {
		return nil, bizInvalid()
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if err := s.requireOrganizationBinding(ctx, input.OrgDeptID); err != nil {
			return err
		}
		if err := s.lockSupportSettings(ctx); err != nil {
			return err
		}
		if err := s.requireDepartmentParent(ctx, departmentNo, input.ParentDepartmentNo); err != nil {
			return err
		}
		if err := s.requireQueueConfigurationScope(ctx, departmentNo); err != nil {
			return err
		}
		if input.Status == SettingStatusDisabled {
			if err := s.requireQueueConfigurationScope(ctx, departmentNo); err != nil {
				return err
			}
			if err := s.ensureDepartmentCanDisable(ctx, departmentNo); err != nil {
				return err
			}
		}
		now, err := databaseNow(ctx, s.database)
		if err != nil {
			return err
		}
		result, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_support_department SET org_dept_id=?,name_zh_cn=?,name_en_us=?,parent_department_no=?,assignment_mode=?,status_code=?,version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND department_no=? AND version=?`, input.OrgDeptID, input.NameZhCN, input.NameEnUS, nullable(input.ParentDepartmentNo), input.AssignmentMode, input.Status, a.UserID, now, departmentNo, input.ExpectedVersion)
		return requireOneChanged(result, err)
	})
	if err = settingWriteResult(err, "update support department setting"); err != nil {
		return nil, err
	}
	return s.departmentSetting(ctx, departmentNo)
}

func (s *serviceImpl) DisableDepartmentSetting(ctx context.Context, departmentNo string, expectedVersion int64) (*DepartmentOption, error) {
	a, err := s.staffActor(ctx, PermissionConfigure)
	if err != nil {
		return nil, err
	}
	if !businessNoPattern.MatchString(departmentNo) || expectedVersion <= 0 {
		return nil, bizInvalid()
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if err := s.lockSupportSettings(ctx); err != nil {
			return err
		}
		if err := s.ensureDepartmentCanDisable(ctx, departmentNo); err != nil {
			return err
		}
		now, err := databaseNow(ctx, s.database)
		if err != nil {
			return err
		}
		result, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_support_department SET status_code='disabled',version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND department_no=? AND version=? AND status_code<>'disabled'`, a.UserID, now, departmentNo, expectedVersion)
		return requireOneChanged(result, err)
	})
	if err = settingWriteResult(err, "disable support department setting"); err != nil {
		return nil, err
	}
	return s.departmentSetting(ctx, departmentNo)
}

func (s *serviceImpl) IssueCategorySettings(ctx context.Context) ([]IssueCategorySetting, error) {
	if _, err := s.staffActor(ctx, PermissionConfigure); err != nil {
		return nil, err
	}
	var rows []IssueCategorySetting
	err := s.database.GetScan(ctx, &rows, `SELECT category_no,name_zh_cn,name_en_us,COALESCE(department_no,'') AS department_no,priority_code AS priority,sort_order,status_code AS status,version FROM plugin_idc_core_support_issue_category WHERE tenant_id=0 ORDER BY sort_order,id LIMIT ?`, maxSupportSettings)
	return rows, settingReadResult(err, "load support issue category settings")
}

func (s *serviceImpl) CreateIssueCategorySetting(ctx context.Context, input IssueCategorySettingInput) (*IssueCategorySetting, error) {
	a, err := s.staffActor(ctx, PermissionConfigure)
	if err != nil {
		return nil, err
	}
	input, ok := normalizeIssueCategorySetting(input)
	if !ok {
		return nil, bizInvalid()
	}
	categoryNo, err := s.newNo("CAT_")
	if err != nil {
		return nil, wrapUnavailable(err, "generate support category number")
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if err := s.lockSupportSettings(ctx); err != nil {
			return err
		}
		if err := s.requireActiveDepartment(ctx, input.DepartmentNo); err != nil {
			return err
		}
		_, err := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_issue_category (tenant_id,category_no,name_zh_cn,name_en_us,department_no,priority_code,sort_order,status_code,version,created_by,updated_by) VALUES (0,?,?,?,?,?,?,?,1,?,?)`, categoryNo, input.NameZhCN, input.NameEnUS, nullable(input.DepartmentNo), input.Priority, input.SortOrder, input.Status, a.UserID, a.UserID)
		return err
	})
	if err = settingWriteResult(err, "create support issue category setting"); err != nil {
		return nil, err
	}
	return s.issueCategorySetting(ctx, categoryNo)
}

func (s *serviceImpl) UpdateIssueCategorySetting(ctx context.Context, categoryNo string, input IssueCategorySettingInput) (*IssueCategorySetting, error) {
	a, err := s.staffActor(ctx, PermissionConfigure)
	if err != nil {
		return nil, err
	}
	input, ok := normalizeIssueCategorySetting(input)
	if !ok || !businessNoPattern.MatchString(categoryNo) || input.ExpectedVersion <= 0 {
		return nil, bizInvalid()
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if err := s.lockSupportSettings(ctx); err != nil {
			return err
		}
		if err := s.requireActiveDepartment(ctx, input.DepartmentNo); err != nil {
			return err
		}
		now, err := databaseNow(ctx, s.database)
		if err != nil {
			return err
		}
		result, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_support_issue_category SET name_zh_cn=?,name_en_us=?,department_no=?,priority_code=?,sort_order=?,status_code=?,version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND category_no=? AND version=?`, input.NameZhCN, input.NameEnUS, nullable(input.DepartmentNo), input.Priority, input.SortOrder, input.Status, a.UserID, now, categoryNo, input.ExpectedVersion)
		return requireOneChanged(result, err)
	})
	if err = settingWriteResult(err, "update support issue category setting"); err != nil {
		return nil, err
	}
	return s.issueCategorySetting(ctx, categoryNo)
}

func (s *serviceImpl) DisableIssueCategorySetting(ctx context.Context, categoryNo string, expectedVersion int64) (*IssueCategorySetting, error) {
	if err := s.disableVersionedSetting(ctx, "plugin_idc_core_support_issue_category", "category_no", categoryNo, expectedVersion); err != nil {
		return nil, err
	}
	return s.issueCategorySetting(ctx, categoryNo)
}

func (s *serviceImpl) CustomStatusSettings(ctx context.Context) ([]CustomStatusSetting, error) {
	if _, err := s.staffActor(ctx, PermissionConfigure); err != nil {
		return nil, err
	}
	var rows []CustomStatusSetting
	err := s.database.GetScan(ctx, &rows, `SELECT status_no,name_zh_cn,name_en_us,base_stage,sort_order,status_code AS status,version FROM plugin_idc_core_support_custom_status WHERE tenant_id=0 ORDER BY sort_order,id LIMIT ?`, maxSupportSettings)
	return rows, settingReadResult(err, "load support custom status settings")
}

func (s *serviceImpl) CreateCustomStatusSetting(ctx context.Context, input CustomStatusSettingInput) (*CustomStatusSetting, error) {
	a, err := s.staffActor(ctx, PermissionConfigure)
	if err != nil {
		return nil, err
	}
	input, ok := normalizeCustomStatusSetting(input)
	if !ok {
		return nil, bizInvalid()
	}
	statusNo, err := s.newNo("STS_")
	if err != nil {
		return nil, wrapUnavailable(err, "generate support status number")
	}
	_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_custom_status (tenant_id,status_no,name_zh_cn,name_en_us,base_stage,sort_order,status_code,version,created_by,updated_by) VALUES (0,?,?,?,?,?,?,1,?,?)`, statusNo, input.NameZhCN, input.NameEnUS, input.BaseStage, input.SortOrder, input.Status, a.UserID, a.UserID)
	if err = settingWriteResult(err, "create support custom status setting"); err != nil {
		return nil, err
	}
	return s.customStatusSetting(ctx, statusNo)
}

func (s *serviceImpl) UpdateCustomStatusSetting(ctx context.Context, statusNo string, input CustomStatusSettingInput) (*CustomStatusSetting, error) {
	a, err := s.staffActor(ctx, PermissionConfigure)
	if err != nil {
		return nil, err
	}
	input, ok := normalizeCustomStatusSetting(input)
	if !ok || !businessNoPattern.MatchString(statusNo) || input.ExpectedVersion <= 0 {
		return nil, bizInvalid()
	}
	now, err := databaseNow(ctx, s.database)
	if err != nil {
		return nil, wrapUnavailable(err, "load database time")
	}
	result, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_support_custom_status SET name_zh_cn=?,name_en_us=?,base_stage=?,sort_order=?,status_code=?,version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND status_no=? AND version=?`, input.NameZhCN, input.NameEnUS, input.BaseStage, input.SortOrder, input.Status, a.UserID, now, statusNo, input.ExpectedVersion)
	if err = settingWriteResult(requireOneChanged(result, err), "update support custom status setting"); err != nil {
		return nil, err
	}
	return s.customStatusSetting(ctx, statusNo)
}

func (s *serviceImpl) DisableCustomStatusSetting(ctx context.Context, statusNo string, expectedVersion int64) (*CustomStatusSetting, error) {
	if err := s.disableVersionedSetting(ctx, "plugin_idc_core_support_custom_status", "status_no", statusNo, expectedVersion); err != nil {
		return nil, err
	}
	return s.customStatusSetting(ctx, statusNo)
}

func (s *serviceImpl) ReplyPresetSettings(ctx context.Context) ([]ReplyPresetSetting, error) {
	if _, err := s.staffActor(ctx, PermissionConfigure); err != nil {
		return nil, err
	}
	var rows []ReplyPresetSetting
	err := s.database.GetScan(ctx, &rows, `SELECT preset_no,department_no,locale,title,body,sort_order,status_code AS status,version FROM plugin_idc_core_support_reply_preset WHERE tenant_id=0 ORDER BY department_no,locale,sort_order,id LIMIT ?`, maxSupportSettings)
	return rows, settingReadResult(err, "load support reply preset settings")
}

func (s *serviceImpl) CreateReplyPresetSetting(ctx context.Context, input ReplyPresetSettingInput) (*ReplyPresetSetting, error) {
	a, err := s.staffActor(ctx, PermissionConfigure)
	if err != nil {
		return nil, err
	}
	input, ok := normalizeReplyPresetSetting(input)
	if !ok {
		return nil, bizInvalid()
	}
	presetNo, err := s.newNo("RPL_")
	if err != nil {
		return nil, wrapUnavailable(err, "generate support reply preset number")
	}
	err = s.requireActiveDepartment(ctx, input.DepartmentNo)
	if err == nil {
		_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_reply_preset (tenant_id,preset_no,department_no,locale,title,body,sort_order,status_code,version,created_by,updated_by) VALUES (0,?,?,?,?,?,?,?,1,?,?)`, presetNo, input.DepartmentNo, input.Locale, input.Title, input.Body, input.SortOrder, input.Status, a.UserID, a.UserID)
	}
	if err = settingWriteResult(err, "create support reply preset setting"); err != nil {
		return nil, err
	}
	return s.replyPresetSetting(ctx, presetNo)
}

func (s *serviceImpl) UpdateReplyPresetSetting(ctx context.Context, presetNo string, input ReplyPresetSettingInput) (*ReplyPresetSetting, error) {
	a, err := s.staffActor(ctx, PermissionConfigure)
	if err != nil {
		return nil, err
	}
	input, ok := normalizeReplyPresetSetting(input)
	if !ok || !businessNoPattern.MatchString(presetNo) || input.ExpectedVersion <= 0 {
		return nil, bizInvalid()
	}
	if err = s.requireActiveDepartment(ctx, input.DepartmentNo); err != nil {
		return nil, err
	}
	now, err := databaseNow(ctx, s.database)
	if err != nil {
		return nil, wrapUnavailable(err, "load database time")
	}
	result, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_support_reply_preset SET department_no=?,locale=?,title=?,body=?,sort_order=?,status_code=?,version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND preset_no=? AND version=?`, input.DepartmentNo, input.Locale, input.Title, input.Body, input.SortOrder, input.Status, a.UserID, now, presetNo, input.ExpectedVersion)
	if err = settingWriteResult(requireOneChanged(result, err), "update support reply preset setting"); err != nil {
		return nil, err
	}
	return s.replyPresetSetting(ctx, presetNo)
}

func (s *serviceImpl) DisableReplyPresetSetting(ctx context.Context, presetNo string, expectedVersion int64) (*ReplyPresetSetting, error) {
	if err := s.disableVersionedSetting(ctx, "plugin_idc_core_support_reply_preset", "preset_no", presetNo, expectedVersion); err != nil {
		return nil, err
	}
	return s.replyPresetSetting(ctx, presetNo)
}

func (s *serviceImpl) SLACalendarSettings(ctx context.Context) ([]SLACalendarSetting, error) {
	if _, err := s.staffActor(ctx, PermissionConfigure); err != nil {
		return nil, err
	}
	var rows []SLACalendarSetting
	err := s.database.GetScan(ctx, &rows, `SELECT calendar_no,name,mode_code AS mode,COALESCE(timezone_name,'') AS timezone,workdays,work_start_minute,work_end_minute,status_code AS status,version FROM plugin_idc_core_support_sla_calendar WHERE tenant_id=0 ORDER BY name,calendar_no LIMIT ?`, maxSupportSettings)
	return rows, settingReadResult(err, "load support SLA calendar settings")
}

func (s *serviceImpl) CreateSLACalendarSetting(ctx context.Context, input SLACalendarSettingInput) (*SLACalendarSetting, error) {
	a, err := s.staffActor(ctx, PermissionConfigure)
	if err != nil {
		return nil, err
	}
	input, ok := normalizeSLACalendarSetting(input)
	if !ok {
		return nil, bizInvalid()
	}
	calendarNo, err := s.newNo("SLC_")
	if err != nil {
		return nil, wrapUnavailable(err, "generate support SLA calendar number")
	}
	_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_sla_calendar (tenant_id,calendar_no,name,mode_code,timezone_name,workdays,work_start_minute,work_end_minute,status_code,version,created_by,updated_by) VALUES (0,?,?,?,?,?::SMALLINT[],?,?,?,1,?,?)`, calendarNo, input.Name, input.Mode, nullable(input.Timezone), postgresIntArray(input.Workdays), input.WorkStartMinute, input.WorkEndMinute, input.Status, a.UserID, a.UserID)
	if err = settingWriteResult(err, "create support SLA calendar setting"); err != nil {
		return nil, err
	}
	return s.slaCalendarSetting(ctx, calendarNo)
}

func (s *serviceImpl) UpdateSLACalendarSetting(ctx context.Context, calendarNo string, input SLACalendarSettingInput) (*SLACalendarSetting, error) {
	a, err := s.staffActor(ctx, PermissionConfigure)
	if err != nil {
		return nil, err
	}
	input, ok := normalizeSLACalendarSetting(input)
	if !ok || !businessNoPattern.MatchString(calendarNo) || input.ExpectedVersion <= 0 {
		return nil, bizInvalid()
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if err := s.lockSupportSettings(ctx); err != nil {
			return err
		}
		if input.Status == SettingStatusDisabled {
			if err := s.ensureSLACalendarCanDisable(ctx, calendarNo); err != nil {
				return err
			}
		}
		now, err := databaseNow(ctx, s.database)
		if err != nil {
			return err
		}
		result, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_support_sla_calendar SET name=?,mode_code=?,timezone_name=?,workdays=?::SMALLINT[],work_start_minute=?,work_end_minute=?,status_code=?,version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND calendar_no=? AND version=?`, input.Name, input.Mode, nullable(input.Timezone), postgresIntArray(input.Workdays), input.WorkStartMinute, input.WorkEndMinute, input.Status, a.UserID, now, calendarNo, input.ExpectedVersion)
		return requireOneChanged(result, err)
	})
	if err = settingWriteResult(err, "update support SLA calendar setting"); err != nil {
		return nil, err
	}
	return s.slaCalendarSetting(ctx, calendarNo)
}

func (s *serviceImpl) DisableSLACalendarSetting(ctx context.Context, calendarNo string, expectedVersion int64) (*SLACalendarSetting, error) {
	if _, err := s.staffActor(ctx, PermissionConfigure); err != nil {
		return nil, err
	}
	if !businessNoPattern.MatchString(calendarNo) || expectedVersion <= 0 {
		return nil, bizInvalid()
	}
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if err := s.lockSupportSettings(ctx); err != nil {
			return err
		}
		if err := s.ensureSLACalendarCanDisable(ctx, calendarNo); err != nil {
			return err
		}
		return s.disableVersionedSettingLocked(ctx, "plugin_idc_core_support_sla_calendar", "calendar_no", calendarNo, expectedVersion)
	})
	if err = settingWriteResult(err, "disable support SLA calendar setting"); err != nil {
		return nil, err
	}
	return s.slaCalendarSetting(ctx, calendarNo)
}

func (s *serviceImpl) SLAHolidaySettings(ctx context.Context, calendarNo string) ([]SLAHolidaySetting, error) {
	if _, err := s.staffActor(ctx, PermissionConfigure); err != nil {
		return nil, err
	}
	if !businessNoPattern.MatchString(calendarNo) {
		return nil, bizInvalid()
	}
	var rows []SLAHolidaySetting
	err := s.database.GetScan(ctx, &rows, `SELECT calendar_no,to_char(holiday_date,'YYYY-MM-DD') AS date,name FROM plugin_idc_core_support_sla_holiday WHERE tenant_id=0 AND calendar_no=? ORDER BY holiday_date LIMIT ?`, calendarNo, maxSupportSettings)
	return rows, settingReadResult(err, "load support SLA holidays")
}

func (s *serviceImpl) PutSLAHolidaySetting(ctx context.Context, calendarNo string, input SLAHolidaySettingInput) (*SLAHolidaySetting, error) {
	_, err := s.staffActor(ctx, PermissionConfigure)
	if err != nil {
		return nil, err
	}
	input.Name, _ = normalizeText(input.Name, 128)
	if !businessNoPattern.MatchString(calendarNo) || !validDate(input.Date) || input.Name == "" {
		return nil, bizInvalid()
	}
	calendarCount, err := queryCount(ctx, s.database, `SELECT COUNT(1) FROM plugin_idc_core_support_sla_calendar WHERE tenant_id=0 AND calendar_no=? AND status_code='active' AND mode_code='business_hours'`, calendarNo)
	if err != nil {
		return nil, wrapUnavailable(err, "load support SLA calendar")
	}
	if calendarCount != 1 {
		return nil, bizInvalid()
	}
	_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_sla_holiday (tenant_id,calendar_no,holiday_date,name,created_at) VALUES (0,?,?,?,CURRENT_TIMESTAMP) ON CONFLICT (calendar_no,holiday_date) DO UPDATE SET name=EXCLUDED.name`, calendarNo, input.Date, input.Name)
	if err = settingWriteResult(err, "save support SLA holiday"); err != nil {
		return nil, err
	}
	return &SLAHolidaySetting{CalendarNo: calendarNo, Date: input.Date, Name: input.Name}, nil
}

func (s *serviceImpl) DeleteSLAHolidaySetting(ctx context.Context, calendarNo, date string) error {
	if _, err := s.staffActor(ctx, PermissionConfigure); err != nil {
		return err
	}
	if !businessNoPattern.MatchString(calendarNo) || !validDate(date) {
		return bizInvalid()
	}
	result, err := s.database.Exec(ctx, `DELETE FROM plugin_idc_core_support_sla_holiday WHERE tenant_id=0 AND calendar_no=? AND holiday_date=?`, calendarNo, date)
	return settingWriteResult(requireOneChanged(result, err), "delete support SLA holiday")
}

func (s *serviceImpl) SLARuleSettings(ctx context.Context) ([]SLARuleSetting, error) {
	if _, err := s.staffActor(ctx, PermissionConfigure); err != nil {
		return nil, err
	}
	var rows []SLARuleSetting
	err := s.database.GetScan(ctx, &rows, `SELECT rule_no,department_no,COALESCE(category_no,'') AS category_no,priority_code AS priority,calendar_no,first_response_minutes,resolution_minutes,near_due_minutes,status_code AS status,version FROM plugin_idc_core_support_sla_rule WHERE tenant_id=0 ORDER BY department_no,category_no NULLS FIRST,priority_code,id LIMIT ?`, maxSupportSettings)
	return rows, settingReadResult(err, "load support SLA rule settings")
}

func (s *serviceImpl) CreateSLARuleSetting(ctx context.Context, input SLARuleSettingInput) (*SLARuleSetting, error) {
	a, err := s.staffActor(ctx, PermissionConfigure)
	if err != nil {
		return nil, err
	}
	input, ok := normalizeSLARuleSetting(input)
	if !ok {
		return nil, bizInvalid()
	}
	ruleNo, err := s.newNo("SLR_")
	if err != nil {
		return nil, wrapUnavailable(err, "generate support SLA rule number")
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if err := s.lockSupportSettings(ctx); err != nil {
			return err
		}
		if err := s.validateSLARuleReferences(ctx, input); err != nil {
			return err
		}
		if err := s.lockSLARuleScope(ctx, input, ""); err != nil {
			return err
		}
		_, err := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_sla_rule (tenant_id,rule_no,department_no,category_no,priority_code,calendar_no,first_response_minutes,resolution_minutes,near_due_minutes,status_code,version,created_by,updated_by) VALUES (0,?,?,?,?,?,?,?,?::INT[],?,1,?,?)`, ruleNo, input.DepartmentNo, nullable(input.CategoryNo), input.Priority, input.CalendarNo, input.FirstResponseMinutes, input.ResolutionMinutes, postgresIntArray(input.NearDueMinutes), input.Status, a.UserID, a.UserID)
		return err
	})
	if err = settingWriteResult(err, "create support SLA rule setting"); err != nil {
		return nil, err
	}
	return s.slaRuleSetting(ctx, ruleNo)
}

func (s *serviceImpl) UpdateSLARuleSetting(ctx context.Context, ruleNo string, input SLARuleSettingInput) (*SLARuleSetting, error) {
	a, err := s.staffActor(ctx, PermissionConfigure)
	if err != nil {
		return nil, err
	}
	input, ok := normalizeSLARuleSetting(input)
	if !ok || !businessNoPattern.MatchString(ruleNo) || input.ExpectedVersion <= 0 {
		return nil, bizInvalid()
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if err := s.lockSupportSettings(ctx); err != nil {
			return err
		}
		if err := s.validateSLARuleReferences(ctx, input); err != nil {
			return err
		}
		if err := s.lockSLARuleScope(ctx, input, ruleNo); err != nil {
			return err
		}
		now, err := databaseNow(ctx, s.database)
		if err != nil {
			return err
		}
		result, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_support_sla_rule SET department_no=?,category_no=?,priority_code=?,calendar_no=?,first_response_minutes=?,resolution_minutes=?,near_due_minutes=?::INT[],status_code=?,version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND rule_no=? AND version=?`, input.DepartmentNo, nullable(input.CategoryNo), input.Priority, input.CalendarNo, input.FirstResponseMinutes, input.ResolutionMinutes, postgresIntArray(input.NearDueMinutes), input.Status, a.UserID, now, ruleNo, input.ExpectedVersion)
		return requireOneChanged(result, err)
	})
	if err = settingWriteResult(err, "update support SLA rule setting"); err != nil {
		return nil, err
	}
	return s.slaRuleSetting(ctx, ruleNo)
}

func (s *serviceImpl) DisableSLARuleSetting(ctx context.Context, ruleNo string, expectedVersion int64) (*SLARuleSetting, error) {
	if err := s.disableVersionedSetting(ctx, "plugin_idc_core_support_sla_rule", "rule_no", ruleNo, expectedVersion); err != nil {
		return nil, err
	}
	return s.slaRuleSetting(ctx, ruleNo)
}

func normalizeDepartmentSetting(input DepartmentSettingInput, create bool) (DepartmentSettingInput, bool) {
	input.NameZhCN, _ = normalizeText(input.NameZhCN, 128)
	input.NameEnUS, _ = normalizeText(input.NameEnUS, 128)
	input.ParentDepartmentNo = strings.TrimSpace(input.ParentDepartmentNo)
	if create && input.AssignmentMode == "" {
		input.AssignmentMode = AssignmentModeManualClaim
	}
	if input.Status == "" {
		input.Status = SettingStatusActive
	}
	validMode := input.AssignmentMode == AssignmentModeManualClaim
	return input, input.OrgDeptID > 0 && input.NameZhCN != "" && input.NameEnUS != "" && validMode && validSettingStatus(input.Status) && (input.ParentDepartmentNo == "" || businessNoPattern.MatchString(input.ParentDepartmentNo))
}

func normalizeIssueCategorySetting(input IssueCategorySettingInput) (IssueCategorySettingInput, bool) {
	input.NameZhCN, _ = normalizeText(input.NameZhCN, 128)
	input.NameEnUS, _ = normalizeText(input.NameEnUS, 128)
	input.DepartmentNo = strings.TrimSpace(input.DepartmentNo)
	input.Priority = strings.TrimSpace(input.Priority)
	if input.Status == "" {
		input.Status = SettingStatusActive
	}
	return input, input.NameZhCN != "" && input.NameEnUS != "" && validSettingPriority(input.Priority) && validSettingStatus(input.Status) && input.SortOrder >= 0 && (input.DepartmentNo == "" || businessNoPattern.MatchString(input.DepartmentNo))
}

func normalizeCustomStatusSetting(input CustomStatusSettingInput) (CustomStatusSettingInput, bool) {
	input.NameZhCN, _ = normalizeText(input.NameZhCN, 128)
	input.NameEnUS, _ = normalizeText(input.NameEnUS, 128)
	input.BaseStage = strings.TrimSpace(input.BaseStage)
	if input.Status == "" {
		input.Status = SettingStatusActive
	}
	return input, input.NameZhCN != "" && input.NameEnUS != "" && validBaseStage(input.BaseStage) && validSettingStatus(input.Status) && input.SortOrder >= 0
}

func normalizeReplyPresetSetting(input ReplyPresetSettingInput) (ReplyPresetSettingInput, bool) {
	input.DepartmentNo = strings.TrimSpace(input.DepartmentNo)
	input.Locale = strings.TrimSpace(input.Locale)
	input.Title, _ = normalizeText(input.Title, 128)
	input.Body, _ = normalizeText(input.Body, maxTicketBodyBytes)
	if input.Status == "" {
		input.Status = SettingStatusActive
	}
	return input, businessNoPattern.MatchString(input.DepartmentNo) && (input.Locale == "zh-CN" || input.Locale == "en-US") && input.Title != "" && input.Body != "" && input.SortOrder >= 0 && validSettingStatus(input.Status)
}

func normalizeSLACalendarSetting(input SLACalendarSettingInput) (SLACalendarSettingInput, bool) {
	input.Name, _ = normalizeText(input.Name, 128)
	input.Mode = strings.TrimSpace(input.Mode)
	input.Timezone = strings.TrimSpace(input.Timezone)
	if input.Mode == "" {
		input.Mode = "continuous"
	}
	if input.Status == "" {
		input.Status = SettingStatusActive
	}
	if input.Name == "" || !validSettingStatus(input.Status) {
		return input, false
	}
	if input.Mode == "continuous" {
		input.Timezone, input.Workdays, input.WorkStartMinute, input.WorkEndMinute = "", []int{}, nil, nil
		return input, true
	}
	if input.Mode != "business_hours" || input.WorkStartMinute == nil || input.WorkEndMinute == nil || *input.WorkStartMinute < 0 || *input.WorkStartMinute > 1439 || *input.WorkEndMinute < 1 || *input.WorkEndMinute > 1440 || *input.WorkStartMinute >= *input.WorkEndMinute {
		return input, false
	}
	if _, err := time.LoadLocation(input.Timezone); err != nil {
		return input, false
	}
	input.Workdays = uniqueSortedDays(input.Workdays)
	return input, len(input.Workdays) != 0
}

func normalizeSLARuleSetting(input SLARuleSettingInput) (SLARuleSettingInput, bool) {
	input.DepartmentNo = strings.TrimSpace(input.DepartmentNo)
	input.CategoryNo = strings.TrimSpace(input.CategoryNo)
	input.Priority = strings.TrimSpace(input.Priority)
	input.CalendarNo = strings.TrimSpace(input.CalendarNo)
	if input.Status == "" {
		input.Status = SettingStatusActive
	}
	if !businessNoPattern.MatchString(input.DepartmentNo) || (input.CategoryNo != "" && !businessNoPattern.MatchString(input.CategoryNo)) || !businessNoPattern.MatchString(input.CalendarNo) || !validSettingPriority(input.Priority) || !validSettingStatus(input.Status) {
		return input, false
	}
	if (input.FirstResponseMinutes == nil && input.ResolutionMinutes == nil) || !validPositiveMinutes(input.FirstResponseMinutes) || !validPositiveMinutes(input.ResolutionMinutes) {
		return input, false
	}
	nodes, ok := uniqueDescendingPositive(input.NearDueMinutes)
	input.NearDueMinutes = nodes
	return input, ok
}

func (s *serviceImpl) departmentSetting(ctx context.Context, departmentNo string) (*DepartmentOption, error) {
	var rows []DepartmentOption
	err := s.database.GetScan(ctx, &rows, `SELECT department_no,org_dept_id,name_zh_cn,name_en_us,COALESCE(parent_department_no,'') AS parent_department_no,assignment_mode,status_code AS status,version FROM plugin_idc_core_support_department WHERE tenant_id=0 AND department_no=? LIMIT 1`, departmentNo)
	if err = settingReadResult(err, "load support department setting"); err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, bizNotAccessible()
	}
	if err := s.projectOrganizationNames(ctx, rows); err != nil {
		return nil, err
	}
	return &rows[0], nil
}

func (s *serviceImpl) issueCategorySetting(ctx context.Context, categoryNo string) (*IssueCategorySetting, error) {
	var rows []IssueCategorySetting
	err := s.database.GetScan(ctx, &rows, `SELECT category_no,name_zh_cn,name_en_us,COALESCE(department_no,'') AS department_no,priority_code AS priority,sort_order,status_code AS status,version FROM plugin_idc_core_support_issue_category WHERE tenant_id=0 AND category_no=? LIMIT 1`, categoryNo)
	if err = settingReadResult(err, "load support issue category setting"); err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, bizNotAccessible()
	}
	return &rows[0], nil
}

func (s *serviceImpl) customStatusSetting(ctx context.Context, statusNo string) (*CustomStatusSetting, error) {
	var rows []CustomStatusSetting
	err := s.database.GetScan(ctx, &rows, `SELECT status_no,name_zh_cn,name_en_us,base_stage,sort_order,status_code AS status,version FROM plugin_idc_core_support_custom_status WHERE tenant_id=0 AND status_no=? LIMIT 1`, statusNo)
	if err = settingReadResult(err, "load support custom status setting"); err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, bizNotAccessible()
	}
	return &rows[0], nil
}

func (s *serviceImpl) replyPresetSetting(ctx context.Context, presetNo string) (*ReplyPresetSetting, error) {
	var rows []ReplyPresetSetting
	err := s.database.GetScan(ctx, &rows, `SELECT preset_no,department_no,locale,title,body,sort_order,status_code AS status,version FROM plugin_idc_core_support_reply_preset WHERE tenant_id=0 AND preset_no=? LIMIT 1`, presetNo)
	if err = settingReadResult(err, "load support reply preset setting"); err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, bizNotAccessible()
	}
	return &rows[0], nil
}

func (s *serviceImpl) slaCalendarSetting(ctx context.Context, calendarNo string) (*SLACalendarSetting, error) {
	var rows []SLACalendarSetting
	err := s.database.GetScan(ctx, &rows, `SELECT calendar_no,name,mode_code AS mode,COALESCE(timezone_name,'') AS timezone,workdays,work_start_minute,work_end_minute,status_code AS status,version FROM plugin_idc_core_support_sla_calendar WHERE tenant_id=0 AND calendar_no=? LIMIT 1`, calendarNo)
	if err = settingReadResult(err, "load support SLA calendar setting"); err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, bizNotAccessible()
	}
	return &rows[0], nil
}

func (s *serviceImpl) slaRuleSetting(ctx context.Context, ruleNo string) (*SLARuleSetting, error) {
	var rows []SLARuleSetting
	err := s.database.GetScan(ctx, &rows, `SELECT rule_no,department_no,COALESCE(category_no,'') AS category_no,priority_code AS priority,calendar_no,first_response_minutes,resolution_minutes,near_due_minutes,status_code AS status,version FROM plugin_idc_core_support_sla_rule WHERE tenant_id=0 AND rule_no=? LIMIT 1`, ruleNo)
	if err = settingReadResult(err, "load support SLA rule setting"); err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, bizNotAccessible()
	}
	return &rows[0], nil
}

func (s *serviceImpl) disableVersionedSetting(ctx context.Context, table, keyColumn, key string, expectedVersion int64) error {
	a, err := s.staffActor(ctx, PermissionConfigure)
	if err != nil {
		return err
	}
	if !businessNoPattern.MatchString(key) || expectedVersion <= 0 {
		return bizInvalid()
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if err := s.lockSupportSettings(ctx); err != nil {
			return err
		}
		return s.disableVersionedSettingAs(ctx, table, keyColumn, key, expectedVersion, a.UserID)
	})
	return settingWriteResult(err, "disable support setting")
}

func (s *serviceImpl) disableVersionedSettingLocked(ctx context.Context, table, keyColumn, key string, expectedVersion int64) error {
	a, err := s.staffActor(ctx, PermissionConfigure)
	if err != nil {
		return err
	}
	return s.disableVersionedSettingAs(ctx, table, keyColumn, key, expectedVersion, a.UserID)
}

func (s *serviceImpl) disableVersionedSettingAs(ctx context.Context, table, keyColumn, key string, expectedVersion int64, userID int) error {
	now, err := databaseNow(ctx, s.database)
	if err != nil {
		return err
	}
	query := fmt.Sprintf(`UPDATE %s SET status_code='disabled',version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND %s=? AND version=? AND status_code<>'disabled'`, table, keyColumn)
	result, err := s.database.Exec(ctx, query, userID, now, key, expectedVersion)
	return requireOneChanged(result, err)
}

func (s *serviceImpl) requireDepartmentParent(ctx context.Context, departmentNo, parentNo string) error {
	if parentNo == "" {
		return nil
	}
	if departmentNo == parentNo {
		return bizInvalid()
	}
	count, err := queryCount(ctx, s.database, `WITH RECURSIVE descendants AS (SELECT department_no FROM plugin_idc_core_support_department WHERE tenant_id=0 AND parent_department_no=? UNION ALL SELECT d.department_no FROM plugin_idc_core_support_department d JOIN descendants p ON d.parent_department_no=p.department_no WHERE d.tenant_id=0) SELECT COUNT(1) FROM plugin_idc_core_support_department p WHERE p.tenant_id=0 AND p.department_no=? AND p.status_code='active' AND NOT EXISTS (SELECT 1 FROM descendants WHERE department_no=?)`, departmentNo, parentNo, parentNo)
	if err != nil {
		return err
	}
	if count != 1 {
		return bizInvalid()
	}
	return nil
}

func (s *serviceImpl) requireActiveDepartment(ctx context.Context, departmentNo string) error {
	if departmentNo == "" {
		return nil
	}
	count, err := queryCount(ctx, s.database, `SELECT COUNT(1) FROM plugin_idc_core_support_department WHERE tenant_id=0 AND department_no=? AND status_code='active'`, departmentNo)
	if err != nil {
		return err
	}
	if count != 1 {
		return bizInvalid()
	}
	return nil
}

func (s *serviceImpl) ensureDepartmentCanDisable(ctx context.Context, departmentNo string) error {
	count, err := queryCount(ctx, s.database, `SELECT (SELECT COUNT(1) FROM plugin_idc_core_support_issue_category WHERE tenant_id=0 AND department_no=? AND status_code='active')+(SELECT COUNT(1) FROM plugin_idc_core_support_sla_rule WHERE tenant_id=0 AND department_no=? AND status_code='active')`, departmentNo, departmentNo)
	if err != nil {
		return err
	}
	if count != 0 {
		return bizerr.NewCode(CodeConflict)
	}
	return nil
}

func (s *serviceImpl) ensureSLACalendarCanDisable(ctx context.Context, calendarNo string) error {
	activeRules, err := queryCount(ctx, s.database, `SELECT COUNT(1) FROM plugin_idc_core_support_sla_rule WHERE tenant_id=0 AND calendar_no=? AND status_code='active'`, calendarNo)
	if err != nil {
		return err
	}
	if activeRules != 0 {
		return bizerr.NewCode(CodeConflict)
	}
	return nil
}

func (s *serviceImpl) lockSupportSettings(ctx context.Context) error {
	_, err := s.database.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('idc-core:support-settings'))`)
	return err
}

func (s *serviceImpl) validateSLARuleReferences(ctx context.Context, input SLARuleSettingInput) error {
	count, err := queryCount(ctx, s.database, `SELECT COUNT(1) FROM plugin_idc_core_support_department d JOIN plugin_idc_core_support_sla_calendar c ON c.tenant_id=0 AND c.calendar_no=? AND c.status_code='active' WHERE d.tenant_id=0 AND d.department_no=? AND d.status_code='active'`, input.CalendarNo, input.DepartmentNo)
	if err != nil || count != 1 {
		if err != nil {
			return err
		}
		return bizInvalid()
	}
	if input.CategoryNo == "" {
		return nil
	}
	count, err = queryCount(ctx, s.database, `SELECT COUNT(1) FROM plugin_idc_core_support_issue_category WHERE tenant_id=0 AND category_no=? AND status_code='active' AND (department_no IS NULL OR department_no=?)`, input.CategoryNo, input.DepartmentNo)
	if err != nil {
		return err
	}
	if count != 1 {
		return bizInvalid()
	}
	return nil
}

func (s *serviceImpl) lockSLARuleScope(ctx context.Context, input SLARuleSettingInput, excludeRuleNo string) error {
	scope := input.DepartmentNo + ":" + input.CategoryNo + ":" + input.Priority
	if _, err := s.database.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext(?))`, scope); err != nil {
		return err
	}
	count, err := queryCount(ctx, s.database, `SELECT COUNT(1) FROM plugin_idc_core_support_sla_rule WHERE tenant_id=0 AND department_no=? AND category_no IS NOT DISTINCT FROM ? AND priority_code=? AND rule_no<>?`, input.DepartmentNo, nullable(input.CategoryNo), input.Priority, excludeRuleNo)
	if err != nil {
		return err
	}
	if count != 0 {
		return bizerr.NewCode(CodeConflict)
	}
	return nil
}

func requireOneChanged(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return bizerr.NewCode(CodeConflict)
	}
	return nil
}

func queryCount(ctx context.Context, database gdb.DB, query string, args ...any) (int, error) {
	value, err := database.GetValue(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return value.Int(), nil
}

func postgresIntArray(values []int) string {
	parts := make([]string, len(values))
	for index, value := range values {
		parts[index] = fmt.Sprintf("%d", value)
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func settingReadResult(err error, operation string) error {
	if err == nil {
		return nil
	}
	if _, ok := bizerr.As(err); ok {
		return err
	}
	return wrapUnavailable(err, operation)
}

func settingWriteResult(err error, operation string) error {
	return settingReadResult(err, operation)
}

func validSettingStatus(status SettingStatus) bool {
	return status == SettingStatusActive || status == SettingStatusDisabled
}

func validSettingPriority(priority string) bool {
	return priority == "low" || priority == "normal" || priority == "high" || priority == "urgent"
}

func validBaseStage(stage string) bool {
	return stage == StageNew || stage == StageInProgress || stage == StageWaitingCustomer || stage == StageWaitingUpstream || stage == StageResolved || stage == StageClosed
}

func validPositiveMinutes(value *int) bool {
	return value == nil || (*value > 0 && *value <= 525600)
}

func uniqueSortedDays(values []int) []int {
	seen := make(map[int]struct{}, len(values))
	result := make([]int, 0, len(values))
	for _, value := range values {
		if value < 1 || value > 7 {
			return nil
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Ints(result)
	return result
}

func uniqueDescendingPositive(values []int) ([]int, bool) {
	if len(values) > 20 {
		return nil, false
	}
	seen := make(map[int]struct{}, len(values))
	result := make([]int, 0, len(values))
	for _, value := range values {
		if value <= 0 || value > 525600 {
			return nil, false
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(result)))
	return result, true
}

func validDate(value string) bool {
	parsed, err := time.Parse("2006-01-02", value)
	return err == nil && parsed.Format("2006-01-02") == value
}
