// Customer segment removals revoke only the scoped assignment and preserve shared definitions.

package admin

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/customer"
	"lina-plugin-idc-core/backend/internal/service/customermanagement"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) CustomerSegmentRemove(ctx context.Context, req *v1.CustomerSegmentRemoveReq) (res *v1.CustomerSegmentRemoveRes, err error) {
	service, ok := c.customerMutation.(customermanagement.SegmentMutationService)
	if !ok {
		return nil, bizerr.NewCode(customer.CodeUnavailable)
	}
	input := customermanagement.SegmentMutationInput{CustomerNo: req.CustomerNo, Kind: customermanagement.SegmentKind(req.Kind), SegmentNo: req.SegmentNo, IdempotencyKey: req.IdempotencyKey}
	if err := service.RemoveSegment(ctx, input); err != nil {
		return nil, err
	}
	return &v1.CustomerSegmentRemoveRes{}, nil
}
