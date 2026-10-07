// This file records the separate enterprise-verification consent decision.

package account

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/account/v1"
	"lina-plugin-idc-compliance/backend/internal/service/enterpriseverification"
	"lina-plugin-idc-core/backend/cap/identitycap"
)

// EnterpriseVerificationConsentCreate records or replays the accepted notice.
func (c *ControllerV1) EnterpriseVerificationConsentCreate(ctx context.Context, req *v1.EnterpriseVerificationConsentCreateReq) (res *v1.EnterpriseVerificationConsentCreateRes, err error) {
	if req == nil || !req.Accepted {
		return nil, bizerr.NewCode(enterpriseverification.CodeInvalid)
	}
	consent, err := c.identities.RecordEnterpriseConsent(ctx, identitycap.EnterpriseConsentInput{
		PolicyVersion: req.PolicyVersion, Locale: req.Locale, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.EnterpriseVerificationConsentCreateRes{
		ConsentNo: consent.ConsentNo, PolicyVersion: consent.PolicyVersion,
		OccurredAt: consent.OccurredAt.UTC().UnixMilli(), Reused: consent.Reused,
	}, nil
}
