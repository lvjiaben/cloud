package admin

import (
	"context"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/customermanagement"
)

func (c *ControllerV1) CustomerSegmentList(
	ctx context.Context,
	req *v1.CustomerSegmentListReq,
) (res *v1.CustomerSegmentListRes, err error) {
	page, err := c.customerManagement.SearchSegments(ctx, customermanagement.SegmentSearchInput{
		Kind: customermanagement.SegmentKind(req.Kind), Locale: customermanagement.SegmentLocale(req.Locale),
		NamePrefix: req.NamePrefix, Cursor: req.Cursor, Limit: req.Limit,
	})
	if err != nil {
		return nil, err
	}
	items := make([]*v1.CustomerSegmentItem, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, &v1.CustomerSegmentItem{
			Number: item.Number, Code: item.Code, NameZhCn: item.NameZhCn,
			NameEnUs: item.NameEnUs, ColorToken: item.ColorToken, Status: item.Status,
		})
	}
	return &v1.CustomerSegmentListRes{Items: items, NextCursor: page.NextCursor}, nil
}
