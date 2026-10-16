package renewal

import (
	"context"
	"strconv"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/commerce"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

type confirmationRow struct {
	TrialConversion            bool
	EntitlementStatus          string
	RequestNo                  string    `orm:"request_no"`
	CustomerNo                 string    `orm:"customer_no"`
	EntitlementNo              string    `orm:"entitlement_no"`
	ProductNo                  string    `orm:"product_no"`
	CurrencyCode               string    `orm:"currency_code"`
	QuoteNo                    string    `orm:"quote_no"`
	QuoteExpiresAt             time.Time `orm:"quote_expires_at"`
	PriceSnapshotNo            string    `orm:"price_snapshot_no"`
	PriceContentDigest         string    `orm:"price_content_digest"`
	RenewalModeCode            string    `orm:"renewal_mode_code"`
	EntitlementOwnerID         string    `orm:"entitlement_owner_id"`
	PluginID                   string    `orm:"plugin_id"`
	PluginReleaseVersion       string    `orm:"plugin_release_version"`
	ContractVersion            string    `orm:"contract_version"`
	DeclarationDigest          string    `orm:"declaration_digest"`
	RenewalMethod              string    `orm:"renewal_method"`
	ExpectedEntitlementVersion int64     `orm:"expected_entitlement_version"`
	CurrentEntitlementVersion  int64     `orm:"current_entitlement_version"`
	ProductNameZhCN            string    `orm:"product_name_zh_cn"`
	ProductNameEnUS            string    `orm:"product_name_en_us"`
	RenewalCycles              int       `orm:"purchase_cycles"`
	PeriodUnitCode             string    `orm:"period_unit_code"`
	PeriodCount                int       `orm:"period_count"`
	SubtotalMinor              int64     `orm:"subtotal_minor"`
	DiscountMinor              int64     `orm:"discount_minor"`
	NetAmountMinor             int64     `orm:"net_amount_minor"`
	TaxAmountMinor             int64     `orm:"tax_amount_minor"`
	TotalAmountMinor           int64     `orm:"total_amount_minor"`
	ProjectedExpiresAt         time.Time `orm:"projected_expires_at"`
}

type existingOperationRow struct {
	OperationNo       string    `orm:"operation_no"`
	QuoteNo           string    `orm:"quote_no"`
	InvoiceNo         string    `orm:"invoice_no"`
	EntitlementNo     string    `orm:"entitlement_no"`
	PayerCustomerNo   string    `orm:"payer_customer_no"`
	CurrencyCode      string    `orm:"currency_code"`
	StatusCode        string    `orm:"status_code"`
	ExecutionModeCode string    `orm:"execution_mode_code"`
	IdempotencyDigest string    `orm:"idempotency_digest"`
	RequestDigest     string    `orm:"request_digest"`
	TotalAmountMinor  int64     `orm:"total_amount_minor"`
	PaymentDueAt      time.Time `orm:"payment_due_at"`
}

type confirmationNumbers struct {
	InvoiceNo, InvoiceItemNo, OperationNo, EventNo, SecondEventNo string
}

type confirmationLifecycle struct {
	InvoiceStatus, OperationStatus, FirstEventStatus string
	SettledAt, ExecutionStartedAt, CompletedAt       any
	PaidMinor                                        int64
}

type confirmationDigests struct {
	Invoice, InvoiceItem, Operation, Event string
}

func validConfirmationActor(customerNo, actorType, actorReference string) bool {
	return validReference(customerNo, 40) &&
		(actorType == "customer" || actorType == "system" || actorType == "staff") &&
		validReference(actorReference, 64)
}

func replayConfirmedOperation(
	existing *existingOperationRow,
	confirmation *confirmationRow,
	idempotency string,
	requestDigest string,
	payerCustomerNo string,
) (*Operation, error) {
	if existing == nil || confirmation == nil || existing.IdempotencyDigest != idempotency ||
		existing.RequestDigest != requestDigest || existing.EntitlementNo != confirmation.EntitlementNo ||
		existing.CurrencyCode != confirmation.CurrencyCode ||
		existing.PayerCustomerNo != payerCustomerNo {
		return nil, bizerr.NewCode(CodeConflict)
	}
	return projectExistingOperation(existing, true), nil
}

func (s *serviceImpl) consumeRenewalQuoteReservations(
	ctx context.Context,
	confirmation *confirmationRow,
) error {
	adjustmentCount, err := s.database.GetValue(ctx, `
SELECT COUNT(*)
FROM plugin_idc_core_commerce_quote_adjustment_snapshot adjustment
INNER JOIN plugin_idc_core_commerce_quote_item_snapshot item
    ON item.tenant_id = adjustment.tenant_id
   AND item.quote_item_no = adjustment.quote_item_no
WHERE item.tenant_id = 0 AND item.quote_no = ?
 AND adjustment.adjustment_type_code <> ?`, confirmation.QuoteNo, commerce.QuoteAdjustmentAgentPrice)
	if err != nil {
		return gerror.Wrap(err, "count renewal quote reservations")
	}
	if adjustmentCount.Int() == 0 {
		return nil
	}
	if err = s.reservations.ConsumeQuote(
		ctx, confirmation.CustomerNo, confirmation.QuoteNo,
	); err != nil {
		return bizerr.NewCode(CodeConflict)
	}
	return nil
}

func (s *serviceImpl) newConfirmationNumbers(totalAmountMinor int64) (confirmationNumbers, error) {
	var result confirmationNumbers
	var err error
	if result.InvoiceNo, err = newNumber(s.random, "INV_"); err != nil {
		return confirmationNumbers{}, err
	}
	if result.InvoiceItemNo, err = newNumber(s.random, "IIT_"); err != nil {
		return confirmationNumbers{}, err
	}
	if result.OperationNo, err = newNumber(s.random, "ROP_"); err != nil {
		return confirmationNumbers{}, err
	}
	if result.EventNo, err = newNumber(s.random, "OEV_"); err != nil {
		return confirmationNumbers{}, err
	}
	if totalAmountMinor == 0 {
		result.SecondEventNo, err = newNumber(s.random, "OEV_")
	}
	return result, err
}

func lifecycleForConfirmation(
	confirmation *confirmationRow,
	now time.Time,
) confirmationLifecycle {
	result := confirmationLifecycle{
		InvoiceStatus: "pending_payment", OperationStatus: "awaiting_financial_settlement",
		FirstEventStatus: "awaiting_financial_settlement",
	}
	if confirmation.TotalAmountMinor != 0 {
		return result
	}
	result.InvoiceStatus = "paid"
	result.SettledAt = now
	result.OperationStatus = "ready"
	result.FirstEventStatus = "financially_settled"
	if confirmation.RenewalModeCode == "local_entitlement" {
		result.OperationStatus = "succeeded"
		result.ExecutionStartedAt = now
		result.CompletedAt = now
	}
	return result
}

func buildConfirmationDigests(
	confirmation *confirmationRow,
	numbers confirmationNumbers,
	lifecycle confirmationLifecycle,
	idempotency string,
	requestDigest string,
	actorType string,
	actorReference string,
	now time.Time,
	payerCustomerNo string,
) (confirmationDigests, error) {
	var result confirmationDigests
	var err error
	result.Invoice, err = contentDigest(struct {
		InvoiceNo, CustomerNo, QuoteNo, PriceSnapshotNo, CurrencyCode                  string
		SubtotalMinor, DiscountMinor, NetAmountMinor, TaxAmountMinor, TotalAmountMinor int64
		PaymentDueAt                                                                   time.Time
	}{numbers.InvoiceNo, payerCustomerNo, confirmation.QuoteNo,
		confirmation.PriceSnapshotNo, confirmation.CurrencyCode,
		confirmation.SubtotalMinor, confirmation.DiscountMinor,
		confirmation.NetAmountMinor, confirmation.TaxAmountMinor,
		confirmation.TotalAmountMinor, confirmation.QuoteExpiresAt})
	if err != nil {
		return confirmationDigests{}, err
	}
	result.InvoiceItem, err = contentDigest(struct {
		InvoiceItemNo, InvoiceNo, EntitlementNo, PriceSnapshotNo                       string
		SubtotalMinor, DiscountMinor, NetAmountMinor, TaxAmountMinor, TotalAmountMinor int64
	}{numbers.InvoiceItemNo, numbers.InvoiceNo, confirmation.EntitlementNo,
		confirmation.PriceSnapshotNo, confirmation.SubtotalMinor, confirmation.DiscountMinor,
		confirmation.NetAmountMinor, confirmation.TaxAmountMinor, confirmation.TotalAmountMinor})
	if err != nil {
		return confirmationDigests{}, err
	}
	expectedEntitlementVersion := confirmation.CurrentEntitlementVersion + 1
	result.Operation, err = contentDigest(struct {
		OperationNo, CustomerNo, PayerCustomerNo, EntitlementNo, CurrencyCode, PriceSnapshotNo string
		QuoteNo, InvoiceNo, Status, ExecutionMode                                              string
		EntitlementOwnerID, PluginID, PluginReleaseVersion, ContractVersion                    string
		DeclarationDigest, RenewalMethod, IdempotencyDigest, RequestDigest                     string
		ExpectedEntitlementVersion                                                             int64
	}{numbers.OperationNo, confirmation.CustomerNo, payerCustomerNo, confirmation.EntitlementNo,
		confirmation.CurrencyCode, confirmation.PriceSnapshotNo,
		confirmation.QuoteNo, numbers.InvoiceNo, lifecycle.OperationStatus,
		confirmation.RenewalModeCode, confirmation.EntitlementOwnerID, confirmation.PluginID,
		confirmation.PluginReleaseVersion, confirmation.ContractVersion,
		confirmation.DeclarationDigest, confirmation.RenewalMethod,
		idempotency, requestDigest, expectedEntitlementVersion})
	if err != nil {
		return confirmationDigests{}, err
	}
	reason := "renewal.invoice_created"
	if confirmation.TotalAmountMinor == 0 {
		reason = "renewal.zero_amount_settled"
	}
	result.Event, err = contentDigest(struct {
		EventNo, OperationNo, EntitlementNo, ToStatus, Reason, ActorType, ActorReference string
		EvidenceDigest                                                                   string
		Sequence                                                                         int
		At                                                                               time.Time
	}{numbers.EventNo, numbers.OperationNo, confirmation.EntitlementNo,
		lifecycle.FirstEventStatus, reason, actorType, actorReference,
		result.Operation, 1, now})
	if err != nil {
		return confirmationDigests{}, err
	}
	return result, nil
}

func (s *serviceImpl) ConfirmCurrentCustomer(
	ctx context.Context,
	input ConfirmInput,
) (*Operation, error) {
	audience, err := s.principal.Current(ctx)
	if err != nil || audience == nil || audience.Code != principal.AudienceCustomer ||
		audience.Status != principal.AudienceStatusActive || !validReference(audience.CustomerNo, 40) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return s.confirmOwned(
		ctx, audience.CustomerNo, audience.CustomerNo, input, "customer", audience.CustomerNo,
	)
}

// ConfirmForStaff confirms one previously previewed staff quote. The direct
// employee identifier is recorded as the actor reference and customer/service
// ownership is rechecked inside the transaction.
func (s *serviceImpl) ConfirmForStaff(
	ctx context.Context,
	customerNo string,
	input ConfirmInput,
	staffUserID int,
) (*Operation, error) {
	if staffUserID <= 0 {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return s.confirmOwned(ctx, customerNo, customerNo, input, "staff", strconv.Itoa(staffUserID))
}

func (s *serviceImpl) ConfirmForChannel(
	ctx context.Context,
	serviceOwnerCustomerNo string,
	payerCustomerNo string,
	input ConfirmInput,
	channelID string,
) (*Operation, error) {
	if !validReference(channelID, 64) || !validReference(payerCustomerNo, 40) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return s.confirmOwned(ctx, serviceOwnerCustomerNo, payerCustomerNo, input, "system", channelID)
}

func (s *serviceImpl) confirmOwned(
	ctx context.Context,
	customerNo string,
	payerCustomerNo string,
	input ConfirmInput,
	actorType string,
	actorReference string,
) (*Operation, error) {
	if !validConfirmationActor(customerNo, actorType, actorReference) ||
		!validReference(payerCustomerNo, 40) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	normalized, err := normalizeConfirmInput(input)
	if err != nil {
		return nil, err
	}
	idempotency := idempotencyDigest(normalized.IdempotencyKey)
	requestDigest, err := contentDigest(struct {
		CustomerNo, PayerCustomerNo, QuoteNo string
	}{customerNo, payerCustomerNo, normalized.QuoteNo})
	if err != nil {
		return nil, normalizeError(err)
	}
	var result *Operation
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		confirmation, loadErr := lockConfirmation(
			ctx, s.database, customerNo, normalized.QuoteNo,
		)
		if loadErr != nil {
			return loadErr
		}
		if confirmation == nil {
			return bizerr.NewCode(CodeNotAccessible)
		}
		existing, loadErr := loadOperationByQuote(ctx, s.database, normalized.QuoteNo)
		if loadErr != nil {
			return loadErr
		}
		if existing != nil {
			result, loadErr = replayConfirmedOperation(
				existing, confirmation, idempotency, requestDigest, payerCustomerNo,
			)
			return loadErr
		}
		now, nowErr := renewalDatabaseNow(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		if !confirmation.QuoteExpiresAt.After(now) ||
			confirmation.CurrentEntitlementVersion != confirmation.ExpectedEntitlementVersion {
			return bizerr.NewCode(CodeConflict)
		}
		if err := s.catalog.RequireRenewalSale(ctx, confirmation.ProductNo); err != nil {
			return bizerr.NewCode(CodePriceUnavailable)
		}
		if err := s.requireConversionQuote(ctx, confirmation.QuoteNo); err != nil {
			return err
		}
		if consumeErr := s.consumeRenewalQuoteReservations(ctx, confirmation); consumeErr != nil {
			return consumeErr
		}
		numbers, numberErr := s.newConfirmationNumbers(confirmation.TotalAmountMinor)
		if numberErr != nil {
			return numberErr
		}
		lifecycle := lifecycleForConfirmation(confirmation, now)
		expectedEntitlementVersion := confirmation.CurrentEntitlementVersion + 1
		digests, digestErr := buildConfirmationDigests(
			confirmation, numbers, lifecycle, idempotency, requestDigest,
			actorType, actorReference, now, payerCustomerNo,
		)
		if digestErr != nil {
			return digestErr
		}
		if _, err = s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_billing_invoice
    (tenant_id, invoice_no, customer_no, source_code, currency_code,
     status_code, item_count, subtotal_minor, discount_minor,
     net_amount_minor, tax_amount_minor, total_amount_minor,
     paid_amount_minor, refundable_amount_minor, refunded_amount_minor,
     source_snapshot_digest, content_digest, payment_due_at,
     issued_at, settled_at, version, created_at, updated_at)
VALUES (0, ?, ?, 'renewal', ?, ?, 1, ?, ?, ?, ?, ?, ?, 0, 0, ?, ?, ?, ?, ?, 1, ?, ?)`,
			numbers.InvoiceNo, payerCustomerNo, confirmation.CurrencyCode,
			lifecycle.InvoiceStatus,
			confirmation.SubtotalMinor, confirmation.DiscountMinor,
			confirmation.NetAmountMinor, confirmation.TaxAmountMinor,
			confirmation.TotalAmountMinor, lifecycle.PaidMinor, confirmation.PriceContentDigest,
			digests.Invoice, confirmation.QuoteExpiresAt, now, lifecycle.SettledAt, now, now); err != nil {
			return gerror.Wrap(err, "insert renewal invoice")
		}
		if _, err = s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_billing_invoice_item
    (tenant_id, invoice_item_no, invoice_no, customer_no, currency_code,
     line_number, item_type_code, source_reference,
     description_zh_cn, description_en_us, subtotal_minor, discount_minor,
     net_amount_minor, tax_amount_minor, total_amount_minor,
     refundable_cap_minor, content_digest)
VALUES (0, ?, ?, ?, ?, 1, 'renewal', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			numbers.InvoiceItemNo, numbers.InvoiceNo, payerCustomerNo,
			confirmation.CurrencyCode,
			confirmation.EntitlementNo,
			confirmation.ProductNameZhCN, confirmation.ProductNameEnUS,
			confirmation.SubtotalMinor, confirmation.DiscountMinor,
			confirmation.NetAmountMinor, confirmation.TaxAmountMinor,
			confirmation.TotalAmountMinor, confirmation.TotalAmountMinor,
			digests.InvoiceItem); err != nil {
			return gerror.Wrap(err, "insert renewal invoice item")
		}
		ownerMethod := confirmation.RenewalMethod
		if ownerMethod == "" {
			ownerMethod = confirmation.PluginID
		}
		if _, err = s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_business_operation
    (tenant_id, operation_no, operation_type_code, customer_no, payer_customer_no, subject_type_code, subject_no,
     currency_code, price_snapshot_no, quote_no, invoice_no, status_code,
     financial_settlement_required, execution_mode_code,
     resource_owner_code, channel_no, pool_no, capability_evidence_no,
     capability_evidence_digest, idempotency_digest, request_digest,
     content_digest, expected_subject_version, financially_settled_at,
     execution_started_at, completed_at, version, created_at, updated_at)
VALUES (0, ?, 'renewal', ?, ?, 'entitlement', ?, ?, ?, ?, ?, ?, TRUE, ?,
        ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`,
			numbers.OperationNo, confirmation.CustomerNo, payerCustomerNo, confirmation.EntitlementNo,
			confirmation.CurrencyCode, confirmation.PriceSnapshotNo,
			confirmation.QuoteNo, numbers.InvoiceNo, lifecycle.OperationStatus,
			confirmation.RenewalModeCode, confirmation.EntitlementOwnerID,
			confirmation.PluginReleaseVersion, confirmation.ContractVersion,
			ownerMethod, confirmation.DeclarationDigest,
			idempotency, requestDigest, digests.Operation,
			expectedEntitlementVersion, lifecycle.SettledAt, lifecycle.ExecutionStartedAt,
			lifecycle.CompletedAt,
			now, now); err != nil {
			return gerror.Wrap(err, "insert renewal business operation")
		}
		reasonCode := "renewal.invoice_created"
		if confirmation.TotalAmountMinor == 0 {
			reasonCode = "renewal.zero_amount_settled"
		}
		if _, err = s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_business_operation_event
    (tenant_id, event_no, operation_no, subject_type_code, subject_no, event_sequence,
     from_status_code, to_status_code, reason_code, actor_type_code,
     actor_reference, evidence_digest, content_digest, created_at)
VALUES (0, ?, ?, 'entitlement', ?, 1, NULL, ?, ?, ?, ?, ?, ?, ?)`, numbers.EventNo,
			numbers.OperationNo, confirmation.EntitlementNo, lifecycle.FirstEventStatus, reasonCode,
			actorType, actorReference, digests.Operation, digests.Event, now); err != nil {
			return gerror.Wrap(err, "insert renewal business operation event")
		}
		if confirmation.TotalAmountMinor == 0 {
			finalEvidence, evidenceErr := contentDigest(struct {
				OperationNo, EntitlementNo, PriceSnapshotNo, Status string
				ProjectedExpiresAt, At                              time.Time
			}{numbers.OperationNo, confirmation.EntitlementNo, confirmation.PriceSnapshotNo,
				lifecycle.OperationStatus, confirmation.ProjectedExpiresAt, now})
			if evidenceErr != nil {
				return evidenceErr
			}
			reason := "renewal.provider_ready"
			if lifecycle.OperationStatus == "succeeded" {
				reason = "renewal.local_entitlement_succeeded"
			}
			secondContent, contentErr := contentDigest(struct {
				EventNo, OperationNo, EntitlementNo, FromStatus, ToStatus, Reason string
				ActorType, ActorReference, Evidence                               string
				Sequence                                                          int
				At                                                                time.Time
			}{numbers.SecondEventNo, numbers.OperationNo, confirmation.EntitlementNo,
				"financially_settled", lifecycle.OperationStatus, reason,
				actorType, actorReference, finalEvidence, 2, now})
			if contentErr != nil {
				return contentErr
			}
			if _, err = s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_business_operation_event
    (tenant_id, event_no, operation_no, subject_type_code, subject_no, event_sequence,
     from_status_code, to_status_code, reason_code, actor_type_code,
     actor_reference, evidence_digest, content_digest, created_at)
VALUES (0, ?, ?, 'entitlement', ?, 2, 'financially_settled', ?, ?, ?, ?, ?, ?, ?)`,
				numbers.SecondEventNo, numbers.OperationNo, confirmation.EntitlementNo,
				lifecycle.OperationStatus, reason,
				actorType, actorReference, finalEvidence, secondContent, now); err != nil {
				return gerror.Wrap(err, "insert zero-amount renewal completion event")
			}
		}
		serviceUpdateSQL := `
UPDATE plugin_idc_core_service_entitlement
SET last_renewal_operation_no = ?, version = version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND entitlement_no = ? AND customer_no = ? AND version = ?`
		serviceUpdateArgs := []any{numbers.OperationNo, confirmation.EntitlementNo,
			confirmation.CustomerNo, confirmation.CurrentEntitlementVersion}
		if confirmation.TotalAmountMinor == 0 && confirmation.RenewalModeCode == "local_entitlement" {
			serviceUpdateSQL = `
UPDATE plugin_idc_core_service_entitlement
SET last_renewal_operation_no = ?, status_code = 'active',
    expires_at = ?, current_price_snapshot_no = ?, version = version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND entitlement_no = ? AND customer_no = ? AND version = ?
  AND expires_at < ?`
			serviceUpdateArgs = []any{numbers.OperationNo, confirmation.ProjectedExpiresAt,
				confirmation.PriceSnapshotNo, confirmation.EntitlementNo,
				confirmation.CustomerNo, confirmation.CurrentEntitlementVersion,
				confirmation.ProjectedExpiresAt}
		}
		updated, updateErr := s.database.Exec(ctx, serviceUpdateSQL, serviceUpdateArgs...)
		if updateErr != nil {
			return gerror.Wrap(updateErr, "attach renewal business operation")
		}
		affected, affectedErr := updated.RowsAffected()
		if affectedErr != nil || affected != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		if err := s.finishFreeTrialConversion(ctx, confirmation, numbers.OperationNo); err != nil {
			return err
		}
		result = &Operation{
			OperationNo: numbers.OperationNo, QuoteNo: confirmation.QuoteNo,
			InvoiceNo: numbers.InvoiceNo, EntitlementNo: confirmation.EntitlementNo,
			CurrencyCode: confirmation.CurrencyCode, Status: lifecycle.OperationStatus,
			RenewalMode:      confirmation.RenewalModeCode,
			TotalAmountMinor: confirmation.TotalAmountMinor,
			PaymentDueAt:     confirmation.QuoteExpiresAt,
		}
		return nil
	})
	return result, normalizeError(err)
}

