// This file binds support queues to live organization assignments without a
// second employee directory or role-name inference. Missing organization fails closed.

package supportdesk

import (
	"context"
	"strings"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/orgcap"
)

// OrganizationDepartmentPage is a bounded projection used by queue setup.
type OrganizationDepartmentPage struct {
	Items []*orgcap.DeptInfo
	Total int
}

// OrganizationDepartments applies role scope before returning department candidates.
func (s *serviceImpl) OrganizationDepartments(ctx context.Context, keyword string, page, pageSize int) (*OrganizationDepartmentPage, error) {
	_, err := s.staffActor(ctx, PermissionConfigure)
	if err != nil {
		return nil, err
	}
	if page < 1 || pageSize < 1 || pageSize > 100 {
		return nil, bizInvalid()
	}
	result := &OrganizationDepartmentPage{Items: []*orgcap.DeptInfo{}}
	if s.organization == nil || !s.organization.Available(ctx) {
		return result, nil
	}
	enabled := 1
	rows, err := s.organization.Department().List(ctx, orgcap.DeptListInput{Keyword: strings.TrimSpace(keyword), Status: &enabled, Page: capmodel.PageRequest{PageNum: page, PageSize: pageSize}})
	if err != nil {
		return nil, wrapUnavailable(err, "search organization departments")
	}
	if rows != nil {
		result.Items = rows.Items
		result.Total = rows.Total
	}
	return result, nil
}

// actorDepartments resolves only the current primary department; org hierarchy
// does not silently grant membership in sibling or child queues.
func (s *serviceImpl) actorDepartments(ctx context.Context, userID int) ([]string, error) {
	result := []string{}
	if s.organization == nil || !s.organization.Available(ctx) {
		return result, nil
	}
	profile, err := s.organization.Assignment().ListByUser(ctx, userID)
	if err != nil {
		return nil, wrapUnavailable(err, "load current organization membership")
	}
	if profile == nil || profile.DeptID <= 0 {
		return result, nil
	}
	dept, err := s.organization.Department().Get(ctx, profile.DeptID)
	if err != nil {
		return nil, wrapUnavailable(err, "load current department")
	}
	if dept == nil || dept.Status != 1 {
		return result, nil
	}
	var rows []struct {
		DepartmentNo string `orm:"department_no"`
	}
	if err = s.database.GetScan(ctx, &rows, `SELECT department_no FROM plugin_idc_core_support_department WHERE tenant_id=0 AND org_dept_id=? AND status_code='active' ORDER BY department_no`, profile.DeptID); err != nil {
		return nil, wrapUnavailable(err, "load organization support queues")
	}
	for _, row := range rows {
		result = append(result, row.DepartmentNo)
	}
	return result, nil
}

// requireOrganizationBinding validates the target within the caller's current data scope.
func (s *serviceImpl) requireOrganizationBinding(ctx context.Context, orgDeptID int) error {
	if orgDeptID <= 0 || s.organization == nil || !s.organization.Available(ctx) {
		return bizNotAccessible()
	}
	current := s.bizCtx.Current(ctx)
	if current.DataScope >= 3 {
		if current.DataScope == 4 {
			return bizNotAccessible()
		}
		profile, err := s.organization.Assignment().ListByUser(ctx, current.UserID)
		if err != nil {
			return wrapUnavailable(err, "check organization scope")
		}
		if profile == nil || profile.DeptID != orgDeptID {
			return bizNotAccessible()
		}
	}
	dept, err := s.organization.Department().Get(ctx, orgDeptID)
	if err != nil {
		return wrapUnavailable(err, "check organization department")
	}
	if dept == nil || dept.Status != 1 {
		return bizNotAccessible()
	}
	return nil
}

// requireActiveOrganizationDepartment rejects stale, unbound and disabled queues.
func (s *serviceImpl) requireActiveOrganizationDepartment(ctx context.Context, departmentNo string) error {
	var row struct {
		OrgDeptID int `orm:"org_dept_id"`
	}
	if err := s.database.GetScan(ctx, &row, `SELECT org_dept_id FROM plugin_idc_core_support_department WHERE tenant_id=0 AND department_no=? AND status_code='active'`, departmentNo); err != nil {
		return wrapUnavailable(err, "check support queue")
	}
	return s.requireOrganizationBinding(ctx, row.OrgDeptID)
}

// projectOrganizationNames batches display-only reads; queue authorization is
// decided before projection and never depends on a name.
func (s *serviceImpl) projectOrganizationNames(ctx context.Context, items []DepartmentOption) error {
	if s.organization == nil || !s.organization.Available(ctx) {
		return nil
	}
	ids := []int{}
	seen := map[int]bool{}
	for _, item := range items {
		if item.OrgDeptID > 0 && !seen[item.OrgDeptID] {
			ids = append(ids, item.OrgDeptID)
			seen[item.OrgDeptID] = true
		}
	}
	names := map[int]string{}
	for start := 0; start < len(ids); start += orgcap.MaxVisibilityCheckSize {
		end := min(start+orgcap.MaxVisibilityCheckSize, len(ids))
		rows, err := s.organization.Department().BatchGet(ctx, ids[start:end])
		if err != nil {
			return wrapUnavailable(err, "load queue department names")
		}
		if rows != nil {
			for id, dept := range rows.Items {
				if dept != nil {
					names[id] = dept.DeptName
				}
			}
		}
	}
	for i := range items {
		items[i].OrgDeptName = names[items[i].OrgDeptID]
	}
	return nil
}

// requireQueueConfigurationScope also permits a broad administrator to link an old unbound queue.
func (s *serviceImpl) requireQueueConfigurationScope(ctx context.Context, departmentNo string) error {
	current := s.bizCtx.Current(ctx)
	if current.DataScope <= 2 {
		return nil
	}
	var row struct {
		OrgDeptID int `orm:"org_dept_id"`
	}
	if err := s.database.GetScan(ctx, &row, `SELECT org_dept_id FROM plugin_idc_core_support_department WHERE tenant_id=0 AND department_no=?`, departmentNo); err != nil {
		return wrapUnavailable(err, "check queue configuration scope")
	}
	return s.requireOrganizationBinding(ctx, row.OrgDeptID)
}

// ManagedDepartments requires supervisor authority as well as ticket list access.
func (s *serviceImpl) ManagedDepartments(ctx context.Context) ([]DepartmentOption, error) {
	if _, err := s.staffActor(ctx, PermissionLead); err != nil {
		return nil, err
	}
	return s.Departments(ctx)
}
