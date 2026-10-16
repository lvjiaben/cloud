// This file checks current-actor queue membership and exact role permissions.
// It intentionally does not impersonate another employee to infer eligibility.

package supportdesk

import (
	"context"
	"lina-core/pkg/plugin/capability/authcap/authz"
)

func (s *serviceImpl) targetDepartmentMember(ctx context.Context, userID int64, departmentNo string, leader bool) (bool, error) {
	current := s.bizCtx.Current(ctx)
	if userID != int64(current.UserID) || departmentNo == "" {
		return false, nil
	}
	departments, err := s.actorDepartments(ctx, current.UserID)
	if err != nil || !contains(departments, departmentNo) {
		return false, err
	}
	if leader {
		return s.authorization.HasPermission(ctx, authz.PermissionKey(PermissionLead))
	}
	return true, nil
}

func (s *serviceImpl) targetStaffAllowed(ctx context.Context, userID int64, permission, departmentNo string, leader bool) (bool, error) {
	member, err := s.targetDepartmentMember(ctx, userID, departmentNo, leader)
	if err != nil || !member {
		return false, err
	}
	keys := []authz.PermissionKey{authz.PermissionKey(PermissionList), authz.PermissionKey(PermissionQuery), authz.PermissionKey(PermissionReply), authz.PermissionKey(permission)}
	grants, err := s.authorization.BatchHasPermissions(ctx, keys)
	if err != nil {
		return false, err
	}
	for _, key := range keys {
		if !grants[key] {
			return false, nil
		}
	}
	return true, nil
}
