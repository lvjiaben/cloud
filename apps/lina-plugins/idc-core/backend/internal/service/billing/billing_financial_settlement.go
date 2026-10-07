// This file owns the sole financial authority that can make a commerce order
// eligible for downstream fulfillment. It never mutates fulfillment or service
// runtime state directly.
package billing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/outbox"
)

const (
	maximumFinancialSettlementSources      = 1000
	maximumFinancialSettlementEvidenceRows = 2000
	financiallySettledEventType            = "commerce.order.financially_settled"
)

// financialSettlementServiceImpl coordinates one local billing transaction and
// its transaction-bound outbox writer.
type financialSettlementServiceImpl struct {
	database gdb.DB
	events   outbox.Enqueuer
	credits  CreditReservationService
}

var _ FinancialSettlementService = (*financialSettlementServiceImpl)(nil)

// settlementOrderItemSummary is the bounded aggregate used to prove order
// amount and generic quantity conservation.
type settlementOrderItemSummary struct {
	ItemCount        int64 `orm:"item_count"`
	TotalQuantity    int64 `orm:"total_quantity"`
	TrialCount       int64 `orm:"trial_count"`
	SubtotalMinor    int64 `orm:"subtotal_minor"`
	DiscountMinor    int64 `orm:"discount_minor"`
	NetAmountMinor   int64 `orm:"net_amount_minor"`
	TaxAmountMinor   int64 `orm:"tax_amount_minor"`
	TotalAmountMinor int64 `orm:"total_amount_minor"`
}

// settlementInvoiceItemSummary is the bounded aggregate used to prove invoice
// amount conservation without loading each item.
type settlementInvoiceItemSummary struct {
	ItemCount        int64 `orm:"item_count"`
	SubtotalMinor    int64 `orm:"subtotal_minor"`
	DiscountMinor    int64 `orm:"discount_minor"`
	NetAmountMinor   int64 `orm:"net_amount_minor"`
	TaxAmountMinor   int64 `orm:"tax_amount_minor"`
	TotalAmountMinor int64 `orm:"total_amount_minor"`
}

// settlementInventorySummary proves frozen inventory reservations have not
// advanced before the financial fact is committed.
type settlementInventorySummary struct {
	SnapshotCount        int64 `orm:"snapshot_count"`
	TrackedItemCount     int64 `orm:"tracked_item_count"`
	TrackedQuantity      int64 `orm:"tracked_quantity"`
	ReservationCount     int64 `orm:"reservation_count"`
	HeldReservations     int64 `orm:"held_reservations"`
	ReservedQuantity     int64 `orm:"reserved_quantity"`
	MatchingReservations int64 `orm:"matching_reservations"`
}

// settlementIdentityEvidence is the minimal current identity projection whose
// digest is persisted without personal material.
type settlementIdentityEvidence struct {
	CustomerType   string
	SubjectVersion int64
	StatusCode     string
	CaseNo         string
	Version        int64
	VerifiedAt     *time.Time
	ExpiresAt      *time.Time
}

// newFinancialSettlementService validates the explicit database and outbox
// dependencies used by the settlement authority.
func newFinancialSettlementService(
	database gdb.DB,
	events outbox.Enqueuer,
	credits CreditReservationService,
) (FinancialSettlementService, error) {
	if isNilFinancialSettlementDependency(database) ||
		isNilFinancialSettlementDependency(events) ||
		isNilFinancialSettlementDependency(credits) {
		return nil, gerror.New("financial settlement service requires database, outbox, and credit services")
	}
	return &financialSettlementServiceImpl{
		database: database, events: events, credits: credits,
	}, nil
}

// isNilFinancialSettlementDependency rejects both nil interfaces and typed nil
// implementations at construction time.
func isNilFinancialSettlementDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

