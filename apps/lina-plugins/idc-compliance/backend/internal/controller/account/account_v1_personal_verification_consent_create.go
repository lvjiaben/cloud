package account

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/account/v1"
	"lina-plugin-idc-compliance/backend/internal/service/personalverification"
	"lina-plugin-idc-core/backend/cap/identitycap"
)

func (c *ControllerV1) PersonalVerificationConsentCreate(ctx context.Context, req *v1.PersonalVerificationConsentCreateReq) (res *v1.PersonalVerificationConsentCreateRes, err error) {
	if req == nil || !req.Accepted {
		return nil, bizerr.NewCode(personalverification.CodeInvalid)
	}
	consent, err := c.identities.RecordPersonalConsent(ctx, identitycap.PersonalConsentInput{
		PolicyVersion: req.PolicyVersion, Locale: req.Locale, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.PersonalVerificationConsentCreateRes{
		ConsentNo: consent.ConsentNo, PolicyVersion: consent.PolicyVersion,
		OccurredAt: consent.OccurredAt.UTC().UnixMilli(), Reused: consent.Reused,
	}, nil
}
