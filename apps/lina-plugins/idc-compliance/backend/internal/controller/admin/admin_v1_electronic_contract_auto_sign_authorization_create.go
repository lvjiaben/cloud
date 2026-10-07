package admin

import (
	"context"
	"strconv"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/contractautosign"
)

func (c *ControllerV1) ElectronicContractAutoSignAuthorizationCreate(ctx context.Context, req *v1.ElectronicContractAutoSignAuthorizationCreateReq) (res *v1.ElectronicContractAutoSignAuthorizationCreateRes, err error) {
	amount, err := strconv.ParseInt(req.MaxAmountMinor, 10, 64)
	if err != nil {
		return nil, bizerr.NewCode(contractautosign.CodeInvalid)
	}
	templates := make([]contractautosign.TemplateRef, 0, len(req.Templates))
	for _, ref := range req.Templates {
		templates = append(templates, contractautosign.TemplateRef{TemplateNo: ref.TemplateNo, VersionNo: ref.VersionNo})
	}
	item, err := c.contractAutoSign.Create(ctx, contractautosign.CreateInput{
		CurrencyCode: req.CurrencyCode, AllowRequiredOrder: req.AllowRequiredOrder,
		AllowManualOptional: req.AllowManualOptional, MaxAmountMinor: amount,
		SuccessLimit: req.SuccessLimit, ExpiresAt: time.UnixMilli(req.ExpiresAt),
		Templates: templates, Confirmed: req.Confirmed, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ElectronicContractAutoSignAuthorizationCreateRes{Item: mapAutoSignAuthorization(*item)}, nil
}

func mapAutoSignAuthorization(item contractautosign.Item) v1.ElectronicContractAutoSignAuthorizationItem {
	result := v1.ElectronicContractAutoSignAuthorizationItem{
		AuthorizationNo: item.AuthorizationNo, CurrencyCode: item.CurrencyCode,
		AllowRequiredOrder: item.AllowRequiredOrder, AllowManualOptional: item.AllowManualOptional,
		MaxAmountMinor: strconv.FormatInt(item.MaxAmountMinor, 10), SuccessLimit: item.SuccessLimit,
		SuccessCount: item.SuccessCount, StatusCode: item.StatusCode, ExpiresAt: item.ExpiresAt.UnixMilli(),
		CreatedByUserID: item.CreatedByUserID, RevokedByUserID: item.RevokedByUserID,
		Version: item.Version, CreatedAt: item.CreatedAt.UnixMilli(),
		Templates: make([]v1.ElectronicContractAutoSignTemplateRef, 0, len(item.Templates)),
	}
	if item.RevokedAt != nil {
		value := item.RevokedAt.UnixMilli()
		result.RevokedAt = &value
	}
	for _, ref := range item.Templates {
		result.Templates = append(result.Templates, v1.ElectronicContractAutoSignTemplateRef{
			TemplateNo: ref.TemplateNo, VersionNo: ref.VersionNo, Name: ref.Name,
		})
	}
	return result
}
