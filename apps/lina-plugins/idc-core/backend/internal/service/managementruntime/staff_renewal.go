package managementruntime

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/internal/service/renewal"
)

type staffRenewalManager struct {
	service renewal.ManagementService
}

var _ managementcap.StaffRenewalManager = (*staffRenewalManager)(nil)

func (m *staffRenewalManager) QuoteForStaff(
	ctx context.Context,
	input managementcap.StaffRenewalQuoteInput,
) (*managementcap.StaffRenewalQuote, error) {
	if m == nil || ctx == nil || ctx.Err() != nil ||
		managementcap.ValidateStaffRenewalQuoteInput(input) != nil {
		return nil, gerror.New("IDC staff renewal quote service is unavailable")
	}
	service, err := m.serviceFor(ctx)
	if err != nil {
		return nil, err
	}
	quoted, err := service.QuoteForStaff(ctx, input.CustomerNo, renewal.QuoteInput{
		EntitlementNo: input.EntitlementNo, RenewalCycles: input.RenewalCycles,
		IdempotencyKey: input.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	if quoted == nil {
		return nil, gerror.New("IDC staff renewal quote is unavailable")
	}
	return &managementcap.StaffRenewalQuote{
		RequestNo: quoted.RequestNo, QuoteNo: quoted.QuoteNo,
		PriceSnapshotNo: quoted.PriceSnapshotNo, EntitlementNo: quoted.EntitlementNo,
		CurrencyCode: quoted.CurrencyCode, RenewalCycles: quoted.RenewalCycles,
		RenewalMode: quoted.RenewalMode, OriginalExpiresAt: quoted.OriginalExpiresAt,
		ProjectedExpiresAt: quoted.ProjectedExpiresAt, SubtotalMinor: quoted.SubtotalMinor,
		DiscountMinor: quoted.DiscountMinor, NetAmountMinor: quoted.NetAmountMinor,
		TaxAmountMinor: quoted.TaxAmountMinor, TotalAmountMinor: quoted.TotalAmountMinor,
		IssuedAt: quoted.IssuedAt, ExpiresAt: quoted.ExpiresAt, Reused: quoted.Reused,
	}, nil
}

func (m *staffRenewalManager) ConfirmForStaff(
	ctx context.Context,
	input managementcap.StaffRenewalConfirmInput,
) (*managementcap.StaffRenewalOperation, error) {
	if m == nil || ctx == nil || ctx.Err() != nil ||
		managementcap.ValidateStaffRenewalConfirmInput(input) != nil {
		return nil, gerror.New("IDC staff renewal confirmation service is unavailable")
	}
	service, err := m.serviceFor(ctx)
	if err != nil {
		return nil, err
	}
	confirmed, err := service.ConfirmForStaff(ctx, input.CustomerNo, renewal.ConfirmInput{
		QuoteNo: input.QuoteNo, IdempotencyKey: input.IdempotencyKey,
	}, input.ActorUserID)
	if err != nil {
		return nil, err
	}
	if confirmed == nil {
		return nil, gerror.New("IDC staff renewal confirmation is unavailable")
	}
	return &managementcap.StaffRenewalOperation{
		OperationNo: confirmed.OperationNo, QuoteNo: confirmed.QuoteNo,
		InvoiceNo: confirmed.InvoiceNo, EntitlementNo: confirmed.EntitlementNo,
		CurrencyCode: confirmed.CurrencyCode, Status: confirmed.Status,
		RenewalMode: confirmed.RenewalMode, TotalAmountMinor: confirmed.TotalAmountMinor,
		PaymentDueAt: confirmed.PaymentDueAt, Reused: confirmed.Reused,
	}, nil
}

func (m *staffRenewalManager) serviceFor(ctx context.Context) (renewal.ManagementService, error) {
	if m == nil || ctx == nil || ctx.Err() != nil || m.service == nil {
		return nil, gerror.New("IDC staff renewal service is unavailable")
	}
	return m.service, nil
}
