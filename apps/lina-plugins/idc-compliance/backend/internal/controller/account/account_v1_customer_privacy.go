// Customer privacy controllers derive ownership from authentication and map safe results. They never accept a customer number from browser input.

package account

import (
	"context"

	"lina-core/pkg/bizerr"
	v1 "lina-plugin-idc-compliance/backend/api/account/v1"
	"lina-plugin-idc-compliance/backend/internal/service/privacygovernance"
	"lina-plugin-idc-core/backend/cap/identitycap"
)

// CustomerPrivacyGet returns only the current customer's consent decisions.
func (c *ControllerV1) CustomerPrivacyGet(
	ctx context.Context,
	_ *v1.CustomerPrivacyGetReq,
) (*v1.CustomerPrivacyGetRes, error) {
	overview, err := c.privacy.CurrentCustomerPrivacy(ctx)
	if err != nil {
		return nil, err
	}
	response := &v1.CustomerPrivacyGetRes{Consents: make([]v1.CustomerPrivacyConsent, 0, len(overview.Consents))}
	for _, consent := range overview.Consents {
		response.Consents = append(response.Consents, projectCustomerPrivacyConsent(consent))
	}
	return response, nil
}

// CustomerPrivacyConsentWithdrawCreate records a future-processing withdrawal.
func (c *ControllerV1) CustomerPrivacyConsentWithdrawCreate(
	ctx context.Context,
	req *v1.CustomerPrivacyConsentWithdrawCreateReq,
) (*v1.CustomerPrivacyConsentWithdrawCreateRes, error) {
	if req == nil {
		return nil, bizerr.NewCode(privacygovernance.CodeInvalid)
	}
	decision, err := c.privacy.WithdrawCurrentCustomerConsent(ctx, identitycap.PrivacyConsentWithdrawalInput{
		ConsentType:    identitycap.PrivacyConsentType(req.ConsentTypeCode),
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.CustomerPrivacyConsentWithdrawCreateRes{Consent: projectCustomerPrivacyConsent(*decision)}, nil
}

func projectCustomerPrivacyConsent(consent identitycap.PrivacyConsentDecision) v1.CustomerPrivacyConsent {
	return v1.CustomerPrivacyConsent{
		ConsentNo: consent.ConsentNo, ConsentTypeCode: string(consent.ConsentType),
		ActionCode: consent.ActionCode, DecidedAt: consent.OccurredAt.UTC().UnixMilli(),
	}
}
