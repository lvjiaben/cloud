package customeradmin

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/customerclosure"
)

type ControllerV1 struct{ closures customerclosure.Service }

func NewV1(closures customerclosure.Service) (*ControllerV1, error) {
	if closures == nil {
		return nil, gerror.New("customer closure admin controller requires service")
	}
	return &ControllerV1{closures: closures}, nil
}

func (c *ControllerV1) ClosureComplete(ctx context.Context, req *v1.ClosureCompleteReq) (*v1.ClosureCompleteRes, error) {
	item, err := c.closures.Complete(ctx, customerclosure.CompleteInput{RequestNo: req.RequestNo, CustomerNo: req.CustomerNo, IdempotencyKey: req.IdempotencyKey})
	if err != nil {
		return nil, err
	}
	return &v1.ClosureCompleteRes{RequestNo: item.RequestNo, CustomerNo: item.CustomerNo, Status: string(item.Status), CompletedAt: item.CompletedAt.UnixMilli()}, nil
}
