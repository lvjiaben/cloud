// This file maps internal review projections to public secret-safe DTOs.

package admin

import (
	"time"

	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/enterpriseverification"
)

// enterpriseCaseItem maps one internal safe row to the public DTO.
func enterpriseCaseItem(value enterpriseverification.CaseListItem) v1.EnterpriseVerificationCaseItem {
	return v1.EnterpriseVerificationCaseItem{
		CaseNo: value.CaseNo, CustomerNo: value.CustomerNo, StatusCode: value.StatusCode,
		MaskedLegalName:      value.MaskedLegalName,
		MaskedRegistrationID: value.MaskedRegistrationID,
		ApplicantRoleCode:    value.ApplicantRoleCode, Reverification: value.Reverification,
		DecisionReasonCode: value.DecisionReasonCode,
		SubmittedAt:        value.SubmittedAt.UnixMilli(), UpdatedAt: value.UpdatedAt.UnixMilli(),
		Version: value.Version,
	}
}

// enterpriseReviewResult maps one state transition without legal or storage values.
func enterpriseReviewResult(
	value *enterpriseverification.ReviewResult,
) *v1.EnterpriseVerificationReviewResult {
	if value == nil {
		return nil
	}
	return &v1.EnterpriseVerificationReviewResult{
		CaseNo: value.CaseNo, StatusCode: value.StatusCode,
		DecisionReasonCode: value.DecisionReasonCode,
		Version:            value.Version, Reused: value.Reused,
	}
}

// milliseconds converts an optional internal timestamp to the API time contract.
func milliseconds(value *time.Time) *int64 {
	if value == nil {
		return nil
	}
	result := value.UTC().UnixMilli()
	return &result
}
