package supportdesk

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
)

func (s *serviceImpl) Departments(ctx context.Context) ([]DepartmentOption, error) {
	a, err := s.staffActor(ctx, PermissionList)
	if err != nil {
		return nil, err
	}
	query := `SELECT department_no,org_dept_id,name_zh_cn,name_en_us,COALESCE(parent_department_no,'') AS parent_department_no,assignment_mode,status_code AS status,version FROM plugin_idc_core_support_department WHERE tenant_id=0 AND status_code='active' AND org_dept_id>0`
	args := []any{}
	if !a.Global {
		if len(a.Departments) == 0 {
			return []DepartmentOption{}, nil
		}
		query += ` AND department_no IN (` + sqlPlaceholders(len(a.Departments)) + `)`
		for _, departmentNo := range a.Departments {
			args = append(args, departmentNo)
		}
	}
	query += ` ORDER BY name_zh_cn,department_no LIMIT 500`
	var rows []DepartmentOption
	if err = s.database.GetScan(ctx, &rows, query, args...); err != nil {
		return nil, wrapUnavailable(err, "load support departments")
	}
	return rows, nil
}

func (s *serviceImpl) IssueCategories(ctx context.Context) ([]CategoryOption, error) {
	if _, err := s.staffActor(ctx, PermissionList); err != nil {
		return nil, err
	}
	var rows []CategoryOption
	if err := s.database.GetScan(ctx, &rows, `SELECT id,category_no,name_zh_cn,name_en_us,priority_code AS priority,sort_order FROM plugin_idc_core_support_issue_category WHERE tenant_id=0 AND status_code='active' ORDER BY sort_order,id LIMIT 500`); err != nil {
		return nil, wrapUnavailable(err, "load support issue categories")
	}
	return rows, nil
}

func (s *serviceImpl) Statuses(ctx context.Context) ([]StatusOption, error) {
	if _, err := s.staffActor(ctx, PermissionList); err != nil {
		return nil, err
	}
	var rows []StatusOption
	if err := s.database.GetScan(ctx, &rows, `SELECT status_no,name_zh_cn,name_en_us,base_stage,status_code AS status,version FROM plugin_idc_core_support_custom_status WHERE tenant_id=0 AND status_code='active' ORDER BY sort_order,id LIMIT 500`); err != nil {
		return nil, wrapUnavailable(err, "load support custom statuses")
	}
	return rows, nil
}

func (s *serviceImpl) ReplyPresets(ctx context.Context, locale string) ([]ReplyPresetOption, error) {
	a, err := s.staffActor(ctx, PermissionReply)
	if err != nil {
		return nil, err
	}
	if locale != "zh-CN" && locale != "en-US" {
		return nil, bizInvalid()
	}
	query := `SELECT preset_no,department_no,locale,title,body,status_code AS status,version FROM plugin_idc_core_support_reply_preset WHERE tenant_id=0 AND status_code='active' AND locale=?`
	args := []any{locale}
	if !a.Global {
		if len(a.Departments) == 0 {
			return []ReplyPresetOption{}, nil
		}
		query += ` AND department_no IN (` + sqlPlaceholders(len(a.Departments)) + `)`
		for _, departmentNo := range a.Departments {
			args = append(args, departmentNo)
		}
	}
	query += ` ORDER BY sort_order,id LIMIT 500`
	var rows []ReplyPresetOption
	if err = s.database.GetScan(ctx, &rows, query, args...); err != nil {
		return nil, wrapUnavailable(err, "load support reply presets")
	}
	return rows, nil
}

func (s *serviceImpl) AutomationPolicy(ctx context.Context) (*AutomationPolicy, error) {
	if _, err := s.staffActor(ctx, PermissionConfigure); err != nil {
		return nil, err
	}
	return s.loadAutomationPolicy(ctx)
}

func (s *serviceImpl) UpdateAutomationPolicy(ctx context.Context, input AutomationPolicyInput) (*AutomationPolicy, error) {
	a, err := s.staffActor(ctx, PermissionConfigure)
	if err != nil {
		return nil, err
	}
	if input.AutoCloseDays < 1 || input.AutoCloseDays > 365 || input.ReopenDays < 1 || input.ReopenDays > 365 || input.ExpectedVersion <= 0 {
		return nil, bizInvalid()
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now, nowErr := databaseNow(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		result, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_core_support_automation_policy SET auto_close_days=?,reopen_days=?,version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND policy_no='support-default' AND version=?`, input.AutoCloseDays, input.ReopenDays, a.UserID, now, input.ExpectedVersion)
		if updateErr != nil {
			return updateErr
		}
		if changed, _ := result.RowsAffected(); changed != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		return nil
	})
	if err != nil {
		return nil, wrapUnavailable(err, "update support automation policy")
	}
	return s.loadAutomationPolicy(ctx)
}

func (s *serviceImpl) loadAutomationPolicy(ctx context.Context) (*AutomationPolicy, error) {
	var rows []AutomationPolicy
	if err := s.database.GetScan(ctx, &rows, `SELECT auto_close_days,reopen_days,version FROM plugin_idc_core_support_automation_policy WHERE tenant_id=0 AND policy_no='support-default' LIMIT 1`); err != nil {
		return nil, wrapUnavailable(err, "load support automation policy")
	}
	if len(rows) != 1 {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return &rows[0], nil
}
