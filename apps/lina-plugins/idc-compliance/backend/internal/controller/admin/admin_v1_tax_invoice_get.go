// This file returns one safe tax-invoice management detail.
package admin

import (
	"context"

	"lina-plugin-idc-compliance/backend/api/admin/v1"
)

// TaxInvoiceGet returns one safe staff projection.
func (c *ControllerV1) TaxInvoiceGet(
	ctx context.Context,
	req *v1.TaxInvoiceGetReq,
) (*v1.TaxInvoiceGetRes, error) {
	detail, err := c.taxInvoiceManagement.GetDetail(ctx, req.TaxInvoiceNo)
	if err != nil {
		return nil, err
	}
	res := &v1.TaxInvoiceGetRes{
		Item: taxInvoiceManagementItem(&detail.Request), HistoryTruncated: detail.HistoryTruncated,
		Buyer: v1.TaxInvoiceBuyerProfileItem{
			ProfileNo: detail.Buyer.ProfileNo, TitleTypeCode: detail.Buyer.TitleTypeCode,
			InvoiceTitle: detail.Buyer.InvoiceTitle, TaxRegistrationNo: detail.Buyer.TaxRegistrationNo,
			RegisteredAddress: detail.Buyer.RegisteredAddress, RegisteredPhone: detail.Buyer.RegisteredPhone,
			BankName: detail.Buyer.BankName, BankAccountMasked: detail.Buyer.BankAccountMasked,
			Version: detail.Buyer.Version,
		},
	}
	for _, value := range detail.Allocations {
		res.Allocations = append(res.Allocations, v1.TaxInvoiceAllocationItem{AllocationNo: value.AllocationNo, BillingInvoiceNo: value.BillingInvoiceNo, OrderNo: value.OrderNo, GrossAmountMinor: value.GrossAmountMinor, NetAmountMinor: value.NetAmountMinor, TaxAmountMinor: value.TaxAmountMinor, RefundedGrossMinor: value.RefundedGrossMinor, RedLetteredGrossMinor: value.RedLetteredGrossMinor})
	}
	for _, value := range detail.Items {
		res.Items = append(res.Items, v1.TaxInvoiceLineItem{ItemNo: value.ItemNo, AllocationNo: value.AllocationNo, BillingInvoiceItemNo: value.BillingInvoiceItemNo, LineNo: value.LineNo, TaxClassificationCode: value.TaxClassificationCode, TaxRatePPM: value.TaxRatePPM, GrossAmountMinor: value.GrossAmountMinor, NetAmountMinor: value.NetAmountMinor, TaxAmountMinor: value.TaxAmountMinor})
	}
	for _, value := range detail.Files {
		item := v1.TaxInvoiceFileItem{FileNo: value.FileNo, FileTypeCode: value.FileTypeCode, ContentSHA256: value.ContentSHA256, MediaType: value.MediaType, ByteSize: value.ByteSize, StatusCode: value.StatusCode, Immutable: value.Immutable, CreatedAt: value.CreatedAt.UnixMilli()}
		if value.ArchivedAt != nil {
			item.ArchivedAt = value.ArchivedAt.UnixMilli()
		}
		res.Files = append(res.Files, item)
	}
	return res, nil
}
