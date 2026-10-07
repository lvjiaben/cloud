package managementruntime

import (
	"context"
	"sync"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/internal/runtimeconfig"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/principal"
	"lina-plugin-idc-core/backend/internal/service/renewal"
)

type staffRenewalManager struct {
	mutex     sync.Mutex
	principal principal.Reader
	service   renewal.ManagementService
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
	if m == nil {
		return nil, gerror.New("IDC staff renewal service is unavailable")
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.service != nil {
		return m.service, nil
	}
	if m.principal == nil {
		return nil, gerror.New("IDC staff renewal principal is unavailable")
	}
	cursorKey, err := runtimeconfig.LoadCatalogCursorKey()
	if err != nil {
		return nil, err
	}
	catalogReader, err := catalog.NewPublicCatalogReader(g.DB(), cursorKey)
	clear(cursorKey)
	if err != nil {
		return nil, err
	}
	service, err := renewal.New(ctx, g.DB(), m.principal, catalogReader)
	if err != nil {
		return nil, err
	}
	m.service = service
	return service, nil
}
