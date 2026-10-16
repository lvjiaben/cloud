// Customer segment additions delegate scoped, transactional assignment to the customer owner.

package admin

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/customer"
	"lina-plugin-idc-core/backend/internal/service/customermanagement"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) CustomerSegmentAdd(ctx context.Context, req *v1.CustomerSegmentAddReq) (res *v1.CustomerSegmentAddRes, err error) {
	service, ok := c.customerMutation.(customermanagement.SegmentMutationService)
	if !ok {
		return nil, bizerr.NewCode(customer.CodeUnavailable)
	}
	input := customermanagement.SegmentMutationInput{CustomerNo: req.CustomerNo, Kind: customermanagement.SegmentKind(req.Kind), SegmentNo: req.SegmentNo, IdempotencyKey: req.IdempotencyKey}
	input.Name = req.Name
	number, err := service.AddSegment(ctx, input)
	if err != nil {
		return nil, err
	}
	return &v1.CustomerSegmentAddRes{SegmentNo: number}, nil
}
