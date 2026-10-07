// This file clears legal request fields before delegating enterprise submission.

package account

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/account/v1"
	"lina-plugin-idc-compliance/backend/internal/service/enterpriseverification"
)

// EnterpriseVerificationSubmit maps legal input into caller-clearable buffers.
func (c *ControllerV1) EnterpriseVerificationSubmit(ctx context.Context, req *v1.EnterpriseVerificationSubmitReq) (res *v1.EnterpriseVerificationSubmitRes, err error) {
	if req == nil {
		return nil, bizerr.NewCode(enterpriseverification.CodeInvalid)
	}
	var (
		legalName              = []byte(req.LegalName)
		registrationIdentifier = []byte(req.RegistrationIdentifier)
		representativeName     = []byte(req.LegalRepresentativeName)
		representativeIdentity = []byte(req.LegalRepresentativeIdentityNumber)
		agentName              = []byte(req.AgentName)
		agentIdentity          = []byte(req.AgentIdentityNumber)
	)
	req.LegalName = ""
	req.RegistrationIdentifier = ""
	req.LegalRepresentativeName = ""
	req.LegalRepresentativeIdentityNumber = ""
	req.AgentName = ""
	req.AgentIdentityNumber = ""
	item, err := c.enterprise.Submit(ctx, enterpriseverification.SubmitInput{
		CaseNo: req.CaseNo, LegalName: legalName,
		RegistrationCountryCode:           req.RegistrationCountryCode,
		RegistrationIdentifier:            registrationIdentifier,
		LegalRepresentativeName:           representativeName,
		LegalRepresentativeIdentityNumber: representativeIdentity,
		AgentName:                         agentName, AgentIdentityNumber: agentIdentity,
		IndustryCode:  req.IndustryCode,
		ApplicantRole: enterpriseverification.ApplicantRole(req.ApplicantRoleCode),
		MaterialNos:   req.MaterialNos, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.EnterpriseVerificationSubmitRes{Item: &v1.EnterpriseVerificationSubmissionItem{
		CaseNo: item.CaseNo, StatusCode: item.StatusCode,
		Reverification: item.Reverification, Reused: item.Reused,
	}}, nil
}
