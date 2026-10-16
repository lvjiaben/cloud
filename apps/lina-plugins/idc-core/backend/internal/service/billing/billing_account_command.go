// This file owns the authenticated customer boundary for financial commands.
// Monetary facts remain derived by the existing invoice and transform services.

package billing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const accountFinanceSourcePrefix = "account_finance_"

// AccountBalancePaymentInput identifies one owned invoice and one raw,
// request-scoped idempotency key. Customer, currency, and amount are derived.
type AccountBalancePaymentInput struct {
	InvoiceNo      string
	IdempotencyKey string
}

// AccountBalancePayment is the customer-safe result of an atomic balance pay.
type AccountBalancePayment struct {
	TransactionNo          string
	InvoiceNo              string
	CurrencyCode           string
	StatusCode             string
	PaidAmountMinor        int64
	OutstandingAmountMinor int64
	BalanceMinor           int64
	AvailableBalanceMinor  int64
	Reused                 bool
}

// AccountCreditPayment is the customer-safe result of an atomic credit limit
// reservation and order settlement. It never represents credit as cash paid.
type AccountCreditPayment struct {
	SettlementNo      string
	InvoiceNo         string
	OrderNo           string
	CurrencyCode      string
	CreditAmountMinor int64
	StatusCode        string
	Reused            bool
}

// AccountInvoiceMergeInput identifies two or more current-customer invoices.
type AccountInvoiceMergeInput struct {
	InvoiceNos     []string
	IdempotencyKey string
}

// AccountInvoiceMergeDestination is one newly derived receivable header.
type AccountInvoiceMergeDestination struct {
	InvoiceNo              string
	StatusCode             string
	TotalAmountMinor       int64
	OutstandingAmountMinor int64
	PaymentDueAt           int64
}

// AccountInvoiceMerge is a customer-safe immutable transform projection.
type AccountInvoiceMerge struct {
	TransformNo         string
	CurrencyCode        string
	SourceInvoiceNos    []string
	DestinationInvoices []AccountInvoiceMergeDestination
	Reused              bool
}

// AccountFinanceCommandService exposes only ownership-safe customer commands.
type AccountFinanceCommandService interface {
	PayAccountInvoiceFromBalance(context.Context, AccountBalancePaymentInput) (*AccountBalancePayment, error)
	PayAccountInvoiceFromCredit(context.Context, string) (*AccountCreditPayment, error)
	MergeAccountInvoices(context.Context, AccountInvoiceMergeInput) (*AccountInvoiceMerge, error)
}

type accountFinanceCommandImpl struct {
	database    gdb.DB
	principal   principal.Reader
	invoices    InvoiceService
	transforms  InvoiceTransformService
	settlements FinancialSettlementService
	renewals    RenewalInvoicePayment
}

var _ AccountFinanceCommandService = (*accountFinanceCommandImpl)(nil)

// NewAccountFinanceCommandService composes authenticated ownership checks with
// the existing append-only invoice authorities.
func NewAccountFinanceCommandService(
	database gdb.DB,
	principalReader principal.Reader,
	settlements FinancialSettlementService,
) (AccountFinanceCommandService, error) {
	if isNilFinanceDirectoryDependency(database) ||
		isNilFinanceDirectoryDependency(principalReader) ||
		isNilFinanceDirectoryDependency(settlements) {
		return nil, gerror.New("account financial command requires database, principal, and settlement services")
	}
	invoices, err := NewInvoiceService(database)
	if err != nil {
		return nil, err
	}
	transforms, err := NewInvoiceTransformService(database)
	if err != nil {
		return nil, err
	}
	return &accountFinanceCommandImpl{
		database: database, principal: principalReader,
		invoices: invoices, transforms: transforms, settlements: settlements,
	}, nil
}

