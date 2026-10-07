// This file maps the bounded customer contract list service to the account API.

package account

import (
	"context"

	"lina-plugin-idc-compliance/backend/api/account/v1"
	"lina-plugin-idc-compliance/backend/internal/service/electroniccontractpreparation"
)

// CustomerContractList returns one bounded page owned by the authenticated customer.
func (c *ControllerV1) CustomerContractList(ctx context.Context, req *v1.CustomerContractListReq) (res *v1.CustomerContractListRes, err error) {
	page, err := c.contracts.ListCurrentContracts(ctx, electroniccontractpreparation.CustomerContractQuery{
		Cursor: req.Cursor, Limit: req.Limit,
	})
	if err != nil {
		return nil, err
	}
	items := make([]v1.CustomerContractItem, 0, len(page.Items))
	for index := range page.Items {
		items = append(items, *customerContractItem(&page.Items[index]))
	}
	return &v1.CustomerContractListRes{Items: items, NextCursor: page.NextCursor}, nil
}