// SettleOrder revalidates every local authority inside one transaction and
// appends exactly one durable settlement fact plus one deduplicated outbox event.
func (s *financialSettlementServiceImpl) SettleOrder(
	ctx context.Context,
	orderNo string,
) (*FinancialSettlement, error) {
	if orderNo != strings.TrimSpace(orderNo) || !validBusinessReference(orderNo, 40) {
		return nil, bizerr.NewCode(CodeSettlementInvalid)
	}
	return s.settleOrder(ctx, orderNo, nil)
}

// SettleOrderWithCredit derives the exact reservation coordinates from the
// locked order and commits the reserve and settlement as one unit.
func (s *financialSettlementServiceImpl) SettleOrderWithCredit(
	ctx context.Context,
	input CreditOrderSettlementInput,
) (*FinancialSettlement, error) {
	if input.OrderNo != strings.TrimSpace(input.OrderNo) ||
		input.ActorType != strings.TrimSpace(input.ActorType) ||
		!validBusinessReference(input.OrderNo, 40) ||
		!validCreditActor(input.ActorType, input.ActorID) {
		return nil, bizerr.NewCode(CodeSettlementInvalid)
	}
	return s.settleOrder(ctx, input.OrderNo, &input)
}

// settleOrder owns the single transaction boundary for ordinary and credit
// settlement. A non-nil credit input is never accepted by the funded path.
func (s *financialSettlementServiceImpl) settleOrder(
	ctx context.Context,
	orderNo string,
	creditInput *CreditOrderSettlementInput,
) (*FinancialSettlement, error) {
	var result *FinancialSettlement
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		order, loadErr := lockFinancialSettlementOrder(ctx, s.database, orderNo)
		if loadErr != nil {
			return loadErr
		}
		if order == nil {
			return bizerr.NewCode(CodeSettlementNotEligible)
		}
		existing, loadErr := loadFinancialSettlementByOrder(ctx, s.database, order.OrderNo)
		if loadErr != nil {
			return loadErr
		}
		if existing != nil {
			result, loadErr = projectPersistedFinancialSettlement(
				ctx, s.database, existing, true,
			)
			if loadErr == nil && creditInput != nil && result.Basis != SettlementBasisCredit {
				return bizerr.NewCode(CodeSettlementNotEligible)
			}
			return loadErr
		}

		invoice, loadErr := lockFinancialSettlementInvoice(ctx, s.database, order.OrderNo)
		if loadErr != nil {
			return loadErr
		}
		now, loadErr := databaseTime(ctx, s.database)
		if loadErr != nil {
			return loadErr
		}
		items, loadErr := validateFinancialSettlementCommercialFacts(
			ctx, s.database, order, invoice,
		)
		if loadErr != nil {
			return gerror.Wrap(loadErr, "validate financial settlement commercial facts")
		}
		identityDigest, loadErr := loadFinancialSettlementIdentityDigest(
			ctx, s.database, order.CustomerNo, now,
		)
		if loadErr != nil {
			return gerror.Wrap(loadErr, "validate financial settlement identity")
		}
		var prepared preparedFinancialSettlement
		if creditInput == nil {
			prepared, loadErr = prepareFinancialSettlement(
				ctx, s.database, order, invoice, items, identityDigest, now,
			)
		} else {
			prepared, loadErr = s.prepareCreditFinancialSettlement(
				ctx, order, invoice, items, identityDigest, now, *creditInput,
			)
		}
		if loadErr != nil {
			return gerror.Wrap(loadErr, "prepare financial settlement evidence")
		}
		if loadErr = persistFinancialSettlement(
			ctx, s.database, invoice, prepared,
		); loadErr != nil {
			return gerror.Wrap(loadErr, "persist financial settlement evidence")
		}
		if loadErr = transitionFinancialSettlementOrder(
			ctx, s.database, order, prepared.settledAt,
		); loadErr != nil {
			return gerror.Wrap(loadErr, "transition financially settled order")
		}
		if loadErr = s.enqueueFinancialSettlement(ctx, prepared, now); loadErr != nil {
			return loadErr
		}
		result = projectPreparedFinancialSettlement(prepared, false)
		return nil
	})
	if err != nil {
		return nil, normalizeFinancialSettlementError(err)
	}
	return result, nil
}

