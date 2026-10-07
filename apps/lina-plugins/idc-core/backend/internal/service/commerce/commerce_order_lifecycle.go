// This file owns local order cancellation, bounded payment-expiry convergence,
// and the payment-owner closure handoff. It never calls a payment channel.

package commerce

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/marketing"
	"lina-plugin-idc-core/backend/internal/service/outbox"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const (
	defaultOrderExpiryBatchLimit = 32
	maximumOrderExpiryBatchLimit = 100
)

type OrderCancellationSource string

const (
	OrderCancellationSourceCustomer OrderCancellationSource = "customer"
	OrderCancellationSourceSystem   OrderCancellationSource = "system"
	OrderCancellationSourceAdmin    OrderCancellationSource = "admin"
	OrderCancellationSourcePayment  OrderCancellationSource = "payment"
)

type OrderCancellationReason string

const (
	OrderCancellationReasonCustomerRequested OrderCancellationReason = "customer_requested"
	OrderCancellationReasonPaymentExpired    OrderCancellationReason = "payment_expired"
	OrderCancellationReasonRiskRejected      OrderCancellationReason = "risk_rejected"
	OrderCancellationReasonManualRejected    OrderCancellationReason = "manual_rejected"
)

type OrderCancellationStatus string

const (
	OrderCancellationStatusPendingPaymentClosure OrderCancellationStatus = "pending_payment_closure"
	OrderCancellationStatusCompleted             OrderCancellationStatus = "completed"
	OrderCancellationStatusManualReview          OrderCancellationStatus = "manual_review"
)

type PaymentClosureOutcome string

const (
	PaymentClosureOutcomeClosed           PaymentClosureOutcome = "closed"
	PaymentClosureOutcomeNoAttempts       PaymentClosureOutcome = "no_attempts"
	PaymentClosureOutcomeConfirmedSuccess PaymentClosureOutcome = "confirmed_success"
	PaymentClosureOutcomeUnknown          PaymentClosureOutcome = "unknown"
)

var orderLifecycleIdentifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

// OrderCancellationRequest is a customer-owned idempotent cancellation intent.
// The raw idempotency key is hashed before any persistence.
type OrderCancellationRequest struct {
	OrderNo        string
	IdempotencyKey string
}

// PaymentClosureConfirmation is the trusted payment-owner result. Evidence is
// a digest of owner-held records, never raw channel material.
type PaymentClosureConfirmation struct {
	CancellationNo string
	Outcome        PaymentClosureOutcome
	Reference      string
	EvidenceDigest string
}

// OrderCancellation is a bounded lifecycle projection without payment secrets.
type OrderCancellation struct {
	CancellationNo string
	OrderNo        string
	Status         OrderCancellationStatus
	Reason         OrderCancellationReason
	RequestedAt    time.Time
	CompletedAt    *time.Time
	Reused         bool
}

// OrderExpiryBatch reports one bounded local convergence run.
type OrderExpiryBatch struct {
	Scanned               int
	Cancelled             int
	PendingPaymentClosure int
}

// OrderLifecycleService is the system/payment-owner boundary. Expiry is local
// and bounded; payment closure confirmation must come from trusted composition.
type OrderLifecycleService interface {
	ExpireDueOrders(context.Context, int) (OrderExpiryBatch, error)
	FinalizePaymentClosure(context.Context, PaymentClosureConfirmation) (*OrderCancellation, error)
}

type orderLifecycleServiceImpl struct {
	database               gdb.DB
	reservationReverser    QuoteReservationReverser
	hasReservationReverser bool
	outbox                 checkoutEventWriter
}

type orderCancellationInput struct {
	orderNo       string
	source        OrderCancellationSource
	reason        OrderCancellationReason
	requestDigest string
	requestedBy   int
}

type orderCancellationState struct {
	PaymentAllocations    int64 `orm:"payment_allocations"`
	ActiveFundHolds       int64 `orm:"active_fund_holds"`
	LedgerEntries         int64 `orm:"ledger_entries"`
	InventoryTotal        int64 `orm:"inventory_total"`
	InventoryHeld         int64 `orm:"inventory_held"`
	OrderItemTotal        int64 `orm:"order_item_total"`
	MarketingReservations int64 `orm:"marketing_reservations"`
}

var _ OrderLifecycleService = (*orderLifecycleServiceImpl)(nil)

