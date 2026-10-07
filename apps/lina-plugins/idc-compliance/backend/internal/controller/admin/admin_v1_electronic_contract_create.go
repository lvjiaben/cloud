// This file adapts manual optional contract preparation to the management API.

package admin

import (
	"context"
	"strconv"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/electroniccontractpreparation"
)

func (c *ControllerV1) ElectronicContractCreate(
	ctx context.Context,
	req *v1.ElectronicContractCreateReq,
) (*v1.ElectronicContractCreateRes, error) {
	if c == nil || c.electronicContractPreparation == nil || req == nil {
		return nil, bizerr.NewCode(electroniccontractpreparation.CodeUnavailable)
	}
	prepared, err := c.electronicContractPreparation.PrepareManualOptional(
		ctx,
		electroniccontractpreparation.ManualOptionalInput{
			OrderNo: req.OrderNo, TemplateNo: req.TemplateNo, SignerNo: req.SignerNo,
			IdempotencyKey: req.IdempotencyKey,
		},
	)
	if err != nil {
		return nil, err
	}
	defer prepared.Clear()
	item := mapElectronicContractPreparation(prepared)
	if item == nil {
		return nil, bizerr.NewCode(electroniccontractpreparation.CodeUnavailable)
	}
	return &v1.ElectronicContractCreateRes{Item: item}, nil
}

func mapElectronicContractPreparation(
	prepared *electroniccontractpreparation.PreparedContract,
) *v1.ElectronicContractPreparationItem {
	if prepared == nil || prepared.Preview == nil {
		return nil
	}
	preview := prepared.Preview
	item := &v1.ElectronicContractPreparationItem{
		ContractNo: prepared.ContractNo, OrderNo: prepared.OrderNo,
		CustomerType: string(prepared.CustomerType),
		CustomerName: string(preview.CustomerName), MerchantName: string(preview.MerchantName),
		SignerNo: prepared.SignerNo, SignerName: string(prepared.SignerName), SignerPhoneMasked: prepared.SignerPhoneMasked,
		TemplateNo: prepared.Template.TemplateNo, TemplateVersion: prepared.Template.VersionNo,
		TemplateName: prepared.Template.Name, LanguageCode: prepared.Template.LanguageCode,
		SigningOrderCode: prepared.SigningOrderCode, StatusCode: "pending_launch",
		TotalAmountMinor: strconv.FormatInt(prepared.AmountMinor, 10),
		CurrencyCode:     prepared.CurrencyCode, CreatedAt: prepared.CreatedAt.UnixMilli(),
		SigningExpiresAt:       prepared.SigningExpiresAt.UnixMilli(),
		RequiredForFulfillment: prepared.RequiredForFulfillment,
		SigningDate:            preview.SigningDate, SigningDateNoticeCode: preview.SigningDateNoticeCode,
		Lines:   make([]v1.ElectronicContractPreparationLineItem, 0, len(preview.Items)),
		Reused:  prepared.Reused,
		Version: prepared.Version,
	}
	for _, line := range preview.Items {
		item.Lines = append(item.Lines, v1.ElectronicContractPreparationLineItem{
			OrderItemNo: line.OrderItemNo, ServiceName: line.ServiceName,
			Quantity: line.Quantity, PurchaseCycles: line.PurchaseCycles,
			PeriodUnitCode: line.PeriodUnitCode, PeriodCount: line.PeriodCount,
			AmountMinor: strconv.FormatInt(line.AmountMinor, 10),
		})
	}
	return item
}