func lockConfirmation(
	ctx context.Context,
	database gdb.DB,
	customerNo string,
	quoteNo string,
) (*confirmationRow, error) {
	var row *confirmationRow
	if err := database.GetScan(ctx, &row, `
SELECT request.request_no, request.customer_no, request.entitlement_no, service.product_no,
       request.currency_code, request.quote_no,
 service.charge_type_code='trial' AS trial_conversion, service.status_code AS entitlement_status,
       quote.expires_at AS quote_expires_at,
       request.price_snapshot_no,
       price.content_digest AS price_content_digest,
       price.renewal_mode_code,service.entitlement_owner_id,
       service.plugin_id,service.plugin_release_version,service.contract_version,
       service.declaration_digest,service.renewal_method,
       request.expected_entitlement_version,
       service.version AS current_entitlement_version,
       item.product_name_zh_cn, item.product_name_en_us,
       price.purchase_cycles, price.period_unit_code, price.period_count,
       price.subtotal_minor, price.discount_minor, price.net_amount_minor,
       price.tax_amount_minor, price.total_amount_minor,
       price.projected_expires_at
FROM plugin_idc_core_service_renewal_quote_request request
INNER JOIN plugin_idc_core_commerce_quote_snapshot quote
    ON quote.tenant_id = request.tenant_id AND quote.quote_no = request.quote_no
INNER JOIN plugin_idc_core_service_price_snapshot price
    ON price.tenant_id = request.tenant_id
   AND price.price_snapshot_no = request.price_snapshot_no
INNER JOIN plugin_idc_core_commerce_quote_item_snapshot item
    ON item.tenant_id = request.tenant_id
   AND item.quote_no = request.quote_no
   AND item.quote_item_no = price.source_quote_item_no
INNER JOIN plugin_idc_core_service_entitlement service
    ON service.tenant_id = request.tenant_id
   AND service.entitlement_no = request.entitlement_no
   AND service.customer_no = request.customer_no
WHERE request.tenant_id = 0 AND request.customer_no = ?
  AND request.quote_no = ?
FOR UPDATE OF service`, customerNo, quoteNo); err != nil {
		return nil, gerror.Wrap(err, "lock renewal quote confirmation")
	}
	return row, nil
}

