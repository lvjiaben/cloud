package admin

import (
	"context"
	"time"

	api "lina-plugin-staff-access/backend/api/admin"
	v1 "lina-plugin-staff-access/backend/api/admin/v1"
	"lina-plugin-staff-access/backend/internal/service/staffaccess"
)

type ControllerV1 struct{ service *staffaccess.Service }

func NewV1(service *staffaccess.Service) api.IAdminV1 { return &ControllerV1{service: service} }

func grantItem(item staffaccess.Grant) v1.GrantItem {
	result := v1.GrantItem{GrantNo: item.GrantNo, UserID: item.UserID, Username: item.Username, Nickname: item.Nickname, CapabilityCode: item.CapabilityCode, ScopeType: item.ScopeType, ScopeKey: item.ScopeKey, Status: item.Status, Version: item.Version, CreatedAt: item.CreatedAt.UnixMilli(), UpdatedAt: item.UpdatedAt.UnixMilli()}
	if item.ValidFrom != nil {
		value := item.ValidFrom.UnixMilli()
		result.ValidFrom = &value
	}
	if item.ValidUntil != nil {
		value := item.ValidUntil.UnixMilli()
		result.ValidUntil = &value
	}
	return result
}

func (c *ControllerV1) GrantList(ctx context.Context, req *v1.GrantListReq) (*v1.GrantListRes, error) {
	page, err := c.service.List(ctx, req.Keyword, req.CapabilityCode, req.Status, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	items := make([]v1.GrantItem, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, grantItem(item))
	}
	return &v1.GrantListRes{Items: items, Total: page.Total}, nil
}

func (c *ControllerV1) GrantSave(ctx context.Context, req *v1.GrantSaveReq) (*v1.GrantSaveRes, error) {
	input := staffaccess.SaveInput{UserID: req.UserID, CapabilityCode: req.CapabilityCode, ScopeType: req.ScopeType, ScopeKey: req.ScopeKey, Enabled: req.Enabled, Reason: req.Reason, ExpectedVersion: req.ExpectedVersion}
	if req.ValidFrom != nil {
		value := time.UnixMilli(*req.ValidFrom).UTC()
		input.ValidFrom = &value
	}
	if req.ValidUntil != nil {
		value := time.UnixMilli(*req.ValidUntil).UTC()
		input.ValidUntil = &value
	}
	item, err := c.service.Save(ctx, input)
	if err != nil {
		return nil, err
	}
	return &v1.GrantSaveRes{Item: grantItem(*item)}, nil
}

func (c *ControllerV1) UserCandidateList(ctx context.Context, req *v1.UserCandidateListReq) (*v1.UserCandidateListRes, error) {
	rows, total, err := c.service.Candidates(ctx, req.Keyword, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	items := make([]v1.UserCandidateItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, v1.UserCandidateItem{UserID: row.UserID, Username: row.Username, Nickname: row.Nickname})
	}
	return &v1.UserCandidateListRes{Items: items, Total: total}, nil
}