// NewOrderLifecycleService creates the production local expiry/payment-closure
// service without starting goroutines. Scheduling belongs to pluginhost.Jobs.
func NewOrderLifecycleService(database gdb.DB) (OrderLifecycleService, error) {
	if database == nil {
		return nil, gerror.New("order lifecycle service requires a database")
	}
	marketingService, err := marketing.New(database, marketing.Config{})
	if err != nil {
		return nil, gerror.Wrap(err, "create order lifecycle marketing service")
	}
	evaluator, err := NewMarketingQuoteAdjustmentEvaluator(marketingService)
	if err != nil {
		return nil, err
	}
	reverser, ok := evaluator.(QuoteReservationReverser)
	if !ok {
		return nil, gerror.New("order lifecycle marketing reverser is unavailable")
	}
	outboxWriter, err := outbox.NewEnqueuer(database, outbox.Config{})
	if err != nil {
		return nil, gerror.Wrap(err, "create order lifecycle outbox service")
	}
	return &orderLifecycleServiceImpl{
		database: database, reservationReverser: reverser,
		hasReservationReverser: true, outbox: outboxWriter,
	}, nil
}

func (s *serviceImpl) CancelAccountOrder(
	ctx context.Context,
	request OrderCancellationRequest,
) (*OrderCancellation, error) {
	audience, err := s.currentCheckoutAudience(ctx)
	if err != nil {
		return nil, err
	}
	input, err := normalizeCustomerCancellation(*audience, request)
	if err != nil {
		return nil, err
	}
	var result *OrderCancellation
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		order, loadErr := lockOrderForCustomerCancellation(ctx, tx, input.orderNo, audience.CustomerNo)
		if loadErr != nil {
			return loadErr
		}
		result, loadErr = initiateOrderCancellation(ctx, tx, order, input, cancellationDependencies{
			reverser: s.reservationReverser, hasReverser: s.hasReservationReverser, outbox: s.outbox,
		})
		return loadErr
	})
	if err != nil {
		return nil, normalizeOrderLifecycleError(err)
	}
	return result, nil
}

func (s *orderLifecycleServiceImpl) ExpireDueOrders(
	ctx context.Context,
	limit int,
) (OrderExpiryBatch, error) {
	if limit == 0 {
		limit = defaultOrderExpiryBatchLimit
	}
	if limit < 1 || limit > maximumOrderExpiryBatchLimit {
		return OrderExpiryBatch{}, bizerr.NewCode(CodeInvalid)
	}
	result := OrderExpiryBatch{}
	for result.Scanned < limit {
		worked := false
		var cancellation *OrderCancellation
		err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
			order, loadErr := claimOneExpiredOrder(ctx, tx)
			if loadErr != nil || order == nil {
				return loadErr
			}
			worked = true
			requestDigest, digestErr := orderCancellationRequestDigest(
				order.CustomerNo, order.OrderNo, OrderCancellationSourceSystem,
				OrderCancellationReasonPaymentExpired, "payment-expired:"+order.OrderNo,
			)
			if digestErr != nil {
				return digestErr
			}
			cancellation, loadErr = initiateOrderCancellation(ctx, tx, order, orderCancellationInput{
				orderNo: order.OrderNo, source: OrderCancellationSourceSystem,
				reason: OrderCancellationReasonPaymentExpired, requestDigest: requestDigest,
			}, cancellationDependencies{
				reverser: s.reservationReverser, hasReverser: s.hasReservationReverser, outbox: s.outbox,
			})
			return loadErr
		})
		if err != nil {
			return result, normalizeOrderLifecycleError(err)
		}
		if !worked {
			break
		}
		result.Scanned++
		if cancellation.Status == OrderCancellationStatusCompleted {
			result.Cancelled++
		} else if cancellation.Status == OrderCancellationStatusPendingPaymentClosure {
			result.PendingPaymentClosure++
		}
	}
	return result, nil
}

