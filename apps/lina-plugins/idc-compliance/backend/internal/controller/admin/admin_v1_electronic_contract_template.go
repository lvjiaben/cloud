// This file adapts electronic-contract template services to management API DTOs.

package admin

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/electroniccontracttemplate"
)

func (c *ControllerV1) ElectronicContractTemplateList(
	ctx context.Context,
	req *v1.ElectronicContractTemplateListReq,
) (*v1.ElectronicContractTemplateListRes, error) {
	if c == nil || c.electronicContractTemplates == nil || req == nil {
		return nil, bizerr.NewCode(electroniccontracttemplate.CodeUnavailable)
	}
	result, err := c.electronicContractTemplates.List(ctx, electroniccontracttemplate.ListInput{
		Name: req.Name, ScopeCode: req.ScopeCode, StatusCode: req.StatusCode, PageSize: req.PageSize, Page: req.Page,
	})
	if err != nil {
		return nil, err
	}
	response := &v1.ElectronicContractTemplateListRes{
		Items: make([]v1.ElectronicContractTemplateItem, 0, len(result.Items)),
		Page:  result.Page, PageSize: result.PageSize, Total: result.Total,
	}
	for _, item := range result.Items {
		response.Items = append(response.Items, mapElectronicContractTemplate(item))
	}
	return response, nil
}

func (c *ControllerV1) ElectronicContractTemplateGet(
	ctx context.Context,
	req *v1.ElectronicContractTemplateGetReq,
) (*v1.ElectronicContractTemplateGetRes, error) {
	if c == nil || c.electronicContractTemplates == nil || req == nil {
		return nil, bizerr.NewCode(electroniccontracttemplate.CodeUnavailable)
	}
	item, err := c.electronicContractTemplates.Get(ctx, req.TemplateNo, req.VersionNo)
	if err != nil {
		return nil, err
	}
	mapped := mapElectronicContractTemplate(*item)
	return &v1.ElectronicContractTemplateGetRes{Item: &mapped}, nil
}

