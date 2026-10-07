package admin

import (
	"context"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/customermanagement"
)

// CustomerTagBatch maps the bounded HTTP contract to the dedicated mutation service.
func (c *ControllerV1) CustomerTagBatch(
	ctx context.Context,
	req *v1.CustomerTagBatchReq,
) (res *v1.CustomerTagBatchRes, err error) {
	result, err := c.customerMutation.BatchUpdateTags(ctx, customermanagement.TagBatchInput{
		Action:         customermanagement.TagBatchAction(req.Action),
		CustomerNos:    req.CustomerNos,
		TagNos:         req.TagNos,
		Reason:         req.Reason,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.CustomerTagBatchRes{
		AuditNo: result.AuditNo, Replayed: result.Replayed,
		CustomerCount: result.CustomerCount, TagCount: result.TagCount,
	}, nil
}
