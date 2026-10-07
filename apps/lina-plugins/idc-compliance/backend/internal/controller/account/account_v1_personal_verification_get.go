package account

import (
	"context"

	"lina-plugin-idc-compliance/backend/api/account/v1"
	"lina-plugin-idc-core/backend/cap/identitycap"
)

func (c *ControllerV1) PersonalVerificationGet(ctx context.Context, req *v1.PersonalVerificationGetReq) (res *v1.PersonalVerificationGetRes, err error) {
	current, err := c.verification.Current(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.PersonalVerificationGetRes{
		Item:          personalVerificationItem(current),
		PolicyVersion: identitycap.PersonalConsentPolicyVersion,
	}, nil
}