func (s *orderLifecycleServiceImpl) FinalizePaymentClosure(
	ctx context.Context,
	confirmation PaymentClosureConfirmation,
) (*OrderCancellation, error) {
	confirmation = normalizePaymentClosureConfirmation(confirmation)
	if confirmation.CancellationNo == "" {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	var result *OrderCancellation
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		cancellation, loadErr := lockCancellationByNumber(ctx, tx, confirmation.CancellationNo)
		if loadErr != nil {
			return loadErr
		}
		if cancellation == nil {
			return bizerr.NewCode(CodeNotAccessible)
		}
		if cancellation.StatusCode != string(OrderCancellationStatusPendingPaymentClosure) {
			if paymentClosureReplayMatches(cancellation, confirmation) {
				result = projectOrderCancellation(cancellation, true)
				return nil
			}
			return bizerr.NewCode(CodeConflict)
		}
		order, loadErr := lockOrderByNumber(ctx, tx, cancellation.OrderNo)
		if loadErr != nil {
			return loadErr
		}
		if order == nil || order.CustomerNo != cancellation.CustomerNo || order.StatusCode != "payment_processing" {
			return bizerr.NewCode(CodeConflict)
		}
		if confirmation.Outcome == PaymentClosureOutcomeClosed ||
			confirmation.Outcome == PaymentClosureOutcomeNoAttempts {
			input := orderCancellationInput{
				orderNo: cancellation.OrderNo, source: OrderCancellationSource(cancellation.SourceCode),
				reason:        OrderCancellationReason(cancellation.ReasonCode),
				requestDigest: cancellation.RequestDigest, requestedBy: cancellation.RequestedBy,
			}
			result, loadErr = completeLocalOrderCancellation(
				ctx, tx, order, cancellation, input, confirmation, cancellationDependencies{
					reverser: s.reservationReverser, hasReverser: s.hasReservationReverser, outbox: s.outbox,
				},
			)
			return loadErr
		}
		result, loadErr = moveCancellationToManualReview(ctx, tx, cancellation, confirmation, s.outbox)
		return loadErr
	})
	if err != nil {
		return nil, normalizeOrderLifecycleError(err)
	}
	return result, nil
}

type cancellationDependencies struct {
	reverser    QuoteReservationReverser
	hasReverser bool
	outbox      checkoutEventWriter
}

func normalizeCustomerCancellation(
	audience principal.Audience,
	request OrderCancellationRequest,
) (orderCancellationInput, error) {
	orderNo := strings.TrimSpace(request.OrderNo)
	idempotencyKey := strings.TrimSpace(request.IdempotencyKey)
	if audience.Code != principal.AudienceCustomer || audience.Status != principal.AudienceStatusActive ||
		audience.UserID < 1 || audience.CustomerNo == "" ||
		orderNo != request.OrderNo || idempotencyKey != request.IdempotencyKey ||
		!orderLifecycleIdentifierPattern.MatchString(orderNo) || len(orderNo) > 40 ||
		!checkoutIdempotencyPattern.MatchString(idempotencyKey) {
		return orderCancellationInput{}, bizerr.NewCode(CodeInvalid)
	}
	digest, err := orderCancellationRequestDigest(
		audience.CustomerNo,
		orderNo,
		OrderCancellationSourceCustomer,
		OrderCancellationReasonCustomerRequested,
		idempotencyKey,
	)
	if err != nil {
		return orderCancellationInput{}, gerror.Wrap(err, "digest customer order cancellation")
	}
	return orderCancellationInput{
		orderNo: orderNo, source: OrderCancellationSourceCustomer,
		reason: OrderCancellationReasonCustomerRequested, requestDigest: digest,
		requestedBy: audience.UserID,
	}, nil
}

func normalizeOrderLifecycleError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := bizerr.As(err); ok {
		return err
	}
	return bizerr.WrapCode(err, CodeUnavailable)
}

