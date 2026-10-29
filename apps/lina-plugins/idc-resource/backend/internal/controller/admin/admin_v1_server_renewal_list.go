// This file serves the authorized local renewal candidate directory.

package admin

import (
	"context"

	"lina-plugin-idc-resource/backend/api/admin/v1"
	"lina-plugin-idc-resource/backend/internal/service/batchrenewal"
)

// ServerRenewalList exposes only customer-safe names and server identifiers.
func (c *ControllerV1) ServerRenewalList(ctx context.Context, req *v1.ServerRenewalListReq) (res *v1.ServerRenewalListRes, err error) {
	page, err := c.renewals.List(ctx, batchrenewal.ListInput{Cursor: req.Cursor, Limit: req.Limit, Page: req.Page, EntitlementNo: req.EntitlementNo})
	if err != nil {
		return nil, err
	}
	items := make([]*v1.ServerRenewalCandidate, len(page.Items))
	for index, item := range page.Items {
		items[index] = &v1.ServerRenewalCandidate{ServerNo: item.ServerNo, ProductNameZhCN: item.ProductNameZhCN,
			ProductNameEnUS: item.ProductNameEnUS}
	}
	return &v1.ServerRenewalListRes{Items: items, NextCursor: page.NextCursor, Total: page.Total}, nil
}