// lockFinancialSettlementOrder serializes all settlement attempts on the order
// row before any evidence is evaluated.
func lockFinancialSettlementOrder(
	ctx context.Context,
	database gdb.DB,
	orderNo string,
) (*entity.CommerceOrder, error) {
	var order *entity.CommerceOrder
	if err := database.Model(dao.CommerceOrder.Table()).Ctx(ctx).
		Where(do.CommerceOrder{TenantId: idccap.DefaultTenantID, OrderNo: orderNo}).
		LockUpdate().Scan(&order); err != nil {
		return nil, gerror.Wrap(err, "lock financial settlement order")
	}
	return order, nil
}

// lockFinancialSettlementInvoice locks the one original invoice owned by the
// order; a missing or ambiguous invoice fails later eligibility checks.
func lockFinancialSettlementInvoice(
	ctx context.Context,
	database gdb.DB,
	orderNo string,
) (*entity.BillingInvoice, error) {
	var invoice *entity.BillingInvoice
	if err := database.Model(dao.BillingInvoice.Table()).Ctx(ctx).
		Where(do.BillingInvoice{
			TenantId: idccap.DefaultTenantID, SourceCode: "order", OrderNo: &orderNo,
		}).LockUpdate().Scan(&invoice); err != nil {
		return nil, gerror.Wrap(err, "lock financial settlement invoice")
	}
	return invoice, nil
}

// loadFinancialSettlementByOrder returns the immutable replay authority after
// the order lock has serialized concurrent attempts.
func loadFinancialSettlementByOrder(
	ctx context.Context,
	database gdb.DB,
	orderNo string,
) (*entity.FinancialSettlement, error) {
	var settlement *entity.FinancialSettlement
	if err := database.Model(dao.FinancialSettlement.Table()).Ctx(ctx).
		Where(do.FinancialSettlement{
			TenantId: idccap.DefaultTenantID, OrderNo: orderNo,
		}).Scan(&settlement); err != nil {
		return nil, gerror.Wrap(err, "load financial settlement by order")
	}
	return settlement, nil
}