func (s *accountFinanceCommandImpl) PayAccountInvoiceFromBalance(
	ctx context.Context,
	input AccountBalancePaymentInput,
) (*AccountBalancePayment, error) {
	audience, err := s.currentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	invoiceNo, err := normalizeAccountBalancePayment(input)
	if err != nil {
		return nil, err
	}
	if s.renewals != nil {
		paid, handled, payErr := s.renewals.PayOwnedRenewalInvoice(ctx, input)
		if handled || payErr != nil {
			return paid, payErr
		}
	}
	keyDigest := accountFinanceIdempotencyDigest(audience.CustomerNo, input.IdempotencyKey)
	if err = s.requireOwnedInvoices(ctx, audience.CustomerNo, []string{invoiceNo}); err != nil {
		return nil, err
	}
	snapshotDigest, err := contentDigest(struct {
		CustomerNo, InvoiceNo, IdempotencyDigest string
	}{audience.CustomerNo, invoiceNo, keyDigest})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeDirectoryUnavailable)
	}
	paid, err := s.invoices.PayFromBalance(ctx, InvoiceBalancePaymentInput{
		InvoiceNo:            invoiceNo,
		SourceReference:      accountFinanceSourcePrefix + keyDigest,
		SourceSnapshotDigest: snapshotDigest,
	})
	if err != nil {
		return nil, err
	}
	if paid == nil || paid.Posting == nil || paid.Invoice == nil ||
		paid.Invoice.InvoiceNo != invoiceNo || paid.Invoice.CustomerNo != audience.CustomerNo ||
		paid.Posting.CustomerNo != audience.CustomerNo || paid.Posting.CurrencyCode != paid.Invoice.CurrencyCode {
		return nil, bizerr.NewCode(CodeDirectoryIntegrity)
	}
	if paid.Invoice.OrderNo != "" && paid.Invoice.Status == InvoiceStatusPaid {
		settled, settleErr := s.settlements.SettleOrder(ctx, paid.Invoice.OrderNo)
		if settleErr != nil {
			return nil, settleErr
		}
		if settled == nil || settled.InvoiceNo != paid.Invoice.InvoiceNo ||
			settled.Basis != SettlementBasisFundedInvoice {
			return nil, bizerr.NewCode(CodeDirectoryIntegrity)
		}
	}
	return &AccountBalancePayment{
		TransactionNo:          paid.Posting.TransactionNo,
		InvoiceNo:              paid.Invoice.InvoiceNo,
		CurrencyCode:           paid.Invoice.CurrencyCode,
		StatusCode:             string(paid.Invoice.Status),
		PaidAmountMinor:        paid.Invoice.PaidAmountMinor,
		OutstandingAmountMinor: paid.Invoice.OutstandingAmountMinor,
		BalanceMinor:           paid.Posting.BalanceMinor,
		AvailableBalanceMinor:  paid.Posting.AvailableBalanceMinor,
		Reused:                 paid.Reused,
	}, nil
}

// PayAccountInvoiceFromCredit re-checks ownership before handing the order to
// the atomic credit-settlement coordinator. Amount and credit account are not
// accepted from the customer request.
func (s *accountFinanceCommandImpl) PayAccountInvoiceFromCredit(
	ctx context.Context,
	invoiceNo string,
) (*AccountCreditPayment, error) {
	audience, err := s.currentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	invoiceNo = strings.TrimSpace(invoiceNo)
	if !validBusinessReference(invoiceNo, 40) {
		return nil, bizerr.NewCode(CodeDirectoryInvalid)
	}
	var invoice *struct {
		InvoiceNo    string  `orm:"invoice_no"`
		OrderNo      *string `orm:"order_no"`
		CurrencyCode string  `orm:"currency_code"`
		StatusCode   string  `orm:"status_code"`
	}
	if err = s.database.GetScan(ctx, &invoice, `
SELECT invoice_no, order_no, currency_code, status_code
FROM plugin_idc_core_billing_invoice
WHERE tenant_id = 0 AND invoice_no = ? AND customer_no = ?
  AND source_code = 'order'`, invoiceNo, audience.CustomerNo); err != nil {
		return nil, bizerr.WrapCode(err, CodeDirectoryUnavailable)
	}
	if invoice == nil || invoice.OrderNo == nil ||
		!validBusinessReference(*invoice.OrderNo, 40) ||
		(invoice.StatusCode != string(InvoiceStatusPendingPayment) &&
			invoice.StatusCode != string(InvoiceStatusCreditSettled)) {
		return nil, bizerr.NewCode(CodeDirectoryNotAccessible)
	}
	settled, err := s.settlements.SettleOrderWithCredit(ctx, CreditOrderSettlementInput{
		OrderNo: *invoice.OrderNo, ActorType: "customer", ActorID: audience.UserID,
	})
	if err != nil {
		return nil, err
	}
	if settled == nil || settled.InvoiceNo != invoice.InvoiceNo ||
		settled.CustomerNo != audience.CustomerNo ||
		settled.CurrencyCode != invoice.CurrencyCode ||
		settled.Basis != SettlementBasisCredit ||
		settled.CreditAmountMinor != settled.TotalAmountMinor {
		return nil, bizerr.NewCode(CodeDirectoryIntegrity)
	}
	return &AccountCreditPayment{
		SettlementNo: settled.SettlementNo, InvoiceNo: settled.InvoiceNo,
		OrderNo: settled.OrderNo, CurrencyCode: settled.CurrencyCode,
		CreditAmountMinor: settled.CreditAmountMinor,
		StatusCode:        string(InvoiceStatusCreditSettled), Reused: settled.Reused,
	}, nil
}

