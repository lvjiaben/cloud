// This file maps one masked enterprise-verification case detail response.

package admin

import (
	"context"

	"lina-plugin-idc-compliance/backend/api/admin/v1"
)

// EnterpriseVerificationCaseGet returns one masked detail and bounded material batch.
func (c *ControllerV1) EnterpriseVerificationCaseGet(
	ctx context.Context,
	req *v1.EnterpriseVerificationCaseGetReq,
) (*v1.EnterpriseVerificationCaseGetRes, error) {
	detail, err := c.enterpriseVerification.GetCase(ctx, req.CaseNo)
	if err != nil {
		return nil, err
	}
	materials := make([]v1.EnterpriseVerificationMaterialItem, 0, len(detail.Materials))
	for _, material := range detail.Materials {
		materials = append(materials, v1.EnterpriseVerificationMaterialItem{
			MaterialNo: material.MaterialNo, MaterialType: string(material.MaterialType),
			SequenceNo: material.SequenceNo, MediaType: material.MediaType,
			ByteSize: material.ByteSize, StatusCode: material.StatusCode,
			RetentionUntil: milliseconds(material.RetentionUntil), LegalHold: material.LegalHold,
		})
	}
	profile := detail.LegalProfile
	return &v1.EnterpriseVerificationCaseGetRes{Item: &v1.EnterpriseVerificationCaseDetail{
		EnterpriseVerificationCaseItem: enterpriseCaseItem(detail.CaseListItem),
		LegalProfile: &v1.EnterpriseVerificationMaskedLegalProfile{
			LegalName:               profile.LegalName,
			RegistrationCountryCode: profile.RegistrationCountryCode,
			RegistrationIdentifier:  profile.RegistrationIdentifier,
			LegalRepresentativeName: profile.LegalRepresentativeName,
			LegalRepresentativeID:   profile.LegalRepresentativeID,
			AgentPresent:            profile.AgentPresent, AgentName: profile.AgentName,
			AgentIdentityNumber: profile.AgentIdentityNumber,
			IndustryCode:        profile.IndustryCode, ApplicantRoleCode: profile.ApplicantRoleCode,
		},
		Materials: materials, VerifiedAt: milliseconds(detail.VerifiedAt),
		RejectedAt: milliseconds(detail.RejectedAt), ExpiresAt: milliseconds(detail.ExpiresAt),
	}}, nil
}