// validateFinancialSettlementCommercialFacts proves order, invoice, item,
// fulfillment, and reservation conservation with a fixed number of queries.
func validateFinancialSettlementCommercialFacts(
	ctx context.Context,
	database gdb.DB,
	order *entity.CommerceOrder,
	invoice *entity.BillingInvoice,
) (settlementOrderItemSummary, error) {
	if !financialSettlementOrderInvoiceMatches(order, invoice) {
		return settlementOrderItemSummary{}, bizerr.NewCode(CodeSettlementNotEligible)
	}

	var itemSummary settlementOrderItemSummary
	if err := database.GetScan(ctx, &itemSummary, `
SELECT COUNT(*) AS item_count,
       COALESCE(SUM(quantity_snapshot.quantity), 0) AS total_quantity,
       COALESCE(SUM(CASE WHEN charge_type_code = 'trial' THEN 1 ELSE 0 END), 0)
           AS trial_count,
       COALESCE(SUM(subtotal_minor), 0) AS subtotal_minor,
       COALESCE(SUM(discount_minor), 0) AS discount_minor,
       COALESCE(SUM(net_amount_minor), 0) AS net_amount_minor,
       COALESCE(SUM(tax_amount_minor), 0) AS tax_amount_minor,
       COALESCE(SUM(total_amount_minor), 0) AS total_amount_minor
FROM plugin_idc_core_commerce_order_item AS order_item
JOIN plugin_idc_core_commerce_order_item_quantity_snapshot AS quantity_snapshot
  ON quantity_snapshot.tenant_id = order_item.tenant_id
 AND quantity_snapshot.order_no = order_item.order_no
 AND quantity_snapshot.order_item_no = order_item.order_item_no
WHERE order_item.tenant_id = 0 AND order_item.order_no = ?`, order.OrderNo); err != nil {
		return settlementOrderItemSummary{}, gerror.Wrap(err, "summarize settlement order items")
	}
	if !financialSettlementOrderItemsMatch(itemSummary, order) {
		return settlementOrderItemSummary{}, gerror.Wrap(
			bizerr.NewCode(CodeSettlementIntegrity), "order item settlement totals",
		)
	}

	var invoiceSummary settlementInvoiceItemSummary
	if err := database.GetScan(ctx, &invoiceSummary, `
SELECT COUNT(*) AS item_count,
       COALESCE(SUM(subtotal_minor), 0) AS subtotal_minor,
       COALESCE(SUM(discount_minor), 0) AS discount_minor,
       COALESCE(SUM(net_amount_minor), 0) AS net_amount_minor,
       COALESCE(SUM(tax_amount_minor), 0) AS tax_amount_minor,
       COALESCE(SUM(total_amount_minor), 0) AS total_amount_minor
FROM plugin_idc_core_billing_invoice_item
WHERE tenant_id = 0 AND invoice_no = ?`, invoice.InvoiceNo); err != nil {
		return settlementOrderItemSummary{}, gerror.Wrap(err, "summarize settlement invoice items")
	}
	if !financialSettlementInvoiceItemsMatch(invoiceSummary, invoice) {
		return settlementOrderItemSummary{}, gerror.Wrap(
			bizerr.NewCode(CodeSettlementIntegrity), "invoice item settlement totals",
		)
	}

	var inventory settlementInventorySummary
	quotePurposeValue, err := database.GetValue(ctx, `
SELECT purpose_code FROM plugin_idc_core_commerce_quote_snapshot
WHERE tenant_id = 0 AND quote_no = ? AND customer_no = ?`, order.QuoteNo, order.CustomerNo)
	if err != nil {
		return settlementOrderItemSummary{}, gerror.Wrap(err, "load settlement quote purpose")
	}
	quotePurpose := quotePurposeValue.String()
	if err := database.GetScan(ctx, &inventory, `
WITH inventory_items AS (
    SELECT inventory.order_item_no, inventory.tracking_enabled,
           quantity.quantity
    FROM plugin_idc_core_commerce_order_item AS order_item
    JOIN plugin_idc_core_commerce_order_item_inventory_snapshot AS inventory
      ON inventory.order_item_no = order_item.order_item_no
    JOIN plugin_idc_core_commerce_order_item_quantity_snapshot AS quantity
      ON quantity.tenant_id = order_item.tenant_id
     AND quantity.order_no = order_item.order_no
     AND quantity.order_item_no = order_item.order_item_no
    WHERE order_item.tenant_id = 0 AND order_item.order_no = ?
), reservations AS (
    SELECT order_item_no, status_code, reserved_quantity
    FROM plugin_idc_core_commerce_inventory_reservation
    WHERE tenant_id = 0 AND order_no = ?
)
SELECT (SELECT COUNT(*) FROM inventory_items) AS snapshot_count,
       (SELECT COUNT(*) FROM inventory_items WHERE tracking_enabled) AS tracked_item_count,
       (SELECT COALESCE(SUM(quantity), 0)
          FROM inventory_items WHERE tracking_enabled) AS tracked_quantity,
       (SELECT COUNT(*) FROM reservations) AS reservation_count,
       (SELECT COUNT(*) FROM reservations WHERE status_code = 'held') AS held_reservations,
       (SELECT COALESCE(SUM(reserved_quantity), 0)
          FROM reservations WHERE status_code = 'held') AS reserved_quantity,
       (SELECT COUNT(*)
          FROM inventory_items AS inventory_item
          JOIN reservations AS reservation
            ON reservation.order_item_no = inventory_item.order_item_no
           AND reservation.status_code = 'held'
           AND reservation.reserved_quantity = inventory_item.quantity
         WHERE inventory_item.tracking_enabled) AS matching_reservations`,
		order.OrderNo, order.OrderNo,
	); err != nil {
		return settlementOrderItemSummary{}, gerror.Wrap(err, "summarize settlement inventory gates")
	}
	if quotePurpose == "addon" {
		if itemSummary.ItemCount != 1 ||
			!financialSettlementInventoryMatches(inventory, itemSummary) {
			return settlementOrderItemSummary{}, bizerr.NewCode(CodeSettlementNotEligible)
		}
		return itemSummary, nil
	}
	if !financialSettlementInventoryMatches(inventory, itemSummary) {
		return settlementOrderItemSummary{}, bizerr.NewCode(CodeSettlementNotEligible)
	}
	return itemSummary, nil
}

