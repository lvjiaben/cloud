// This file converts an already provisioned local trial using a selected formal
// period. Original order/configuration evidence stays immutable; payment creates
// a new effective price and recovery is durable rather than a request-side call.

package renewal

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/commerce"
	"lina-plugin-idc-core/backend/internal/service/principal"
	"lina-plugin-idc-core/backend/internal/service/triallifecycle"
)

// NewWithTrialConversion adds the startup-owned recovery producer to the same
// renewal/payment graph. Workers and channels without it cannot convert trials.
func NewWithTrialConversion(ctx context.Context, database gdb.DB, principalReader principal.Reader,
	catalogReader catalog.CatalogReader, evaluator commerce.QuoteAdjustmentEvaluator,
	recovery triallifecycle.RecoveryScheduler) (RuntimeService, error) {
	if recovery == nil {
		return nil, gerror.New("trial conversion requires durable recovery")
	}
	runtime, err := NewWithQuoteAdjustments(ctx, database, principalReader, catalogReader, evaluator)
	if err != nil {
		return nil, err
	}
	impl, ok := runtime.(*serviceImpl)
	if !ok {
		return nil, gerror.New("unexpected renewal runtime implementation")
	}
	impl.recovery = recovery
	return runtime, nil
}

// prepareTrialConversion only accepts a still-identical published configuration.
// New versions require a separate compatibility proof, never a silent replacement.
func (s *serviceImpl) prepareTrialConversion(ctx context.Context, row *entitlementQuoteRow, ruleNo string) error {
	if s.recovery == nil || row == nil || row.ChargeTypeCode != string(catalog.ChargeTypeTrial) ||
		row.RenewalModeCode != "local_entitlement" || row.EntitlementOwnerID != "idc-resource" ||
		(row.EntitlementStatusCode != "active" && row.EntitlementStatusCode != "suspended") {
		return bizerr.NewCode(CodeCapabilityUnavailable)
	}
	var rule *struct {
		PeriodUnitCode string
		PeriodCount    int
	}
	if err := s.database.GetScan(ctx, &rule, `SELECT rule.period_unit_code,rule.period_count
 FROM plugin_idc_core_catalog_billing_rule rule
 JOIN plugin_idc_core_catalog_product product ON product.tenant_id=rule.tenant_id AND product.product_no=?
 WHERE rule.tenant_id=0 AND rule.version_no=? AND rule.billing_rule_no=?
 AND rule.charge_type_code='recurring' AND rule.status_code='active' AND rule.price_phase_code<>'renewal'
 AND product.current_version_no=rule.version_no AND product.status_code='active' AND product.deleted_at IS NULL
 AND EXISTS (SELECT 1 FROM plugin_idc_core_service_entitlement e WHERE e.tenant_id=0 AND e.entitlement_no=? AND e.customer_no=? AND e.upstream_service_no='')`, row.ProductNo, row.VersionNo, ruleNo, row.EntitlementNo, row.CustomerNo); err != nil {
		return err
	}
	if rule == nil {
		return bizerr.NewCode(CodePriceUnavailable)
	}
	row.TrialConversion = true
	row.BillingRuleNo = ruleNo
	row.PeriodUnitCode = rule.PeriodUnitCode
	row.PeriodCount = rule.PeriodCount
	// Eligibility and assembly now use the selected formal period, while the
	// persisted entitlement remains a trial until its invoice is settled.
	row.ChargeTypeCode = string(catalog.ChargeTypeRecurring)
	row.EntitlementStatusCode = "active"
	return nil
}

// requireConversionQuote revalidates the exact current version, selected formal
// schedule, currency, and price selections before confirmation or a new debit.
func (s *serviceImpl) requireConversionQuote(ctx context.Context, quoteNo string) error {
	var row *struct {
		CurrencyCode, EntitlementNo, ProductNo, VersionNo, BillingRuleNo, ConfigurationSnapshotNo string
		PurchaseCycles                                                                            int
	}
	if err := s.database.GetScan(ctx, &row, `SELECT price.currency_code,price.subject_no AS entitlement_no,
 price.product_no,price.version_no,price.billing_rule_no,price.configuration_snapshot_no,price.purchase_cycles
 FROM plugin_idc_core_service_price_snapshot price
 JOIN plugin_idc_core_service_entitlement entitlement ON entitlement.tenant_id=price.tenant_id
 AND entitlement.entitlement_no=price.subject_no AND entitlement.customer_no=price.customer_no
 WHERE price.tenant_id=0 AND price.source_quote_no=? AND price.subject_type_code='entitlement'
 AND entitlement.charge_type_code='trial'`, quoteNo); err != nil {
		return err
	}
	if row == nil {
		return nil
	}
	if s.recovery == nil {
		return bizerr.NewCode(CodeCapabilityUnavailable)
	}
	choices, err := s.priceChoices.LoadConfigurationPriceSelections(ctx, []string{row.ConfigurationSnapshotNo})
	if err != nil {
		return bizerr.NewCode(CodePriceUnavailable)
	}
	selections, found := choices[row.ConfigurationSnapshotNo]
	if !found {
		return bizerr.NewCode(CodePriceUnavailable)
	}
	lines, err := s.catalog.AssembleRenewalQuote(ctx, catalog.RenewalQuoteInput{CurrencyCode: row.CurrencyCode, TrialConversion: true,
		Lines: []catalog.RenewalQuoteLineInput{{ReferenceNo: row.EntitlementNo, ProductNo: row.ProductNo, VersionNo: row.VersionNo,
			BillingRuleNo: row.BillingRuleNo, Quantity: 1, PurchaseCycles: row.PurchaseCycles, PriceSelections: selections}}})
	if err != nil || len(lines) != 1 {
		return bizerr.NewCode(CodePriceUnavailable)
	}
	return nil
}

