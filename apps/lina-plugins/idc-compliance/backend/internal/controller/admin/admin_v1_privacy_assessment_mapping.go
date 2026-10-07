// Privacy assessment mapping converts service models to API-safe timestamps and fields. It must not copy private implementation state into responses.

package admin

import (
	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/privacygovernance"
)

func privacyAssessmentItem(item *privacygovernance.Assessment) v1.PrivacyAssessmentItem {
	if item == nil {
		return v1.PrivacyAssessmentItem{}
	}
	result := v1.PrivacyAssessmentItem{
		AssessmentNo: item.AssessmentNo, AssessmentTypeCode: string(item.Type),
		Version: item.Version, StatusCode: string(item.Status), Purpose: item.Purpose,
		Necessity: item.Necessity, Impact: item.Impact, Recipients: item.Recipients,
		StorageLocation: item.StorageLocation, RetentionDescription: item.RetentionDescription,
		RightsEntry: item.RightsEntry, MerchantRole: item.MerchantRole,
		ThirdPartyRole: item.ThirdPartyRole, ProcessingInstruction: item.ProcessingInstruction,
		SubcontractingBoundary:   item.SubcontractingBoundary,
		AuditEvidenceDescription: item.AuditEvidenceDescription,
		AIAllowedFields:          append([]string(nil), item.AIAllowedFields...),
		CrossBorderTransfer:      item.CrossBorderTransfer,
		ConfirmedByUserID:        item.ConfirmedByUserID,
		CreatedAt:                item.CreatedAt.UnixMilli(), UpdatedAt: item.UpdatedAt.UnixMilli(),
		RowVersion: item.RowVersion,
	}
	if item.ConfirmedAt != nil {
		result.ConfirmedAt = item.ConfirmedAt.UnixMilli()
	}
	return result
}
