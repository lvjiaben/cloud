// This file serves the authorized credit application management list.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

// ManagementCreditApplicationList returns one authorized application page.
func (c *ControllerV1) ManagementCreditApplicationList(ctx context.Context, req *v1.ManagementCreditApplicationListReq) (res *v1.ManagementCreditApplicationListRes, err error) {
	page, err := c.credit.ListManagementCreditApplications(ctx, billing.CreditApplicationDirectoryFilter{
		CustomerNo: req.CustomerNo, StatusCode: req.StatusCode,
		CurrencyCode: req.CurrencyCode, Cursor: req.Cursor, Limit: req.Limit, Page: req.Page,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ManagementCreditApplicationListRes{
		Applications: toAPIManagementCreditApplications(page.Applications), NextCursor: page.NextCursor, Total: page.Total,
	}, nil
}
