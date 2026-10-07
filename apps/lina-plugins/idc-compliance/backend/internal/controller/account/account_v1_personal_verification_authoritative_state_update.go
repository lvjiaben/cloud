package account

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/account/v1"
	"lina-plugin-idc-compliance/backend/internal/service/personalverification"
)

func (c *ControllerV1) PersonalVerificationAuthoritativeStateUpdate(ctx context.Context, req *v1.PersonalVerificationAuthoritativeStateUpdateReq) (res *v1.PersonalVerificationAuthoritativeStateUpdateRes, err error) {
	if req == nil {
		return nil, bizerr.NewCode(personalverification.CodeInvalid)
	}
	refreshed, err := c.verification.QueryCurrent(ctx, req.AttemptNo)
	if err != nil {
		return nil, err
	}
	return &v1.PersonalVerificationAuthoritativeStateUpdateRes{
		Item: personalVerificationItem(refreshed),
	}, nil
}
