// This file opens the core-owned category-only enterprise legal-change workflow.

package account

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/account/v1"
	"lina-plugin-idc-compliance/backend/internal/service/enterpriseverification"
	"lina-plugin-idc-core/backend/cap/identitycap"
)

// EnterpriseLegalChangeCreate creates or replays one current-enterprise request.
func (c *ControllerV1) EnterpriseLegalChangeCreate(
	ctx context.Context,
	req *v1.EnterpriseLegalChangeCreateReq,
) (*v1.EnterpriseLegalChangeCreateRes, error) {
	if req == nil {
		return nil, bizerr.NewCode(enterpriseverification.CodeInvalid)
	}
	fields := make([]identitycap.EnterpriseLegalChangeField, 0, len(req.FieldCodes))
	for _, field := range req.FieldCodes {
		fields = append(fields, identitycap.EnterpriseLegalChangeField(field))
	}
	result, err := c.enterpriseAccount.RequestEnterpriseLegalChange(
		ctx,
		identitycap.EnterpriseLegalChangeInput{
			Fields: fields, Reason: identitycap.EnterpriseLegalChangeReason(req.ReasonCode),
			ExpectedProfileVersion: req.ExpectedProfileVersion,
			IdempotencyKey:         req.IdempotencyKey,
		},
	)
	if err != nil {
		return nil, err
	}
	return &v1.EnterpriseLegalChangeCreateRes{Item: enterpriseLegalChangeItem(result)}, nil
}
