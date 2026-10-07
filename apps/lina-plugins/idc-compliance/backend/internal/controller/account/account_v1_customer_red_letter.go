// This file maps ownership-only customer red-letter endpoints to the service contract.

package account

import (
	"context"

	accountv1 "lina-plugin-idc-compliance/backend/api/account/v1"
	"lina-plugin-idc-compliance/backend/internal/service/taxinvoiceredletter"
)

func (c *ControllerV1) CustomerRedLetterTaskGet(
	ctx context.Context,
	req *accountv1.CustomerRedLetterTaskGetReq,
) (*accountv1.CustomerRedLetterTaskGetRes, error) {
	item, err := c.taxInvoiceRedLetters.GetCustomer(ctx, req.TaskNo)
	if err != nil {
		return nil, err
	}
	return &accountv1.CustomerRedLetterTaskGetRes{Item: mapCustomerRedLetter(item)}, nil
}

func (c *ControllerV1) CustomerRedLetterAuthoritativeStateUpdate(
	ctx context.Context,
	req *accountv1.CustomerRedLetterAuthoritativeStateUpdateReq,
) (*accountv1.CustomerRedLetterAuthoritativeStateUpdateRes, error) {
	item, err := c.taxInvoiceRedLetters.RecheckCustomer(ctx, taxinvoiceredletter.RecheckInput{
		TaskNo: req.TaskNo, ExpectedVersion: req.ExpectedVersion, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &accountv1.CustomerRedLetterAuthoritativeStateUpdateRes{Item: mapCustomerRedLetter(item)}, nil
}

func mapCustomerRedLetter(item *taxinvoiceredletter.Task) *accountv1.CustomerRedLetterTaskItem {
	if item == nil {
		return nil
	}
	result := &accountv1.CustomerRedLetterTaskItem{
		TaskNo: item.TaskNo, TaxInvoiceNo: item.TaxInvoiceNo,
		StatusCode: item.StatusCode, CustomerConfirmationRequired: item.CustomerConfirmationRequired,
		GrossAmountMinor: item.GrossAmountMinor, CurrencyCode: item.CurrencyCode,
		Version: item.Version, CreatedAt: item.CreatedAt.UnixMilli(),
		CustomerActionURL: item.CustomerActionURL,
	}
	if item.CompletedAt != nil {
		result.CompletedAt = item.CompletedAt.UnixMilli()
	}
	return result
}
