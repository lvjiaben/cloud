package admin

import (
	"context"

	"lina-plugin-idc-portal/backend/api/admin/v1"
	"lina-plugin-idc-portal/backend/internal/service/externalmenu"
)

func (c *ControllerV1) ExternalMenuList(ctx context.Context, _ *v1.ExternalMenuListReq) (*v1.ExternalMenuListRes, error) {
	items, err := c.externalMenus.List(ctx, false)
	return &v1.ExternalMenuListRes{Items: items}, err
}

func (c *ControllerV1) ExternalMenuSave(ctx context.Context, req *v1.ExternalMenuSaveReq) (*v1.ExternalMenuSaveRes, error) {
	item, err := c.externalMenus.Save(ctx, externalmenu.SaveInput{
		ID: req.ID, NameZh: req.NameZh, NameEn: req.NameEn, Icon: req.Icon,
		URL: req.URL, SortOrder: req.SortOrder, Version: req.Version,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ExternalMenuSaveRes{Item: *item}, nil
}

func (c *ControllerV1) ExternalMenuStatus(ctx context.Context, req *v1.ExternalMenuStatusReq) (*v1.ExternalMenuStatusRes, error) {
	item, err := c.externalMenus.SetEnabled(ctx, req.ID, req.Version, req.Enabled)
	if err != nil {
		return nil, err
	}
	return &v1.ExternalMenuStatusRes{Item: *item}, nil
}