func loadOperationByQuote(
	ctx context.Context,
	database gdb.DB,
	quoteNo string,
) (*existingOperationRow, error) {
	var row *existingOperationRow
	if err := database.GetScan(ctx, &row, `
SELECT operation.operation_no, operation.quote_no, operation.invoice_no,
       operation.subject_no AS entitlement_no, operation.currency_code, operation.status_code,
       operation.payer_customer_no,
       operation.execution_mode_code, operation.idempotency_digest,
       operation.request_digest, invoice.total_amount_minor,
       invoice.payment_due_at
FROM plugin_idc_core_business_operation operation
INNER JOIN plugin_idc_core_billing_invoice invoice
    ON invoice.tenant_id = operation.tenant_id
   AND invoice.invoice_no = operation.invoice_no
WHERE operation.tenant_id = 0 AND operation.operation_type_code = 'renewal'
  AND operation.quote_no = ?`, quoteNo); err != nil {
		return nil, gerror.Wrap(err, "load renewal operation replay")
	}
	return row, nil
}

func projectExistingOperation(row *existingOperationRow, reused bool) *Operation {
	return &Operation{
		OperationNo: row.OperationNo, QuoteNo: row.QuoteNo, InvoiceNo: row.InvoiceNo,
		EntitlementNo: row.EntitlementNo, CurrencyCode: row.CurrencyCode,
		Status: row.StatusCode, RenewalMode: row.ExecutionModeCode,
		TotalAmountMinor: row.TotalAmountMinor, PaymentDueAt: row.PaymentDueAt.UTC(),
		Reused: reused,
	}
}