// finishTrialConversion runs after the local entitlement update, inside its
// transaction. Invoice settlement time is authoritative, including a replay
// after delayed payment notification; the original quote expiry is illustrative.
func (s *serviceImpl) finishTrialConversion(ctx context.Context, entitlementNo, customerNo, operationNo, previousStatus string) error {
	var row *struct {
		OriginalExpiresAt           time.Time
		SettledAt                   time.Time
		PeriodUnitCode              string
		PeriodCount, PurchaseCycles int
		BillingRuleNo               string
	}
	if err := s.database.GetScan(ctx, &row, `SELECT price.original_expires_at,invoice.settled_at,
 price.period_unit_code,price.period_count,price.purchase_cycles,price.billing_rule_no
 FROM plugin_idc_core_business_operation operation
 JOIN plugin_idc_core_service_price_snapshot price ON price.tenant_id=operation.tenant_id AND price.price_snapshot_no=operation.price_snapshot_no
 JOIN plugin_idc_core_billing_invoice invoice ON invoice.tenant_id=operation.tenant_id AND invoice.invoice_no=operation.invoice_no
 WHERE operation.tenant_id=0 AND operation.operation_no=? AND operation.subject_no=? AND operation.customer_no=?
 AND invoice.status_code IN ('paid','credit_settled')`, operationNo, entitlementNo, customerNo); err != nil {
		return err
	}
	if row == nil || row.SettledAt.IsZero() {
		return bizerr.NewCode(CodeConflict)
	}
	expiry, err := projectedExpiry(row.OriginalExpiresAt, row.SettledAt, row.PeriodUnitCode, row.PeriodCount, row.PurchaseCycles)
	if err != nil {
		return err
	}
	status := "active"
	if previousStatus == "suspended" {
		status = "suspended"
	}
	result, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_service_entitlement
 SET charge_type_code='recurring',expires_at=?,status_code=?
 WHERE tenant_id=0 AND entitlement_no=? AND customer_no=? AND charge_type_code='trial'
 AND last_renewal_operation_no=?`, expiry, status, entitlementNo, customerNo, operationNo)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return bizerr.NewCode(CodeConflict)
	}
	if status == "suspended" {
		return s.recovery.ScheduleRecovery(ctx, entitlementNo, customerNo)
	}
	return nil
}

// PayOwnedRenewalInvoice preserves business convergence when a customer pays
// through the generic finance screen instead of the trial conversion page.
func (s *serviceImpl) PayOwnedRenewalInvoice(ctx context.Context, input billing.AccountBalancePaymentInput) (*billing.AccountBalancePayment, bool, error) {
	audience, err := s.principal.Current(ctx)
	if err != nil || audience == nil || audience.Code != principal.AudienceCustomer || audience.Status != principal.AudienceStatusActive {
		return nil, true, bizerr.NewCode(CodeNotAccessible)
	}
	var row *struct{ SourceCode, OperationNo string }
	if err = s.database.GetScan(ctx, &row, `SELECT invoice.source_code,operation.operation_no
 FROM plugin_idc_core_billing_invoice invoice
 LEFT JOIN plugin_idc_core_business_operation operation ON operation.tenant_id=invoice.tenant_id AND operation.invoice_no=invoice.invoice_no
 AND operation.payer_customer_no=invoice.customer_no AND operation.operation_type_code='renewal' AND operation.subject_type_code='entitlement'
 WHERE invoice.tenant_id=0 AND invoice.invoice_no=? AND invoice.customer_no=?`, input.InvoiceNo, audience.CustomerNo); err != nil {
		return nil, true, normalizeError(err)
	}
	if row == nil || row.SourceCode != "renewal" {
		return nil, false, nil
	}
	if row.OperationNo == "" {
		return nil, true, bizerr.NewCode(CodeConflict)
	}
	settlement, err := s.PayCurrentCustomer(ctx, PayInput{OperationNo: row.OperationNo})
	if err != nil {
		return nil, true, err
	}
	payment := settlement.balancePayment
	if payment == nil || payment.Posting == nil || payment.Invoice == nil {
		return nil, true, bizerr.NewCode(CodeConflict)
	}
	return &billing.AccountBalancePayment{TransactionNo: payment.Posting.TransactionNo, InvoiceNo: payment.Invoice.InvoiceNo,
		CurrencyCode: payment.Invoice.CurrencyCode, StatusCode: string(payment.Invoice.Status), PaidAmountMinor: payment.Invoice.PaidAmountMinor,
		OutstandingAmountMinor: payment.Invoice.OutstandingAmountMinor, BalanceMinor: payment.Posting.BalanceMinor,
		AvailableBalanceMinor: payment.Posting.AvailableBalanceMinor, Reused: settlement.Reused}, true, nil
}

// finishFreeTrialConversion shares the paid conversion projection without a debit.
func (s *serviceImpl) finishFreeTrialConversion(ctx context.Context, row *confirmationRow, operationNo string) error {
	if !row.TrialConversion || row.TotalAmountMinor != 0 {
		return nil
	}
	return s.finishTrialConversion(ctx, row.EntitlementNo, row.CustomerNo, operationNo, row.EntitlementStatus)
}
