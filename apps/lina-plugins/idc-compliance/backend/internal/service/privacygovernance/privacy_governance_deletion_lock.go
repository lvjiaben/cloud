// Material operations lock their existing target before the optional task, including the first-scan insertion window.

package privacygovernance

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
)

// retentionTargetColumns resolves only the fixed local record types supported by material management.
func retentionTargetColumns(targetType string) ([2]string, bool) {
	targets := map[string][2]string{
		"personal_verification_case":       {"plugin_idc_compliance_verification_case", "case_no"},
		"enterprise_verification_case":     {"plugin_idc_compliance_verification_case", "case_no"},
		"enterprise_verification_material": {"plugin_idc_compliance_enterprise_material", "material_no"},
		"tax_invoice_file":                 {"plugin_idc_compliance_tax_invoice_file", "file_no"},
	}
	target, ok := targets[targetType]
	return target, ok
}

// lockDeletionTask uses the first read only to locate the immutable target and rereads task state after both locks.
func (s *serviceImpl) lockDeletionTask(ctx context.Context, taskNo string) (*deletionTaskRow, error) {
	var located []deletionTaskRow
	if err := s.database.GetScan(ctx, &located, `SELECT `+deletionTaskFields()+` FROM plugin_idc_compliance_privacy_deletion_task WHERE tenant_id = ? AND task_no = ?`, idccap.DefaultTenantID, taskNo); err != nil {
		return nil, err
	}
	if len(located) != 1 {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	target, ok := retentionTargetColumns(located[0].TargetTypeCode)
	if !ok {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	id, err := s.database.GetValue(ctx, `SELECT id FROM `+target[0]+` WHERE tenant_id = ? AND `+target[1]+` = ? FOR UPDATE`, idccap.DefaultTenantID, located[0].TargetNo)
	if err != nil {
		return nil, err
	}
	if id.Int64() <= 0 {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	var locked []deletionTaskRow
	if err = s.database.GetScan(ctx, &locked, `SELECT `+deletionTaskFields()+` FROM plugin_idc_compliance_privacy_deletion_task WHERE tenant_id = ? AND task_no = ? FOR UPDATE`, idccap.DefaultTenantID, taskNo); err != nil {
		return nil, err
	}
	if len(locked) != 1 || locked[0].TargetTypeCode != located[0].TargetTypeCode || locked[0].TargetNo != located[0].TargetNo {
		return nil, bizerr.NewCode(CodeConflict)
	}
	return &locked[0], nil
}
