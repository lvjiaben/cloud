// Privacy assessment draft controller maps validated administrator input to the governance service. It does not add business defaults or expose private data.

package admin

import (
	"context"

	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/privacygovernance"
)

func (c *ControllerV1) PrivacyAssessmentDraftUpdate(ctx context.Context, req *v1.PrivacyAssessmentDraftUpdateReq) (res *v1.PrivacyAssessmentDraftUpdateRes, err error) {
	item, err := c.privacyGovernance.SaveAssessmentDraft(ctx, privacygovernance.AssessmentDraftInput{
		Type:    privacygovernance.AssessmentType(req.AssessmentTypeCode),
		Purpose: req.Purpose, Necessity: req.Necessity, Impact: req.Impact,
		Recipients: req.Recipients, StorageLocation: req.StorageLocation,
		RetentionDescription: req.RetentionDescription, RightsEntry: req.RightsEntry,
		MerchantRole: req.MerchantRole, ThirdPartyRole: req.ThirdPartyRole,
		ProcessingInstruction:    req.ProcessingInstruction,
		SubcontractingBoundary:   req.SubcontractingBoundary,
		AuditEvidenceDescription: req.AuditEvidenceDescription,
		AIAllowedFields:          req.AIAllowedFields, ExpectedRowVersion: req.ExpectedRowVersion,
	})
	if err != nil {
		return nil, err
	}
	return &v1.PrivacyAssessmentDraftUpdateRes{Item: privacyAssessmentItem(item)}, nil
}
