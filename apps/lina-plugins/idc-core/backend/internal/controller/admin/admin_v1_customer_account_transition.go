package admin

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/customer"
)

func (c *ControllerV1) CustomerAccountTransition(
	ctx context.Context,
	req *v1.CustomerAccountTransitionReq,
) (*v1.CustomerAccountTransitionRes, error) {
	if c == nil || c.customerAccountLifecycle == nil {
		return nil, gerror.New("customer account lifecycle service is unavailable")
	}
	result, err := c.customerAccountLifecycle.TransitionAccount(ctx, customer.TransitionAccountInput{
		CustomerNo: req.CustomerNo, ToStatus: customer.Status(req.ToStatus),
		Reason: customer.AccountTransitionReason(req.Reason), ExpectedVersion: req.ExpectedVersion,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.CustomerAccountTransitionRes{
		CustomerNo: result.CustomerNo, FromStatus: string(result.FromStatus), ToStatus: string(result.ToStatus),
		Reason: string(result.Reason), SubjectVersion: result.SubjectVersion, OccurredAt: result.OccurredAt.UnixMilli(),
	}, nil
}
