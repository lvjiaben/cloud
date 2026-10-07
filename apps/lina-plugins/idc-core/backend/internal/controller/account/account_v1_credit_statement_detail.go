// This file serves one owned credit statement and its bounded details.

package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

// CreditStatementDetail returns one owned statement and bounded details.
func (c *ControllerV1) CreditStatementDetail(ctx context.Context, req *v1.CreditStatementDetailReq) (res *v1.CreditStatementDetailRes, err error) {
	detail, err := c.credit.GetAccountCreditStatement(ctx, req.StatementNo, billing.CreditStatementLineDirectoryFilter{
		Cursor: req.LineCursor, Limit: req.LineLimit,
	})
	if err != nil {
		return nil, err
	}
	return &v1.CreditStatementDetailRes{Detail: toAPICreditStatementDetail(detail)}, nil
}