// financialSettlementInventoryMatches verifies one held reservation per order
// item and conservation of the generic frozen quantity.
func financialSettlementInventoryMatches(
	inventory settlementInventorySummary,
	items settlementOrderItemSummary,
) bool {
	return inventory.SnapshotCount == items.ItemCount &&
		inventory.TrackedItemCount >= 0 &&
		inventory.TrackedItemCount <= items.ItemCount &&
		inventory.TrackedQuantity >= inventory.TrackedItemCount &&
		inventory.ReservationCount == inventory.TrackedItemCount &&
		inventory.HeldReservations == inventory.TrackedItemCount &&
		inventory.ReservedQuantity == inventory.TrackedQuantity &&
		inventory.MatchingReservations == inventory.TrackedItemCount
}

// financialSettlementOrderInvoiceMatches checks exact owner, currency, amount,
// risk, and payable lifecycle agreement.
func financialSettlementOrderInvoiceMatches(
	order *entity.CommerceOrder,
	invoice *entity.BillingInvoice,
) bool {
	return order != nil && invoice != nil && invoice.OrderNo != nil &&
		*invoice.OrderNo == order.OrderNo && invoice.SourceCode == "order" &&
		invoice.CustomerNo == order.CustomerNo && invoice.CurrencyCode == order.CurrencyCode &&
		invoice.ItemCount == order.ItemCount && invoice.SubtotalMinor == order.SubtotalMinor &&
		invoice.DiscountMinor == order.DiscountMinor && invoice.NetAmountMinor == order.NetAmountMinor &&
		invoice.TaxAmountMinor == order.TaxAmountMinor &&
		invoice.TotalAmountMinor == order.TotalAmountMinor && invoice.IssuedAt != nil &&
		order.PaidAt == nil && order.RiskDecisionCode == "approved" &&
		(order.StatusCode == "pending_payment" || order.StatusCode == "payment_processing")
}

// financialSettlementOrderItemsMatch verifies the aggregate order item facts.
func financialSettlementOrderItemsMatch(
	summary settlementOrderItemSummary,
	order *entity.CommerceOrder,
) bool {
	return summary.ItemCount == int64(order.ItemCount) &&
		summary.SubtotalMinor == order.SubtotalMinor &&
		summary.DiscountMinor == order.DiscountMinor &&
		summary.NetAmountMinor == order.NetAmountMinor &&
		summary.TaxAmountMinor == order.TaxAmountMinor &&
		summary.TotalAmountMinor == order.TotalAmountMinor && summary.TotalQuantity >= 1
}

// financialSettlementInvoiceItemsMatch verifies the aggregate invoice item facts.
func financialSettlementInvoiceItemsMatch(
	summary settlementInvoiceItemSummary,
	invoice *entity.BillingInvoice,
) bool {
	return summary.ItemCount == int64(invoice.ItemCount) &&
		summary.SubtotalMinor == invoice.SubtotalMinor &&
		summary.DiscountMinor == invoice.DiscountMinor &&
		summary.NetAmountMinor == invoice.NetAmountMinor &&
		summary.TaxAmountMinor == invoice.TaxAmountMinor &&
		summary.TotalAmountMinor == invoice.TotalAmountMinor
}

