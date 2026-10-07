// This file serves one authorized credit account management aggregate.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

// ManagementCreditAccountDetail returns one authorized credit account aggregate.
func (c *ControllerV1) ManagementCreditAccountDetail(ctx context.Context, req *v1.ManagementCreditAccountDetailReq) (res *v1.ManagementCreditAccountDetailRes, err error) {
	account, err := c.credit.GetManagementCreditAccount(ctx, req.CreditAccountNo)
	if err != nil {
		return nil, err
	}
	return &v1.ManagementCreditAccountDetailRes{Account: toAPIManagementCreditAccount(*account)}, nil
}
