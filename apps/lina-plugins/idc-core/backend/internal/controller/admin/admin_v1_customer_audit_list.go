// Customer audit summaries check customer scope before the shared audit reader and omit sensitive snapshots.

package admin

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/audit"
	"lina-plugin-idc-core/backend/internal/service/customer"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) CustomerAuditList(ctx context.Context, req *v1.CustomerAuditListReq) (res *v1.CustomerAuditListRes, err error) {
	if c.customerAudit == nil {
		return nil, bizerr.NewCode(customer.CodeUnavailable)
	}
	// Customer query scope is checked before the independent exact audit permission.
	if _, err := c.customerManagement.GetOverview(ctx, req.CustomerNo); err != nil {
		return nil, err
	}
	result, err := c.customerAudit.Search(ctx, audit.SearchInput{CustomerNo: req.CustomerNo})
	if err != nil {
		return nil, err
	}
	items := make([]*v1.CustomerAuditItem, 0, len(result.Events))
	for _, event := range result.Events {
		items = append(items, &v1.CustomerAuditItem{AuditNo: event.AuditNo, OccurredAt: event.OccurredAt.UnixMilli(), OperationCode: event.OperationCode, Result: string(event.Result)})
	}
	return &v1.CustomerAuditListRes{Items: items}, nil
}