// loadFinancialSettlementIdentityDigest re-reads and share-locks the current
// active customer's verified individual or enterprise identity evidence.
func loadFinancialSettlementIdentityDigest(
	ctx context.Context,
	database gdb.DB,
	customerNo string,
	now time.Time,
) (string, error) {
	var subject *entity.CustomerSubject
	if err := database.GetScan(ctx, &subject, `
SELECT customer_no, customer_type_code, status_code, version
FROM plugin_idc_core_customer_subject
WHERE tenant_id = 0 AND customer_no = ?
FOR SHARE`, customerNo); err != nil {
		return "", gerror.Wrap(err, "lock financial settlement customer")
	}
	if subject == nil || subject.StatusCode != "active" || subject.Version < 1 {
		return "", bizerr.NewCode(CodeSettlementNotEligible)
	}
	evidence := settlementIdentityEvidence{
		CustomerType: subject.CustomerTypeCode, SubjectVersion: subject.Version,
	}
	switch subject.CustomerTypeCode {
	case "individual":
		var profile *entity.CustomerIndividualProfile
		if err := database.GetScan(ctx, &profile, `
SELECT identity_status_code, identity_case_no, identity_verified_at,
       identity_expires_at, version
FROM plugin_idc_core_customer_individual_profile
WHERE tenant_id = 0 AND customer_no = ?
FOR SHARE`, customerNo); err != nil {
			return "", gerror.Wrap(err, "lock individual settlement identity")
		}
		if profile != nil {
			evidence.StatusCode = profile.IdentityStatusCode
			evidence.CaseNo = profile.IdentityCaseNo
			evidence.Version = profile.Version
			evidence.VerifiedAt = profile.IdentityVerifiedAt
			evidence.ExpiresAt = profile.IdentityExpiresAt
		}
	case "enterprise":
		var profile *entity.CustomerEnterpriseProfile
		if err := database.GetScan(ctx, &profile, `
SELECT verification_status_code, verification_case_no, verified_at,
       expires_at, version
FROM plugin_idc_core_customer_enterprise_profile
WHERE tenant_id = 0 AND customer_no = ?
FOR SHARE`, customerNo); err != nil {
			return "", gerror.Wrap(err, "lock enterprise settlement identity")
		}
		if profile != nil {
			evidence.StatusCode = profile.VerificationStatusCode
			evidence.CaseNo = profile.VerificationCaseNo
			evidence.Version = profile.Version
			evidence.VerifiedAt = profile.VerifiedAt
			evidence.ExpiresAt = profile.ExpiresAt
		}
	default:
		return "", bizerr.NewCode(CodeSettlementNotEligible)
	}
	if evidence.StatusCode != "verified" ||
		!validBusinessReference(evidence.CaseNo, 128) || evidence.Version < 1 ||
		evidence.VerifiedAt == nil || evidence.VerifiedAt.After(now) ||
		(evidence.ExpiresAt != nil && !evidence.ExpiresAt.After(now)) {
		return "", bizerr.NewCode(CodeSettlementNotEligible)
	}
	digest, err := contentDigest(struct {
		CustomerType   string     `json:"customerType"`
		SubjectVersion int64      `json:"subjectVersion"`
		StatusCode     string     `json:"statusCode"`
		CaseNo         string     `json:"caseNo"`
		Version        int64      `json:"version"`
		VerifiedAt     *time.Time `json:"verifiedAt"`
		ExpiresAt      *time.Time `json:"expiresAt"`
	}{
		evidence.CustomerType, evidence.SubjectVersion, evidence.StatusCode,
		evidence.CaseNo, evidence.Version, evidence.VerifiedAt, evidence.ExpiresAt,
	})
	if err != nil {
		return "", err
	}
	return digest, nil
}

// transitionFinancialSettlementOrder applies the sole financial order state
// transition while leaving fulfillment and resource state untouched.
func transitionFinancialSettlementOrder(
	ctx context.Context,
	database gdb.DB,
	order *entity.CommerceOrder,
	settledAt time.Time,
) error {
	updated, err := database.Model(dao.CommerceOrder.Table()).Ctx(ctx).
		Where(do.CommerceOrder{
			TenantId: idccap.DefaultTenantID, OrderNo: order.OrderNo,
			RiskDecisionCode: "approved", Version: order.Version,
		}).WhereIn(dao.CommerceOrder.Columns().StatusCode, []string{
		"pending_payment", "payment_processing",
	}).Data(do.CommerceOrder{
		StatusCode: "paid", PaidAt: &settledAt, Version: order.Version + 1,
	}).Update()
	if err != nil {
		return gerror.Wrap(err, "mark financially settled order paid")
	}
	return requireFinancialSettlementAffected(updated, "mark financially settled order paid")
}

