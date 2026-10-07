// This file serves one authorized credit statement and its bounded details.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

// ManagementCreditStatementDetail returns one authorized statement and bounded details.
func (c *ControllerV1) ManagementCreditStatementDetail(ctx context.Context, req *v1.ManagementCreditStatementDetailReq) (res *v1.ManagementCreditStatementDetailRes, err error) {
	detail, err := c.credit.GetManagementCreditStatement(ctx, req.StatementNo, billing.CreditStatementLineDirectoryFilter{
		Cursor: req.LineCursor, Limit: req.LineLimit,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ManagementCreditStatementDetailRes{
		Detail: toAPIManagementCreditStatementDetail(detail),
	}, nil
}
