// This file maps the two owner-safe enterprise state projections to one Portal response.

package account

import (
	"context"

	"lina-plugin-idc-compliance/backend/api/account/v1"
	"lina-plugin-idc-core/backend/cap/identitycap"
)

// EnterpriseVerificationGet returns current core and compliance state without customer input.
func (c *ControllerV1) EnterpriseVerificationGet(
	ctx context.Context,
	_ *v1.EnterpriseVerificationGetReq,
) (*v1.EnterpriseVerificationGetRes, error) {
	profile, err := c.enterpriseAccount.CurrentEnterpriseAccount(ctx)
	if err != nil {
		return nil, err
	}
	currentCase, err := c.enterprise.Current(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.EnterpriseVerificationGetRes{Item: &v1.EnterpriseVerificationAccountStateItem{
		PolicyVersion:     identitycap.EnterpriseConsentPolicyVersion,
		Case:              enterpriseAccountCaseItem(currentCase),
		Profile:           enterpriseAccountProfileItem(profile),
		ActiveLegalChange: enterpriseLegalChangeItem(profile.ActiveLegalChange),
	}}, nil
}
