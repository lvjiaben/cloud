package supportdesk

import (
	"context"

	"lina-plugin-staff-access/backend/cap/staffaccesscap"
)

const (
	staffCapabilityHandle         = "idc.support.handle"
	staffCapabilityLead           = "idc.support.lead"
	staffCapabilityInternalNotify = "idc.support.internal-notify"
)

func (s *serviceImpl) targetStaffEligible(ctx context.Context, userID int64, capabilityCode, departmentNo string) (bool, error) {
	if s.staffAccess == nil || userID <= 0 || departmentNo == "" {
		return false, nil
	}
	decision, err := s.staffAccess.Check(ctx, staffaccesscap.CheckRequest{UserID: userID, CapabilityCode: capabilityCode, ScopeType: staffaccesscap.ScopeDepartment, ScopeKey: departmentNo})
	if err != nil {
		return false, err
	}
	return decision != nil && decision.Eligible, nil
}

func (s *serviceImpl) targetDepartmentMember(ctx context.Context, userID int64, departmentNo string, leader bool) (bool, error) {
	query := `SELECT COUNT(1) FROM plugin_idc_core_support_department_member WHERE tenant_id=0 AND user_id=? AND department_no=? AND status_code='active'`
	args := []any{userID, departmentNo}
	if leader {
		query += ` AND role_code='leader'`
	}
	var count int
	if err := s.database.GetScan(ctx, &count, query, args...); err != nil {
		return false, err
	}
	return count == 1, nil
}

func (s *serviceImpl) targetStaffAllowed(ctx context.Context, userID int64, capabilityCode, departmentNo string, leader bool) (bool, error) {
	member, err := s.targetDepartmentMember(ctx, userID, departmentNo, leader)
	if err != nil || !member {
		return false, err
	}
	return s.targetStaffEligible(ctx, userID, capabilityCode, departmentNo)
}
