package admin

import (
	"context"
	"strconv"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/commerce"
	"lina-plugin-idc-core/backend/internal/service/contractpolicy"
)

func (c *ControllerV1) CommerceAssistedQuoteCreate(ctx context.Context, req *v1.CommerceAssistedQuoteCreateReq) (res *v1.CommerceAssistedQuoteCreateRes, err error) {
	discount, err := strconv.ParseInt(req.ManualDiscountMinor, 10, 64)
	if err != nil {
		return nil, bizerr.NewCode(commerce.CodeAssistedQuoteInvalid)
	}
	items, err := toCommerceAssistedQuoteItems(req.Items, req.CurrencyCode)
	if err != nil {
		return nil, err
	}
	request, err := c.commerceAssistedQuotes.CreateAssistedQuote(ctx, commerce.AssistedQuoteCreateInput{
		CustomerNo: req.CustomerNo, CurrencyCode: req.CurrencyCode, ManualDiscountMinor: discount,
		PaymentDueAt: req.PaymentDueAt, Reason: req.Reason, InternalNote: req.InternalNote,
		ContractPolicyCode: contractpolicy.Policy(req.ContractPolicyCode),
		ContractTemplateNo: req.ContractTemplateNo,
		IdempotencyKey:     req.IdempotencyKey,
		Items:              items,
	})
	if err != nil {
		return nil, err
	}
	return &v1.CommerceAssistedQuoteCreateRes{Request: toAPICommerceAssistedQuote(request)}, nil
}