func (s *accountFinanceCommandImpl) MergeAccountInvoices(
	ctx context.Context,
	input AccountInvoiceMergeInput,
) (*AccountInvoiceMerge, error) {
	audience, err := s.currentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	command, err := normalizeInvoiceMerge(InvoiceMergeInput{
		InvoiceNos: input.InvoiceNos, IdempotencyKey: input.IdempotencyKey,
		RequestedBy: audience.UserID,
	})
	if err != nil {
		return nil, err
	}
	if err = s.requireOwnedInvoices(ctx, audience.CustomerNo, command.SourceInvoiceNos); err != nil {
		return nil, err
	}
	merged, err := s.transforms.MergeInvoices(ctx, InvoiceMergeInput{
		InvoiceNos: command.SourceInvoiceNos,
		IdempotencyKey: "account_merge_" + accountFinanceIdempotencyDigest(
			audience.CustomerNo, input.IdempotencyKey,
		),
		RequestedBy: audience.UserID,
	})
	if err != nil {
		return nil, err
	}
	if merged == nil || merged.CustomerNo != audience.CustomerNo ||
		len(merged.SourceInvoiceNos) != len(command.SourceInvoiceNos) ||
		len(merged.DestinationInvoices) < 1 {
		return nil, bizerr.NewCode(CodeDirectoryIntegrity)
	}
	destinations := make([]AccountInvoiceMergeDestination, 0, len(merged.DestinationInvoices))
	for _, invoice := range merged.DestinationInvoices {
		if invoice == nil || invoice.CustomerNo != audience.CustomerNo ||
			invoice.CurrencyCode != merged.CurrencyCode {
			return nil, bizerr.NewCode(CodeDirectoryIntegrity)
		}
		destinations = append(destinations, AccountInvoiceMergeDestination{
			InvoiceNo: invoice.InvoiceNo, StatusCode: string(invoice.Status),
			TotalAmountMinor:       invoice.TotalAmountMinor,
			OutstandingAmountMinor: invoice.OutstandingAmountMinor,
			PaymentDueAt:           invoice.PaymentDueAt.UnixMilli(),
		})
	}
	return &AccountInvoiceMerge{
		TransformNo: merged.TransformNo, CurrencyCode: merged.CurrencyCode,
		SourceInvoiceNos:    append([]string(nil), merged.SourceInvoiceNos...),
		DestinationInvoices: destinations, Reused: merged.Reused,
	}, nil
}

func (s *accountFinanceCommandImpl) currentCustomer(ctx context.Context) (*principal.Audience, error) {
	if s == nil {
		return nil, bizerr.NewCode(CodeDirectoryNotAccessible)
	}
	return currentFinancialCustomer(ctx, s.principal)
}

func normalizeAccountBalancePayment(input AccountBalancePaymentInput) (string, error) {
	if input.InvoiceNo != strings.TrimSpace(input.InvoiceNo) ||
		!validBusinessReference(input.InvoiceNo, 40) || !validInvoiceTransformKey(input.IdempotencyKey) {
		return "", bizerr.NewCode(CodeDirectoryInvalid)
	}
	return input.InvoiceNo, nil
}

func accountFinanceIdempotencyDigest(customerNo, rawKey string) string {
	digest := sha256.Sum256([]byte(customerNo + "\x00" + rawKey))
	return hex.EncodeToString(digest[:])
}

func (s *accountFinanceCommandImpl) requireOwnedInvoices(
	ctx context.Context,
	customerNo string,
	invoiceNos []string,
) error {
	if len(invoiceNos) < 1 || len(invoiceNos) > maximumInvoiceTransformInvoiceCount {
		return bizerr.NewCode(CodeDirectoryInvalid)
	}
	canonical := append([]string(nil), invoiceNos...)
	sort.Strings(canonical)
	columns := dao.BillingInvoice.Columns()
	count, err := s.database.Model(dao.BillingInvoice.Table()).Ctx(ctx).
		Where(columns.TenantId, 0).
		Where(columns.CustomerNo, customerNo).
		WhereIn(columns.InvoiceNo, canonical).
		Count()
	if err != nil {
		return bizerr.WrapCode(err, CodeDirectoryUnavailable)
	}
	if count != len(canonical) {
		return bizerr.NewCode(CodeDirectoryNotAccessible)
	}
	return nil
}

// RenewalInvoicePayment routes renewal invoices through their business owner so
// generic financial payment also advances the entitlement and queues recovery.
type RenewalInvoicePayment interface {
	// PayOwnedRenewalInvoice returns handled=false only for another invoice source.
	// Renewal failures never fall back to a financial-only debit.
	PayOwnedRenewalInvoice(context.Context, AccountBalancePaymentInput) (*AccountBalancePayment, bool, error)
}

// NewAccountFinanceCommandServiceWithRenewals keeps the same startup renewal
// instance for both the trial page and generic invoice payment entry points.
func NewAccountFinanceCommandServiceWithRenewals(database gdb.DB, principalReader principal.Reader,
	settlements FinancialSettlementService, renewals RenewalInvoicePayment) (AccountFinanceCommandService, error) {
	if renewals == nil {
		return nil, gerror.New("renewal invoice payment owner is required")
	}
	service, err := NewAccountFinanceCommandService(database, principalReader, settlements)
	if err != nil {
		return nil, err
	}
	impl, ok := service.(*accountFinanceCommandImpl)
	if !ok {
		return nil, gerror.New("unexpected financial command implementation")
	}
	impl.renewals = renewals
	return service, nil
}
