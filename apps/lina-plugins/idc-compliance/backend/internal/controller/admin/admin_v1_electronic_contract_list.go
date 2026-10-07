// This file maps the bounded electronic-contract management directory.

package admin

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/electroniccontractpreparation"
)

// ElectronicContractList returns one secret-safe contract page with its authorized total.
func (c *ControllerV1) ElectronicContractList(
	ctx context.Context,
	req *v1.ElectronicContractListReq,
) (*v1.ElectronicContractListRes, error) {
	if c == nil || c.electronicContractPreparation == nil || req == nil {
		return nil, bizerr.NewCode(electroniccontractpreparation.CodeUnavailable)
	}
	page, err := c.electronicContractPreparation.ListManagementContracts(
		ctx,
		electroniccontractpreparation.ManagementContractQuery{
			StatusCode: req.StatusCode, OrderNo: req.OrderNo, CustomerNo: req.CustomerNo,
			Page: req.Page, PageSize: req.PageSize, Cursor: req.Cursor,
		},
	)
	if err != nil {
		return nil, err
	}
	items := make([]v1.ElectronicContractListItem, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, electronicContractListItem(item))
	}
	return &v1.ElectronicContractListRes{Items: items, Total: page.Total, NextCursor: page.NextCursor}, nil
}
