// This file exposes completed cancellation facts without exposing commerce models.

package commerce

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

// cancellationOrderStatus is the order owner's local closure state.
type cancellationOrderStatus string

const cancellationOrderClosed cancellationOrderStatus = "cancelled"

// CompletedCancellationEvidence is an owner-verified unpaid closure, not a request to cancel.
type CompletedCancellationEvidence struct {
	CancellationNo, OrderNo, InvoiceNo, QuoteNo, CustomerNo, CurrencyCode string
	CompletedAt                                                           time.Time
}

// CancellationEvidenceReader rechecks durable closure and zero financial activity.
// Callers share the startup database and their transaction context so the locks
// remain held until their local projection has committed.
type CancellationEvidenceReader interface {
	// GetCompletedCancellationEvidence rejects missing, mismatched, open or paid
	// owner facts; success returns a non-nil projection without storage models.
	GetCompletedCancellationEvidence(context.Context, string, string) (*CompletedCancellationEvidence, error)
}

type cancellationEvidenceReader struct{ database gdb.DB }

// NewCancellationEvidenceReader creates a read-only owner boundary on the shared database.
func NewCancellationEvidenceReader(database gdb.DB) (CancellationEvidenceReader, error) {
	if database == nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return &cancellationEvidenceReader{database: database}, nil
}

// GetCompletedCancellationEvidence never initiates payment closure or changes money.
func (s *cancellationEvidenceReader) GetCompletedCancellationEvidence(ctx context.Context, orderNo, cancellationNo string) (*CompletedCancellationEvidence, error) {
	if ctx == nil || !orderLifecycleIdentifierPattern.MatchString(orderNo) || !orderLifecycleIdentifierPattern.MatchString(cancellationNo) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	var proof *CompletedCancellationEvidence
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		order, err := lockOrderByNumber(ctx, tx, orderNo)
		if err != nil {
			return err
		}
		if order == nil || cancellationOrderStatus(order.StatusCode) != cancellationOrderClosed || order.PaidAt != nil || order.CancelledAt == nil {
			return bizerr.NewCode(CodeOrderNotCancelable)
		}
		cancellation, err := lockCancellationByNumber(ctx, tx, cancellationNo)
		if err != nil {
			return err
		}
		if cancellation == nil || cancellation.OrderNo != order.OrderNo || cancellation.CustomerNo != order.CustomerNo ||
			OrderCancellationStatus(cancellation.StatusCode) != OrderCancellationStatusCompleted || cancellation.CompletedAt == nil {
			return bizerr.NewCode(CodeOrderNotCancelable)
		}
		if cancellation.PaymentClosureOutcomeCode != nil {
			outcome := PaymentClosureOutcome(*cancellation.PaymentClosureOutcomeCode)
			if outcome != PaymentClosureOutcomeClosed && outcome != PaymentClosureOutcomeNoAttempts {
				return bizerr.NewCode(CodeOrderNotCancelable)
			}
		}
		invoice, err := lockOrderInvoice(ctx, tx, orderNo)
		if err != nil {
			return err
		}
		if !completedCancellationInvoiceIsUnpaid(invoice, order) {
			return bizerr.NewCode(CodeOrderNotCancelable)
		}
		state, err := loadOrderCancellationState(ctx, tx, order, invoice)
		if err != nil {
			return err
		}
		if state.PaymentAllocations != 0 || state.ActiveFundHolds != 0 || state.LedgerEntries != 0 || state.InventoryHeld != 0 || state.OrderItemTotal < 1 {
			return bizerr.NewCode(CodeOrderNotCancelable)
		}
		settled, err := s.database.Model(dao.FinancialSettlement.Table()).Ctx(ctx).Where(do.FinancialSettlement{TenantId: idccap.DefaultTenantID, OrderNo: orderNo}).Exist()
		if err != nil {
			return err
		}
		if settled {
			return bizerr.NewCode(CodeOrderNotCancelable)
		}
		proof = &CompletedCancellationEvidence{CancellationNo: cancellationNo, OrderNo: orderNo, InvoiceNo: invoice.InvoiceNo,
			QuoteNo: order.QuoteNo, CustomerNo: order.CustomerNo, CurrencyCode: order.CurrencyCode, CompletedAt: *cancellation.CompletedAt}
		return nil
	})
	return proof, err
}

// completedCancellationInvoiceIsUnpaid rechecks owner/customer/currency and all money fields.
func completedCancellationInvoiceIsUnpaid(invoice *entity.BillingInvoice, order *entity.CommerceOrder) bool {
	return invoice != nil && invoice.CustomerNo == order.CustomerNo && invoice.CurrencyCode == order.CurrencyCode &&
		billing.InvoiceStatus(invoice.StatusCode) == billing.InvoiceStatusCancelled && invoice.CancelledAt != nil && invoice.SettledAt == nil &&
		invoice.PaidAmountMinor == 0 && invoice.RefundableAmountMinor == 0 && invoice.RefundedAmountMinor == 0
}