func (c *ControllerV1) ElectronicContractTemplateCreate(
	ctx context.Context,
	req *v1.ElectronicContractTemplateCreateReq,
) (*v1.ElectronicContractTemplateCreateRes, error) {
	if c == nil || c.electronicContractTemplates == nil || req == nil {
		return nil, bizerr.NewCode(electroniccontracttemplate.CodeUnavailable)
	}
	item, err := c.electronicContractTemplates.Create(ctx, electroniccontracttemplate.CreateInput{
		Name: req.Name, ScopeCode: req.ScopeCode, SigningOrderCode: req.SigningOrderCode,
		LanguageCode:   req.LanguageCode,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	mapped := mapElectronicContractTemplate(*item)
	return &v1.ElectronicContractTemplateCreateRes{Item: &mapped}, nil
}

func (c *ControllerV1) ElectronicContractTemplateCopyCreate(
	ctx context.Context,
	req *v1.ElectronicContractTemplateCopyCreateReq,
) (*v1.ElectronicContractTemplateCopyCreateRes, error) {
	if c == nil || c.electronicContractTemplates == nil || req == nil {
		return nil, bizerr.NewCode(electroniccontracttemplate.CodeUnavailable)
	}
	item, err := c.electronicContractTemplates.Copy(ctx, electroniccontracttemplate.CopyInput{
		SourceTemplateNo: req.TemplateNo, SourceVersionNo: req.VersionNo,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	mapped := mapElectronicContractTemplate(*item)
	return &v1.ElectronicContractTemplateCopyCreateRes{Item: &mapped}, nil
}

func (c *ControllerV1) ElectronicContractTemplateUpdate(
	ctx context.Context,
	req *v1.ElectronicContractTemplateUpdateReq,
) (*v1.ElectronicContractTemplateUpdateRes, error) {
	if c == nil || c.electronicContractTemplates == nil || req == nil {
		return nil, bizerr.NewCode(electroniccontracttemplate.CodeUnavailable)
	}
	item, err := c.electronicContractTemplates.Update(ctx, electroniccontracttemplate.UpdateInput{
		TemplateNo: req.TemplateNo, VersionNo: req.VersionNo, Name: req.Name,
		ScopeCode: req.ScopeCode, SigningOrderCode: req.SigningOrderCode,
		LanguageCode:    req.LanguageCode,
		ExpectedVersion: req.ExpectedVersion,
	})
	if err != nil {
		return nil, err
	}
	mapped := mapElectronicContractTemplate(*item)
	return &v1.ElectronicContractTemplateUpdateRes{Item: &mapped}, nil
}

func (c *ControllerV1) ElectronicContractTemplateEnablementCreate(
	ctx context.Context,
	req *v1.ElectronicContractTemplateEnablementCreateReq,
) (*v1.ElectronicContractTemplateEnablementCreateRes, error) {
	if c == nil || c.electronicContractTemplates == nil || req == nil {
		return nil, bizerr.NewCode(electroniccontracttemplate.CodeUnavailable)
	}
	item, err := c.electronicContractTemplates.Enable(ctx, electroniccontracttemplate.StateInput{
		TemplateNo: req.TemplateNo, VersionNo: req.VersionNo,
		ExpectedVersion: req.ExpectedVersion,
	})
	if err != nil {
		return nil, err
	}
	mapped := mapElectronicContractTemplate(*item)
	return &v1.ElectronicContractTemplateEnablementCreateRes{Item: &mapped}, nil
}

func (c *ControllerV1) ElectronicContractTemplateDisablementCreate(
	ctx context.Context,
	req *v1.ElectronicContractTemplateDisablementCreateReq,
) (*v1.ElectronicContractTemplateDisablementCreateRes, error) {
	if c == nil || c.electronicContractTemplates == nil || req == nil {
		return nil, bizerr.NewCode(electroniccontracttemplate.CodeUnavailable)
	}
	item, err := c.electronicContractTemplates.Disable(ctx, electroniccontracttemplate.StateInput{
		TemplateNo: req.TemplateNo, VersionNo: req.VersionNo,
		ExpectedVersion: req.ExpectedVersion,
	})
	if err != nil {
		return nil, err
	}
	mapped := mapElectronicContractTemplate(*item)
	return &v1.ElectronicContractTemplateDisablementCreateRes{Item: &mapped}, nil
}

func (c *ControllerV1) ElectronicContractTemplateTencentSyncCreate(
	ctx context.Context,
	_ *v1.ElectronicContractTemplateTencentSyncCreateReq,
) (*v1.ElectronicContractTemplateTencentSyncCreateRes, error) {
	if c == nil || c.electronicContractTemplates == nil {
		return nil, bizerr.NewCode(electroniccontracttemplate.CodeUnavailable)
	}
	result, err := c.electronicContractTemplates.SyncTencent(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.ElectronicContractTemplateTencentSyncCreateRes{
		Item: &v1.ElectronicContractTemplateSyncResult{
			ProviderTotal: result.ProviderTotal, NewVersions: result.NewVersions,
			MarkedOffline: result.MarkedOffline, SyncedAt: result.SyncedAt.UnixMilli(),
		},
	}, nil
}

func mapElectronicContractTemplate(
	item electroniccontracttemplate.Item,
) v1.ElectronicContractTemplateItem {
	return v1.ElectronicContractTemplateItem{
		TemplateNo: item.TemplateNo, VersionNo: item.VersionNo, Name: item.Name,
		ScopeCode: item.ScopeCode, SigningOrderCode: item.SigningOrderCode,
		LanguageCode: item.LanguageCode,
		StatusCode:   item.StatusCode, OriginCode: item.OriginCode,
		ProviderState: item.ProviderState, ProviderVersion: item.ProviderVersion,
		UsedCount: item.UsedCount, LockVersion: item.LockVersion,
		UpdatedAt: item.UpdatedAt.UnixMilli(),
	}
}