func lockOrderForCustomerCancellation(
	ctx context.Context,
	tx gdb.TX,
	orderNo string,
	customerNo string,
) (*entity.CommerceOrder, error) {
	order, err := lockOrderByNumber(ctx, tx, orderNo)
	if err != nil {
		return nil, err
	}
	if order == nil || order.CustomerNo != customerNo {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return order, nil
}

func lockOrderByNumber(ctx context.Context, tx gdb.TX, orderNo string) (*entity.CommerceOrder, error) {
	var order *entity.CommerceOrder
	if err := tx.Model(dao.CommerceOrder.Table()).Ctx(ctx).Where(do.CommerceOrder{
		TenantId: idccap.DefaultTenantID, OrderNo: orderNo,
	}).LockUpdate().Scan(&order); err != nil {
		return nil, gerror.Wrap(err, "lock commerce order")
	}
	return order, nil
}

func claimOneExpiredOrder(ctx context.Context, tx gdb.TX) (*entity.CommerceOrder, error) {
	var order *entity.CommerceOrder
	if err := tx.Ctx(ctx).GetScan(&order, `
SELECT o.id, o.tenant_id, o.order_no, o.customer_no, o.quote_no,
       o.currency_code, o.status_code, o.payment_due_at, o.paid_at
FROM plugin_idc_core_commerce_order o
WHERE o.tenant_id = 0
  AND o.status_code IN ('pending_payment', 'payment_processing', 'manual_review')
  AND o.payment_due_at <= CURRENT_TIMESTAMP
  AND NOT EXISTS (
      SELECT 1 FROM plugin_idc_core_commerce_order_cancellation c
      WHERE c.tenant_id = 0 AND c.order_no = o.order_no
  )
ORDER BY o.payment_due_at ASC, o.id ASC
FOR UPDATE OF o SKIP LOCKED
LIMIT 1`); err != nil {
		return nil, gerror.Wrap(err, "claim expired commerce order")
	}
	return order, nil
}

func initiateOrderCancellation(
	ctx context.Context,
	tx gdb.TX,
	order *entity.CommerceOrder,
	input orderCancellationInput,
	deps cancellationDependencies,
) (*OrderCancellation, error) {
	if order == nil || order.OrderNo != input.orderNo {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	existing, err := lockCancellationByOrder(ctx, tx, order.OrderNo)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if existing.RequestDigest != input.requestDigest || existing.SourceCode != string(input.source) ||
			existing.ReasonCode != string(input.reason) {
			return nil, bizerr.NewCode(CodeConflict)
		}
		return projectOrderCancellation(existing, true), nil
	}
	now, err := checkoutDatabaseNow(ctx, tx)
	if err != nil {
		return nil, err
	}
	if order.StatusCode == "payment_processing" {
		cancellation, insertErr := insertOrderCancellation(
			ctx, tx, order, input, OrderCancellationStatusPendingPaymentClosure, now, nil,
		)
		if insertErr != nil {
			return nil, insertErr
		}
		if err = enqueueCancellationRequested(ctx, deps.outbox, cancellation, now); err != nil {
			return nil, err
		}
		return projectOrderCancellation(cancellation, false), nil
	}
	if order.StatusCode != "pending_payment" && order.StatusCode != "manual_review" {
		return nil, bizerr.NewCode(CodeOrderNotCancelable)
	}
	return completeLocalOrderCancellation(
		ctx, tx, order, nil, input, PaymentClosureConfirmation{}, deps,
	)
}

func completeLocalOrderCancellation(
	ctx context.Context,
	tx gdb.TX,
	order *entity.CommerceOrder,
	existing *entity.CommerceOrderCancellation,
	input orderCancellationInput,
	confirmation PaymentClosureConfirmation,
	deps cancellationDependencies,
) (*OrderCancellation, error) {
	if err := validateLocalCancellationTransition(order, existing, input, confirmation); err != nil {
		return nil, err
	}
	invoice, state, err := loadCancelableOrderState(ctx, tx, order)
	if err != nil {
		return nil, err
	}
	if err = reverseCancellationMarketing(ctx, order, state, deps); err != nil {
		return nil, err
	}
	now, err := checkoutDatabaseNow(ctx, tx)
	if err != nil {
		return nil, err
	}
	if err = applyLocalCancellationState(ctx, tx, order, invoice, state, input, now); err != nil {
		return nil, err
	}
	cancellation, err := persistCompletedCancellation(
		ctx, tx, order, existing, input, confirmation, now,
	)
	if err != nil {
		return nil, err
	}
	if err = enqueueOrderCancellationEvent(
		ctx, deps.outbox, cancellation, "commerce.order.cancelled", now,
	); err != nil {
		return nil, err
	}
	return projectOrderCancellation(cancellation, false), nil
}

func validateLocalCancellationTransition(
	order *entity.CommerceOrder,
	existing *entity.CommerceOrderCancellation,
	input orderCancellationInput,
	confirmation PaymentClosureConfirmation,
) error {
	if order == nil || order.OrderNo != input.orderNo || order.CustomerNo == "" ||
		(existing != nil && (existing.OrderNo != order.OrderNo || existing.CustomerNo != order.CustomerNo)) {
		return bizerr.NewCode(CodeConflict)
	}
	if existing == nil {
		if order.StatusCode != "pending_payment" && order.StatusCode != "manual_review" {
			return bizerr.NewCode(CodeOrderNotCancelable)
		}
		return nil
	}
	if order.StatusCode != "payment_processing" ||
		(confirmation.Outcome != PaymentClosureOutcomeClosed &&
			confirmation.Outcome != PaymentClosureOutcomeNoAttempts) {
		return bizerr.NewCode(CodeOrderNotCancelable)
	}
	return nil
}

func loadCancelableOrderState(
	ctx context.Context,
	tx gdb.TX,
	order *entity.CommerceOrder,
) (*entity.BillingInvoice, orderCancellationState, error) {
	invoice, err := lockOrderInvoice(ctx, tx, order.OrderNo)
	if err != nil {
		return nil, orderCancellationState{}, err
	}
	if !invoiceCanBeLocallyCancelled(invoice, order) {
		return nil, orderCancellationState{}, bizerr.NewCode(CodeOrderNotCancelable)
	}
	state, err := loadOrderCancellationState(ctx, tx, order, invoice)
	if err != nil {
		return nil, orderCancellationState{}, err
	}
	if state.PaymentAllocations != 0 || state.ActiveFundHolds != 0 || state.LedgerEntries != 0 ||
		state.InventoryHeld != state.InventoryTotal || state.OrderItemTotal < 1 {
		return nil, orderCancellationState{}, bizerr.NewCode(CodeOrderNotCancelable)
	}
	return invoice, state, nil
}

func reverseCancellationMarketing(
	ctx context.Context,
	order *entity.CommerceOrder,
	state orderCancellationState,
	deps cancellationDependencies,
) error {
	if state.MarketingReservations == 0 {
		return nil
	}
	if !deps.hasReverser || deps.reverser == nil {
		return bizerr.NewCode(CodeOrderNotCancelable)
	}
	if err := deps.reverser.ReverseConsumedQuote(ctx, order.CustomerNo, order.QuoteNo); err != nil {
		return bizerr.WrapCode(err, CodeOrderNotCancelable)
	}
	return nil
}

func applyLocalCancellationState(
	ctx context.Context,
	tx gdb.TX,
	order *entity.CommerceOrder,
	invoice *entity.BillingInvoice,
	state orderCancellationState,
	input orderCancellationInput,
	now time.Time,
) error {
	reservationStatus, releaseReason, reviewReason := "released", "order_cancelled", "order_cancelled"
	if input.reason == OrderCancellationReasonPaymentExpired {
		reservationStatus, releaseReason, reviewReason = "expired", "payment_expired", "payment_expired"
	}
	if err := releaseOrderTrialQualifications(ctx, tx, order, input.reason, now); err != nil {
		return err
	}
	updated, err := tx.Ctx(ctx).Exec(`
UPDATE plugin_idc_core_commerce_inventory_reservation
SET status_code = ?, release_reason_code = ?, released_at = ?,
	released_quantity = reserved_quantity,
    version = version + 1, updated_at = ?
WHERE tenant_id = 0 AND order_no = ? AND status_code = 'held'`,
		reservationStatus, releaseReason, now, now, order.OrderNo,
	)
	if err != nil {
		return gerror.Wrap(err, "release cancelled order inventory")
	}
	if err = requireOrderLifecycleAffectedCount(
		updated, state.InventoryTotal, "release cancelled order inventory",
	); err != nil {
		return err
	}
	if _, err = tx.Ctx(ctx).Exec(`
UPDATE plugin_idc_core_commerce_order_review
SET status_code = 'cancelled', reason_code = ?, request_digest = ?, evidence_digest = ?,
    reviewed_at = ?, updated_at = ?
WHERE tenant_id = 0 AND order_no = ? AND status_code = 'pending'`,
		reviewReason, input.requestDigest, input.requestDigest, now, now, order.OrderNo,
	); err != nil {
		return gerror.Wrap(err, "cancel pending order reviews")
	}
	updated, err = tx.Ctx(ctx).Exec(`
UPDATE plugin_idc_core_billing_invoice
SET status_code = 'cancelled', cancelled_at = ?, version = version + 1, updated_at = ?
WHERE tenant_id = 0 AND invoice_no = ? AND status_code IN ('pending_payment', 'overdue')
  AND paid_amount_minor = 0 AND refundable_amount_minor = 0 AND refunded_amount_minor = 0`,
		now, now, invoice.InvoiceNo,
	)
	if err != nil {
		return gerror.Wrap(err, "cancel unpaid order invoice")
	}
	if err = requireOrderLifecycleAffected(updated, "cancel unpaid order invoice"); err != nil {
		return err
	}
	updated, err = tx.Ctx(ctx).Exec(`
UPDATE plugin_idc_core_commerce_order
SET status_code = 'cancelled', cancelled_at = ?, version = version + 1, updated_at = ?
WHERE tenant_id = 0 AND order_no = ? AND status_code = ? AND paid_at IS NULL`,
		now, now, order.OrderNo, order.StatusCode,
	)
	if err != nil {
		return gerror.Wrap(err, "cancel unpaid commerce order")
	}
	return requireOrderLifecycleAffected(updated, "cancel unpaid commerce order")
}

func persistCompletedCancellation(
	ctx context.Context,
	tx gdb.TX,
	order *entity.CommerceOrder,
	existing *entity.CommerceOrderCancellation,
	input orderCancellationInput,
	confirmation PaymentClosureConfirmation,
	now time.Time,
) (*entity.CommerceOrderCancellation, error) {
	if existing == nil {
		return insertOrderCancellation(
			ctx, tx, order, input, OrderCancellationStatusCompleted, now, &now,
		)
	}
	updated, err := tx.Ctx(ctx).Exec(`
UPDATE plugin_idc_core_commerce_order_cancellation
SET status_code = 'completed', payment_closure_outcome_code = ?,
    payment_closure_reference = ?, payment_closure_digest = ?, completed_at = ?,
    version = version + 1, updated_at = ?
WHERE tenant_id = 0 AND cancellation_no = ? AND status_code = 'pending_payment_closure'`,
		confirmation.Outcome, confirmation.Reference, confirmation.EvidenceDigest,
		now, now, existing.CancellationNo,
	)
	if err != nil {
		return nil, gerror.Wrap(err, "complete order cancellation")
	}
	if err = requireOrderLifecycleAffected(updated, "complete order cancellation"); err != nil {
		return nil, err
	}
	return lockCancellationByNumber(ctx, tx, existing.CancellationNo)
}

func lockOrderInvoice(
	ctx context.Context,
	tx gdb.TX,
	orderNo string,
) (*entity.BillingInvoice, error) {
	var invoice *entity.BillingInvoice
	if err := tx.Model(dao.BillingInvoice.Table()).Ctx(ctx).Where(do.BillingInvoice{
		TenantId: idccap.DefaultTenantID, OrderNo: &orderNo, SourceCode: "order",
	}).LockUpdate().Scan(&invoice); err != nil {
		return nil, gerror.Wrap(err, "lock order invoice")
	}
	return invoice, nil
}

func invoiceCanBeLocallyCancelled(invoice *entity.BillingInvoice, order *entity.CommerceOrder) bool {
	return invoice != nil && order != nil && invoice.OrderNo != nil && *invoice.OrderNo == order.OrderNo &&
		invoice.CustomerNo == order.CustomerNo && invoice.CurrencyCode == order.CurrencyCode &&
		(invoice.StatusCode == "pending_payment" || invoice.StatusCode == "overdue") &&
		invoice.PaidAmountMinor == 0 && invoice.RefundableAmountMinor == 0 &&
		invoice.RefundedAmountMinor == 0 && invoice.SettledAt == nil && invoice.CancelledAt == nil
}

func loadOrderCancellationState(
	ctx context.Context,
	tx gdb.TX,
	order *entity.CommerceOrder,
	invoice *entity.BillingInvoice,
) (orderCancellationState, error) {
	var state orderCancellationState
	if err := tx.Ctx(ctx).GetScan(&state, `
SELECT
  (SELECT COUNT(*) FROM plugin_idc_core_billing_payment_allocation
    WHERE tenant_id = 0 AND invoice_no = ?) AS payment_allocations,
  (SELECT COUNT(*) FROM plugin_idc_core_billing_fund_hold
    WHERE tenant_id = 0 AND customer_no = ?
      AND status_code IN ('active', 'partially_consumed')
      AND source_reference IN (?, ?)) AS active_fund_holds,
  (SELECT COUNT(*) FROM plugin_idc_core_billing_ledger_entry
    WHERE tenant_id = 0 AND invoice_no = ?) AS ledger_entries,
  (SELECT COUNT(*) FROM plugin_idc_core_commerce_inventory_reservation
    WHERE tenant_id = 0 AND order_no = ?) AS inventory_total,
  (SELECT COUNT(*) FROM plugin_idc_core_commerce_inventory_reservation
    WHERE tenant_id = 0 AND order_no = ? AND status_code = 'held') AS inventory_held,
  (SELECT COUNT(*) FROM plugin_idc_core_commerce_order_item
    WHERE tenant_id = 0 AND order_no = ?) AS order_item_total,
  (SELECT COUNT(*) FROM plugin_idc_core_commerce_quote_adjustment_snapshot
    WHERE tenant_id = 0 AND quote_item_no IN (
      SELECT quote_item_no FROM plugin_idc_core_commerce_order_item
      WHERE tenant_id = 0 AND order_no = ?
    ) AND reservation_reference IS NOT NULL) AS marketing_reservations`,
		invoice.InvoiceNo, order.CustomerNo, order.OrderNo, invoice.InvoiceNo,
		invoice.InvoiceNo, order.OrderNo, order.OrderNo, order.OrderNo,
		order.OrderNo,
	); err != nil {
		return orderCancellationState{}, gerror.Wrap(err, "load order cancellation guards")
	}
	return state, nil
}

func lockCancellationByOrder(
	ctx context.Context,
	tx gdb.TX,
	orderNo string,
) (*entity.CommerceOrderCancellation, error) {
	var cancellation *entity.CommerceOrderCancellation
	if err := tx.Model(dao.CommerceOrderCancellation.Table()).Ctx(ctx).Where(do.CommerceOrderCancellation{
		TenantId: idccap.DefaultTenantID, OrderNo: orderNo,
	}).LockUpdate().Scan(&cancellation); err != nil {
		return nil, gerror.Wrap(err, "lock order cancellation")
	}
	return cancellation, nil
}

func lockCancellationByNumber(
	ctx context.Context,
	tx gdb.TX,
	cancellationNo string,
) (*entity.CommerceOrderCancellation, error) {
	var cancellation *entity.CommerceOrderCancellation
	if err := tx.Model(dao.CommerceOrderCancellation.Table()).Ctx(ctx).Where(do.CommerceOrderCancellation{
		TenantId: idccap.DefaultTenantID, CancellationNo: cancellationNo,
	}).LockUpdate().Scan(&cancellation); err != nil {
		return nil, gerror.Wrap(err, "lock order cancellation by number")
	}
	return cancellation, nil
}

func insertOrderCancellation(
	ctx context.Context,
	tx gdb.TX,
	order *entity.CommerceOrder,
	input orderCancellationInput,
	status OrderCancellationStatus,
	now time.Time,
	completedAt *time.Time,
) (*entity.CommerceOrderCancellation, error) {
	cancellationNo := "CAN_" + input.requestDigest[:32]
	row := do.CommerceOrderCancellation{
		TenantId: idccap.DefaultTenantID, CancellationNo: cancellationNo,
		OrderNo: order.OrderNo, CustomerNo: order.CustomerNo,
		SourceCode: string(input.source), ReasonCode: string(input.reason),
		RequestDigest: input.requestDigest, StatusCode: string(status), RequestedBy: input.requestedBy,
		RequestedAt: &now, CompletedAt: completedAt, Version: 1, CreatedAt: &now, UpdatedAt: &now,
	}
	if _, err := tx.Model(dao.CommerceOrderCancellation.Table()).Ctx(ctx).Data(row).Insert(); err != nil {
		return nil, gerror.Wrap(err, "insert order cancellation")
	}
	return lockCancellationByNumber(ctx, tx, cancellationNo)
}

func enqueueCancellationRequested(
	ctx context.Context,
	writer checkoutEventWriter,
	cancellation *entity.CommerceOrderCancellation,
	now time.Time,
) error {
	return enqueueOrderCancellationEvent(
		ctx, writer, cancellation, "commerce.order.cancellation.requested", now,
	)
}

func enqueueOrderCancellationEvent(
	ctx context.Context,
	writer checkoutEventWriter,
	cancellation *entity.CommerceOrderCancellation,
	eventType string,
	now time.Time,
) error {
	if writer == nil || cancellation == nil {
		return gerror.New("order cancellation outbox writer is unavailable")
	}
	payload, err := json.Marshal(struct {
		CancellationNo string `json:"cancellationNo"`
		OrderNo        string `json:"orderNo"`
		CustomerNo     string `json:"customerNo"`
		ReasonCode     string `json:"reasonCode"`
		StatusCode     string `json:"statusCode"`
	}{
		CancellationNo: cancellation.CancellationNo, OrderNo: cancellation.OrderNo,
		CustomerNo: cancellation.CustomerNo, ReasonCode: cancellation.ReasonCode,
		StatusCode: cancellation.StatusCode,
	})
	if err != nil {
		return gerror.Wrap(err, "encode order cancellation event")
	}
	metadata, err := json.Marshal(struct {
		Source string `json:"source"`
	}{Source: cancellation.SourceCode})
	if err != nil {
		return gerror.Wrap(err, "encode order cancellation metadata")
	}
	_, err = writer.Enqueue(ctx, outbox.EnqueueInput{
		EventType: eventType, EventVersion: 1, Topic: eventType,
		AggregateType: "commerce_order", AggregateNo: cancellation.OrderNo,
		CorrelationNo:    cancellation.CancellationNo,
		DeduplicationKey: eventType + ":" + cancellation.CancellationNo,
		Payload:          payload, Metadata: metadata, AvailableAt: now,
	})
	if err != nil {
		return gerror.Wrap(err, "enqueue order cancellation event")
	}
	return nil
}

func moveCancellationToManualReview(
	ctx context.Context,
	tx gdb.TX,
	cancellation *entity.CommerceOrderCancellation,
	confirmation PaymentClosureConfirmation,
	writer checkoutEventWriter,
) (*OrderCancellation, error) {
	now, err := checkoutDatabaseNow(ctx, tx)
	if err != nil {
		return nil, err
	}
	result, err := tx.Ctx(ctx).Exec(`
UPDATE plugin_idc_core_commerce_order_cancellation
SET status_code = 'manual_review', payment_closure_outcome_code = ?, payment_closure_reference = ?,
    payment_closure_digest = ?, version = version + 1, updated_at = ?
WHERE tenant_id = 0 AND cancellation_no = ? AND status_code = 'pending_payment_closure'`,
		confirmation.Outcome, confirmation.Reference, confirmation.EvidenceDigest,
		now, cancellation.CancellationNo,
	)
	if err != nil {
		return nil, gerror.Wrap(err, "move order cancellation to manual review")
	}
	if err = requireOrderLifecycleAffected(result, "move order cancellation to manual review"); err != nil {
		return nil, err
	}
	cancellation, err = lockCancellationByNumber(ctx, tx, cancellation.CancellationNo)
	if err != nil {
		return nil, err
	}
	if err = enqueueOrderCancellationEvent(
		ctx, writer, cancellation, "commerce.order.cancellation.manual_review", now,
	); err != nil {
		return nil, err
	}
	return projectOrderCancellation(cancellation, false), nil
}

func projectOrderCancellation(
	cancellation *entity.CommerceOrderCancellation,
	reused bool,
) *OrderCancellation {
	if cancellation == nil {
		return nil
	}
	requestedAt := time.Time{}
	if cancellation.RequestedAt != nil {
		requestedAt = *cancellation.RequestedAt
	}
	return &OrderCancellation{
		CancellationNo: cancellation.CancellationNo, OrderNo: cancellation.OrderNo,
		Status: OrderCancellationStatus(cancellation.StatusCode),
		Reason: OrderCancellationReason(cancellation.ReasonCode), RequestedAt: requestedAt,
		CompletedAt: cancellation.CompletedAt, Reused: reused,
	}
}

func normalizePaymentClosureConfirmation(input PaymentClosureConfirmation) PaymentClosureConfirmation {
	input.CancellationNo = strings.TrimSpace(input.CancellationNo)
	input.Reference = strings.TrimSpace(input.Reference)
	input.EvidenceDigest = strings.TrimSpace(input.EvidenceDigest)
	validOutcome := input.Outcome == PaymentClosureOutcomeClosed || input.Outcome == PaymentClosureOutcomeNoAttempts ||
		input.Outcome == PaymentClosureOutcomeConfirmedSuccess || input.Outcome == PaymentClosureOutcomeUnknown
	if !orderLifecycleIdentifierPattern.MatchString(input.CancellationNo) || len(input.CancellationNo) > 40 ||
		!validOutcome || input.Reference == "" || len(input.Reference) > 128 ||
		!quoteDigestPattern.MatchString(input.EvidenceDigest) {
		return PaymentClosureConfirmation{}
	}
	return input
}

func paymentClosureReplayMatches(
	cancellation *entity.CommerceOrderCancellation,
	confirmation PaymentClosureConfirmation,
) bool {
	return cancellation != nil && cancellation.PaymentClosureReference != nil &&
		cancellation.PaymentClosureDigest != nil && cancellation.PaymentClosureOutcomeCode != nil &&
		*cancellation.PaymentClosureOutcomeCode == string(confirmation.Outcome) &&
		*cancellation.PaymentClosureReference == confirmation.Reference &&
		*cancellation.PaymentClosureDigest == confirmation.EvidenceDigest
}

func orderCancellationRequestDigest(
	customerNo string,
	orderNo string,
	source OrderCancellationSource,
	reason OrderCancellationReason,
	idempotencyKey string,
) (string, error) {
	keyDigest := sha256.Sum256([]byte(idempotencyKey))
	return quoteContentDigest(struct {
		CustomerNo        string                  `json:"customerNo"`
		OrderNo           string                  `json:"orderNo"`
		Source            OrderCancellationSource `json:"source"`
		Reason            OrderCancellationReason `json:"reason"`
		IdempotencyDigest string                  `json:"idempotencyDigest"`
	}{
		CustomerNo: customerNo, OrderNo: orderNo, Source: source, Reason: reason,
		IdempotencyDigest: hex.EncodeToString(keyDigest[:]),
	})
}

func requireOrderLifecycleAffected(result interface{ RowsAffected() (int64, error) }, operation string) error {
	return requireOrderLifecycleAffectedCount(result, 1, operation)
}

func requireOrderLifecycleAffectedCount(
	result interface{ RowsAffected() (int64, error) },
	expected int64,
	operation string,
) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, "read "+operation+" result")
	}
	if affected != expected {
		return bizerr.NewCode(CodeConflict)
	}
	return nil
}
