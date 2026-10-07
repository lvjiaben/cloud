// This file serves the current customer's bounded credit application list.

package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

// CreditApplicationList returns one owned credit application page.
func (c *ControllerV1) CreditApplicationList(ctx context.Context, req *v1.CreditApplicationListReq) (res *v1.CreditApplicationListRes, err error) {
	page, err := c.credit.ListAccountCreditApplications(ctx, billing.CreditApplicationDirectoryFilter{
		StatusCode: req.StatusCode, CurrencyCode: req.CurrencyCode,
		Cursor: req.Cursor, Limit: req.Limit, Page: req.Page,
	})
	if err != nil {
		return nil, err
	}
	return &v1.CreditApplicationListRes{
		Applications: toAPICreditApplications(page.Applications), NextCursor: page.NextCursor, Total: page.Total,
	}, nil
}
