// These controllers expose only identity-owned manual holds; tax and other owner protections remain intact.
package admin

import (
	"context"
	"lina-core/pkg/bizerr"
	v1 "lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/privacygovernance"
)

func (c *ControllerV1) PrivacyRetentionHoldCreate(ctx context.Context, req *v1.PrivacyRetentionHoldCreateReq) (*v1.PrivacyRetentionHoldCreateRes, error) {
	if req == nil {
		return nil, bizerr.NewCode(privacygovernance.CodeInvalid)
	}
	if err := c.privacyGovernance.PlaceRetentionHold(ctx, privacygovernance.RetentionHoldInput{TargetTypeCode: req.TargetTypeCode, TargetNo: req.TargetNo, ReasonCode: "legal_hold", OwnerCode: "identity.retention", IdempotencyKey: req.IdempotencyKey}); err != nil {
		return nil, err
	}
	return &v1.PrivacyRetentionHoldCreateRes{}, nil
}

func (c *ControllerV1) PrivacyRetentionHoldReleaseCreate(ctx context.Context, req *v1.PrivacyRetentionHoldReleaseCreateReq) (*v1.PrivacyRetentionHoldReleaseCreateRes, error) {
	if req == nil {
		return nil, bizerr.NewCode(privacygovernance.CodeInvalid)
	}
	if err := c.privacyGovernance.ReleaseRetentionHold(ctx, privacygovernance.RetentionHoldReleaseInput{TargetTypeCode: req.TargetTypeCode, TargetNo: req.TargetNo, ReasonCode: "legal_hold", OwnerCode: "identity.retention", IdempotencyKey: req.IdempotencyKey}); err != nil {
		return nil, err
	}
	return &v1.PrivacyRetentionHoldReleaseCreateRes{}, nil
}