// enqueueFinancialSettlement writes the stable, deduplicated domain event in
// the caller's existing transaction.
func (s *financialSettlementServiceImpl) enqueueFinancialSettlement(
	ctx context.Context,
	prepared preparedFinancialSettlement,
	availableAt time.Time,
) error {
	payload, err := json.Marshal(struct {
		SettlementNo     string `json:"settlementNo"`
		OrderNo          string `json:"orderNo"`
		InvoiceNo        string `json:"invoiceNo"`
		CustomerNo       string `json:"customerNo"`
		CurrencyCode     string `json:"currencyCode"`
		BasisCode        string `json:"basisCode"`
		TotalAmountMinor int64  `json:"totalAmountMinor"`
		SettledAtUnixMS  int64  `json:"settledAtUnixMs"`
	}{
		prepared.settlementNo, prepared.orderNo, prepared.invoiceNo,
		prepared.customerNo, prepared.currencyCode, string(prepared.basis),
		prepared.totalAmountMinor, prepared.settledAt.UnixMilli(),
	})
	if err != nil {
		return gerror.Wrap(err, "encode financial settlement event payload")
	}
	metadata, err := json.Marshal(struct {
		SourceCount int `json:"sourceCount"`
	}{SourceCount: len(prepared.sources)})
	if err != nil {
		return gerror.Wrap(err, "encode financial settlement event metadata")
	}
	_, err = s.events.Enqueue(ctx, outbox.EnqueueInput{
		EventType: financiallySettledEventType, EventVersion: 1,
		Topic: financiallySettledEventType, AggregateType: "commerce_order",
		AggregateNo: prepared.orderNo, CorrelationNo: prepared.settlementNo,
		DeduplicationKey: "commerce-order-financially-settled:" + prepared.orderNo,
		Payload:          payload, Metadata: metadata, AvailableAt: availableAt,
	})
	if err != nil {
		return gerror.Wrap(err, "enqueue financially settled order event")
	}
	return nil
}

// deterministicFinancialSettlementNo derives the replay-stable settlement key.
func deterministicFinancialSettlementNo(orderNo string) string {
	digest := sha256Digest("financial-settlement\x00" + orderNo)
	return "fst_" + hex.EncodeToString(digest[:16])
}

// deterministicFinancialSettlementSourceNo derives one stable source identity
// from its immutable owner references.
func deterministicFinancialSettlementSourceNo(
	settlementNo string,
	sourceType FinancialSettlementSourceType,
	sourceReference string,
	paymentAllocationNo string,
	creditExposureNo string,
) string {
	digest := sha256Digest(
		settlementNo + "\x00" + string(sourceType) + "\x00" + sourceReference + "\x00" +
			paymentAllocationNo + "\x00" + creditExposureNo,
	)
	return "fss_" + hex.EncodeToString(digest[:16])
}

// sha256Digest returns the fixed cryptographic digest used by deterministic IDs.
func sha256Digest(value string) [32]byte {
	return sha256.Sum256([]byte(value))
}

// requireFinancialSettlementAffected rejects lost optimistic updates and
// unexpected multi-row writes.
func requireFinancialSettlementAffected(
	result interface{ RowsAffected() (int64, error) },
	operation string,
) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, operation+" affected rows")
	}
	if affected != 1 {
		return bizerr.NewCode(CodeSettlementIntegrity)
	}
	return nil
}

// normalizeFinancialSettlementError preserves business errors and hides
// low-level database details behind the stable unavailable code.
func normalizeFinancialSettlementError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := bizerr.As(err); ok {
		return err
	}
	return bizerr.WrapCode(err, CodeSettlementUnavailable)
}
