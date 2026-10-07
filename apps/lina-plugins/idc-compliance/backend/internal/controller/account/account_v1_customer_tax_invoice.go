// This file exposes supplier-neutral tax-invoice requests to the authenticated customer.

package account

import (
	"context"
	"strconv"
	"time"

	"lina-core/pkg/bizerr"
	v1 "lina-plugin-idc-compliance/backend/api/account/v1"
	"lina-plugin-idc-compliance/backend/internal/service/taxinvoicerequest"
	"lina-plugin-idc-core/backend/cap/taxinvoicecap"
)

// CustomerTaxInvoiceOptionsGet returns current server-owned picker data.
func (c *ControllerV1) CustomerTaxInvoiceOptionsGet(
	ctx context.Context,
	_ *v1.CustomerTaxInvoiceOptionsGetReq,
) (*v1.CustomerTaxInvoiceOptionsGetRes, error) {
	options, err := c.taxInvoiceRequests.ListCurrentCustomerOptions(ctx)
	if err != nil {
		return nil, err
	}
	response := &v1.CustomerTaxInvoiceOptionsGetRes{
		Profiles:   make([]v1.CustomerTaxInvoiceProfile, 0, len(options.Profiles)),
		Invoices:   make([]v1.CustomerTaxInvoiceSource, 0, len(options.Invoices)),
		ObservedAt: options.ObservedAt.UTC().UnixMilli(),
	}
	for _, profile := range options.Profiles {
		response.Profiles = append(response.Profiles, v1.CustomerTaxInvoiceProfile{
			ProfileNo: profile.ProfileNo, TitleTypeCode: profile.TitleTypeCode,
			InvoiceTitle: profile.InvoiceTitle, TaxRegistrationNo: profile.TaxRegistrationNo,
			RegisteredAddress: profile.RegisteredAddress, RegisteredPhone: profile.RegisteredPhone,
			BankName: profile.BankName, BankAccountMasked: profile.BankAccountMasked,
			IsDefault: profile.IsDefault, Version: profile.Version,
		})
	}
	for _, invoice := range options.Invoices {
		response.Invoices = append(response.Invoices, v1.CustomerTaxInvoiceSource{
			BillingInvoiceNo: invoice.BillingInvoiceNo, OrderNo: invoice.OrderNo,
			CurrencyCode:          invoice.CurrencyCode,
			SourceGrossMinor:      strconv.FormatInt(invoice.SourceGrossMinor, 10),
			AlreadyAllocatedMinor: strconv.FormatInt(invoice.AlreadyAllocatedMinor, 10),
			RemainingGrossMinor:   strconv.FormatInt(invoice.RemainingGrossMinor, 10),
			SettledAt:             invoice.SettledAt.UTC().UnixMilli(),
		})
	}
	return response, nil
}

// CustomerTaxInvoiceCreate creates or recovers one immutable request.
func (c *ControllerV1) CustomerTaxInvoiceCreate(
	ctx context.Context,
	req *v1.CustomerTaxInvoiceCreateReq,
) (*v1.CustomerTaxInvoiceCreateRes, error) {
	if req == nil {
		return nil, bizerr.NewCode(taxinvoicerequest.CodeInvalid)
	}
	selections := make([]taxinvoicecap.Selection, 0, len(req.Selections))
	for _, selection := range req.Selections {
		amount, err := strconv.ParseInt(selection.RequestedGrossMinor, 10, 64)
		if err != nil || amount <= 0 {
			return nil, bizerr.NewCode(taxinvoicerequest.CodeInvalid)
		}
		selections = append(selections, taxinvoicecap.Selection{
			BillingInvoiceNo:    selection.BillingInvoiceNo,
			RequestedGrossMinor: amount,
		})
	}
	defer clear(selections)
	created, err := c.taxInvoiceRequests.CreateCurrentCustomer(ctx, taxinvoicerequest.CreateInput{
		ProfileNo: req.ProfileNo, Selections: selections, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.CustomerTaxInvoiceCreateRes{
		Item: v1.CustomerTaxInvoiceItem{
			TaxInvoiceNo: created.TaxInvoiceNo, StatusCode: created.StatusCode,
			InvoiceTypeCode: created.InvoiceTypeCode, CurrencyCode: created.CurrencyCode,
			GrossAmountMinor: strconv.FormatInt(created.GrossAmountMinor, 10),
			SubmittedAt:      created.SubmittedAt.UTC().UnixMilli(), Version: created.Version,
		},
		Reused: created.Reused,
	}, nil
}

// CustomerTaxInvoiceList returns one bounded customer-owned history page.
func (c *ControllerV1) CustomerTaxInvoiceList(
	ctx context.Context,
	req *v1.CustomerTaxInvoiceListReq,
) (*v1.CustomerTaxInvoiceListRes, error) {
	if req == nil {
		return nil, bizerr.NewCode(taxinvoicerequest.CodeInvalid)
	}
	page, err := c.taxInvoiceRequests.ListCurrentCustomerRequests(ctx, taxinvoicerequest.CurrentCustomerQuery{
		Cursor: req.Cursor, Limit: req.Limit,
	})
	if err != nil {
		return nil, err
	}
	response := &v1.CustomerTaxInvoiceListRes{
		Items:      make([]v1.CustomerTaxInvoiceItem, 0, len(page.Items)),
		NextCursor: page.NextCursor,
	}
	for index := range page.Items {
		response.Items = append(response.Items, projectCustomerTaxInvoice(&page.Items[index]))
	}
	return response, nil
}

func projectCustomerTaxInvoice(item *taxinvoicerequest.CurrentCustomerRequest) v1.CustomerTaxInvoiceItem {
	projected := v1.CustomerTaxInvoiceItem{
		TaxInvoiceNo: item.TaxInvoiceNo, StatusCode: item.StatusCode,
		InvoiceTypeCode: item.InvoiceTypeCode, CurrencyCode: item.CurrencyCode,
		GrossAmountMinor: strconv.FormatInt(item.GrossAmountMinor, 10),
		MaskedInvoiceNo:  item.MaskedInvoiceNo, DecisionReasonCode: item.DecisionReasonCode,
		SubmittedAt: item.SubmittedAt.UTC().UnixMilli(), Version: item.Version,
	}
	projected.IssuedAt = unixMillis(item.IssuedAt)
	projected.DeliveredAt = unixMillis(item.DeliveredAt)
	return projected
}

func unixMillis(value *time.Time) int64 {
	if value == nil || value.IsZero() {
		return 0
	}
	return value.UTC().UnixMilli()
}
