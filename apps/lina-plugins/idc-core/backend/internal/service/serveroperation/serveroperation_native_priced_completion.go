// Native priced execution uses the existing resource-owned task and completion
// receipt. Only proven resize success projects a new current catalog price.

package serveroperation

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

type nativePricedOperation struct {
	Operation                   servercommercialcap.Operation
	Target                      servercommercialcap.TargetSnapshot
	Price                       *PreparedNativePrice
	Quote                       *nativePricedQuote
	StatusCode                  string
	Version                     int64
	FinancialSettlementRequired bool
	SettlementNo                string
	FinanciallySettledAt        *time.Time
}

func (s *nativeCommercialService) loadNativePricedOperation(ctx context.Context, operationNo string) (*nativePricedOperation, error) {
	var row *struct {
		QuoteNo, CustomerNo, SubjectNo, OperationTypeCode, StatusCode, SettlementNo, ContentDigest string
		OrderNo, InvoiceNo, CurrencyCode, Parameters                                               string
		Version, ExpectedSubjectVersion                                                            int64
		FinancialSettlementRequired                                                                bool
		FinanciallySettledAt                                                                       *time.Time
		Native, ParametersValid                                                                    bool
	}
	if err := s.database.GetScan(ctx, &row, `SELECT op.quote_no,op.customer_no,op.subject_no,op.operation_type_code,op.status_code,
 op.version,op.expected_subject_version,op.financial_settlement_required,
 COALESCE(op.settlement_no,'') AS settlement_no,op.financially_settled_at,op.content_digest,
 COALESCE(op.order_no,'') AS order_no,COALESCE(op.invoice_no,'') AS invoice_no,op.currency_code,
 q.provider_parameters::TEXT AS parameters,q.provider_parameters=q.prepared_price->'Parameters' AS parameters_valid,
 q.quote_no IS NOT NULL AS native
 FROM plugin_idc_core_business_operation op
 LEFT JOIN plugin_idc_core_channel_service_quote q ON q.tenant_id=op.tenant_id AND q.quote_no=op.quote_no AND q.source_code='native'
 WHERE op.tenant_id=0 AND op.operation_no=? AND op.subject_type_code='entitlement'
 AND op.operation_type_code IN ('ip_change','resize')`, operationNo); err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	if !row.Native {
		return nil, nil
	}
	q, err := s.loadNativePricedQuote(ctx, row.QuoteNo, row.CustomerNo, "")
	if err != nil {
		return nil, err
	}
	if q == nil || q.Price.Context.EntitlementNo != row.SubjectNo || string(q.Price.Action) != row.OperationTypeCode || q.Price.Context.EntitlementVersion != row.ExpectedSubjectVersion {
		return nil, ErrOperationConflict
	}
	p := q.Price
	if !row.ParametersValid || row.CurrencyCode != p.Context.CurrencyCode || row.FinancialSettlementRequired != (p.TotalAmountMinor > 0) {
		return nil, ErrOperationConflict
	}
	content, err := nativeAccessoryIntentDigest(operationNo, q.QuoteNo, row.OrderNo, row.InvoiceNo, row.SubjectNo, string(p.Action), row.Parameters, row.FinancialSettlementRequired, row.ExpectedSubjectVersion)
	if err != nil || content != row.ContentDigest {
		return nil, ErrOperationConflict
	}
	op := servercommercialcap.Operation{OperationNo: operationNo, EntitlementNo: row.SubjectNo, CustomerNo: row.CustomerNo, LocalResourceNo: q.Target.LocalResourceNo, ChannelNo: q.Target.ChannelNo, PoolNo: q.Target.PoolNo, ExpectedEntitlementVersion: row.ExpectedSubjectVersion, Action: p.Action, Parameters: json.RawMessage(row.Parameters), ContentDigest: row.ContentDigest, QuoteNo: q.QuoteNo, Direction: p.Direction, EffectiveAt: p.EffectiveAt, TargetConfigurationNo: q.TargetConfigurationNo, TargetVersionNo: p.TargetVersionNo, TargetBillingRuleNo: p.TargetBillingRuleNo, ResourceSnapshotNo: p.ResourceSnapshotNo, IPQuantity: p.IPQuantity}
	if p.Reservation != nil {
		op.ResourceReservationNo = p.Reservation.ReservationNo
	}
	return &nativePricedOperation{Operation: op, Target: q.Target, Price: p, Quote: q, StatusCode: row.StatusCode, Version: row.Version, FinancialSettlementRequired: row.FinancialSettlementRequired, SettlementNo: row.SettlementNo, FinanciallySettledAt: row.FinanciallySettledAt}, nil
}

// nativePreparePricedExecution validates capacity before task admission. A task
// binding is not proof that upstream addresses changed, so it never consumes IPs.
func (s *nativeCommercialService) nativePreparePricedExecution(ctx context.Context, row *nativePricedOperation, taskNo string) error {
	if row == nil {
		return ErrOperationConflict
	}
	now, err := databaseNow(ctx, s.database)
	if err != nil {
		return err
	}
	if row.Price.EffectiveAt.After(now) {
		return ErrActionUnavailable
	}
	if row.FinancialSettlementRequired && (row.SettlementNo == "" || row.FinanciallySettledAt == nil) {
		return ErrOperationConflict
	}
	if row.Price.Action != serveroperationresourcecap.ActionChangeIP {
		return nil
	}
	if taskNo == "" {
		return s.bindNativePricedIP(ctx, row.Price, row.Operation.OperationNo)
	}
	holder, ok := s.prices.source.(serveroperationresourcecap.IPExecutionHold)
	if !ok {
		return ErrServiceUnavailable
	}
	held, err := holder.HoldIP(ctx, serveroperationresourcecap.IPReservationHoldRequest{ReservationNo: row.Price.Reservation.ReservationNo, ProviderTaskNo: taskNo})
	if err != nil {
		return err
	}
	if held == nil || held.ReservationNo != row.Price.Reservation.ReservationNo || held.CapacityObservationNo != row.Price.ResourceSnapshotNo || held.Quantity != row.Price.IPQuantity || held.Status != serveroperationresourcecap.IPReservationInFlight {
		return ErrOperationConflict
	}
	return nil
}

// nativeCompletePriced runs inside the unified completion transaction after the
// resource receipt has passed its task/evidence checks. Unknown outcomes retain
// the original price, reservation and intent for reconciliation.
func (s *nativeCommercialService) nativeCompletePriced(ctx context.Context, input servercommercialcap.Completion) error {
	row, err := s.loadNativePricedOperation(ctx, input.OperationNo)
	if err != nil {
		return err
	}
	if row == nil {
		return nil
	}
	if input.EntitlementNo != row.Operation.EntitlementNo {
		return ErrOperationConflict
	}
	if row.Price.Action == serveroperationresourcecap.ActionChangeIP {
		return s.nativeCompleteIP(ctx, row, input)
	}
	if input.Status != servercommercialcap.CompletionSucceeded || row.Price.Action != serveroperationresourcecap.ActionResize {
		return nil
	}
	priceNo, err := deterministicReference("npt_", row.Quote.QuoteNo)
	if err != nil {
		return err
	}
	already, err := s.database.GetValue(ctx, `SELECT current_price_snapshot_no=? AND version_no=? AND billing_rule_no=? AND configuration_snapshot_no=? FROM plugin_idc_core_service_entitlement WHERE tenant_id=0 AND entitlement_no=? AND customer_no=?`, priceNo, row.Price.TargetVersionNo, row.Price.TargetBillingRuleNo, row.Price.TargetStructuredConfigurationNo, row.Operation.EntitlementNo, row.Operation.CustomerNo)
	if err != nil {
		return err
	}
	if already.Bool() {
		return nil
	}
	itemNo, err := deterministicReference("npi_", row.Quote.QuoteNo)
	if err != nil {
		return err
	}
	p := row.Price
	target := *p
	target.SubtotalMinor, target.NetAmountMinor, target.TaxAmountMinor, target.TotalAmountMinor = p.TargetSubtotalMinor, p.TargetNetMinor, p.TargetTaxMinor, p.TargetRecurringMinor
	// The target price is recurring commercial state, separate from the one-off
	// remaining-period difference that was actually invoiced and settled.
	if err = s.persistNativeResizeTargetPrice(ctx, row.Quote, &target, priceNo, itemNo); err != nil {
		return err
	}
	if !validReference(p.TargetStructuredConfigurationNo, 40) || p.TargetQuoteNo == "" || p.TargetQuoteItemNo == "" || p.PurchaseCycles < 1 {
		return ErrOperationConflict
	}
	return execOne(ctx, s.database, `UPDATE plugin_idc_core_service_entitlement SET version_no=?,billing_rule_no=?,configuration_snapshot_no=?,current_price_snapshot_no=?,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND entitlement_no=? AND customer_no=? AND version=? AND entitlement_owner_id='idc-resource'`, p.TargetVersionNo, p.TargetBillingRuleNo, p.TargetStructuredConfigurationNo, priceNo, p.Context.EntitlementNo, p.Context.CustomerNo, p.Context.EntitlementVersion)
}

func (s *nativeCommercialService) persistNativeResizeTargetPrice(ctx context.Context, q *nativePricedQuote, p *PreparedNativePrice, priceNo, itemNo string) error {
	components := 1
	if p.TaxAmountMinor > 0 {
		components++
	}
	_, err := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_service_price_snapshot
 (tenant_id,price_snapshot_no,service_no,subject_type_code,subject_no,customer_no,phase_code,source_quote_no,source_quote_item_no,product_no,version_no,billing_rule_no,configuration_snapshot_no,structured_configuration_snapshot_no,currency_code,purchase_cycles,period_unit_code,period_count,renewal_mode_code,capability_evidence_no,capability_evidence_digest,capability_evidence_observed_at,capability_evidence_expires_at,tax_policy_no,price_includes_tax,tax_treatment_code,tax_rate_basis_points,tax_category_code,rounding_scope_code,rounding_mode_code,rounding_quantum_minor,component_count,subtotal_minor,discount_minor,net_amount_minor,tax_amount_minor,total_amount_minor,pricing_rule_digest,content_digest)
 VALUES(0,?,NULL,'entitlement',?,?,'resize_target',?,?,?,?,?,?,?,?,?,?,?,'provider_confirmed',?,?,?,?,?,?,?,?,?,?,?,?,?,?,0,?,?,?,?,?)`, priceNo, p.Context.EntitlementNo, p.Context.CustomerNo, q.QuoteNo, itemNo, p.Context.ProductNo, p.TargetVersionNo, p.TargetBillingRuleNo, q.TargetConfigurationNo, p.TargetStructuredConfigurationNo, p.Context.CurrencyCode, p.PurchaseCycles, p.Context.PeriodUnitCode, p.Context.PeriodCount, q.QuoteNo, q.ContentDigest, p.IssuedAt, p.ExpiresAt, p.Tax.PolicyNo, p.Tax.PriceIncludesTax, p.Tax.Treatment, nativePriceTaxRate(p.Tax), p.Tax.Category, p.Tax.Scope, p.Tax.Mode, p.Tax.Quantum, components, p.SubtotalMinor, p.NetAmountMinor, p.TaxAmountMinor, p.TotalAmountMinor, q.ContentDigest, q.ContentDigest)
	if err != nil {
		return err
	}
	for i, part := range []struct {
		Kind, Phase, Reference string
		Amount                 int64
	}{{"base", "periodic", q.QuoteNo, p.NetAmountMinor}, {"tax", "total", p.Tax.PolicyNo, p.TaxAmountMinor}} {
		if i == 1 && part.Amount == 0 {
			continue
		}
		no, refErr := deterministicReference("nptc_", priceNo+":"+part.Kind)
		if refErr != nil {
			return refErr
		}
		if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_service_price_component_snapshot
 (tenant_id,component_snapshot_no,price_snapshot_no,service_no,subject_type_code,subject_no,line_number,component_type_code,charge_phase_code,source_reference,quantity,unit_amount_minor,amount_minor,source_digest,content_digest)
 VALUES(0,?,?,NULL,'entitlement',?,?,?,?,?,1,?,?,?,?)`, no, priceNo, p.Context.EntitlementNo, i+1, part.Kind, part.Phase, part.Reference, part.Amount, part.Amount, q.ContentDigest, q.ContentDigest); err != nil {
			return err
		}
	}
	return nil
}

// nativeCompleteIP materializes capacity only after a source-owned outcome and
// a fresh changed IP set. Uncertain outcomes preserve the reservation untouched.
func (s *nativeCommercialService) nativeCompleteIP(ctx context.Context, row *nativePricedOperation, input servercommercialcap.Completion) error {
	if input.Status == servercommercialcap.CompletionUnknown {
		return nil
	}
	capacity, ok := s.prices.source.(serveroperationresourcecap.IPCapacity)
	if !ok {
		return ErrServiceUnavailable
	}
	if input.Status == servercommercialcap.CompletionFailed {
		released, err := capacity.ReleaseIP(ctx, serveroperationresourcecap.IPReservationReleaseRequest{ReservationNo: row.Price.Reservation.ReservationNo, ReasonCode: "provider_definitive_failure"})
		if err != nil {
			return err
		}
		if released == nil || released.ReservationNo != row.Price.Reservation.ReservationNo || (released.Status != serveroperationresourcecap.IPReservationReleased && released.Status != serveroperationresourcecap.IPReservationExpired) {
			return ErrOperationConflict
		}
		return nil
	}
	if input.Status != servercommercialcap.CompletionSucceeded {
		return ErrOperationConflict
	}
	directory, ok := s.prices.source.(serveroperationresourcecap.AccessoryDirectory)
	if !ok {
		return ErrServiceUnavailable
	}
	p := row.Price
	projection, err := directory.GetAccessories(ctx, serveroperationresourcecap.CapabilityRoute{ServiceNo: p.Context.LocalResourceNo, ChannelNo: p.Context.ChannelNo, PoolNo: p.Context.PoolNo})
	now, clockErr := databaseNow(ctx, s.database)
	if clockErr != nil {
		return clockErr
	}
	if err != nil || projection == nil || projection.ServiceNo != p.Context.LocalResourceNo || projection.SnapshotNo == "" || !projection.ExpiresAt.After(now) || projection.ObservedAt.Before(p.IssuedAt) {
		return ErrActionUnavailable
	}
	addresses, err := currentIPAddresses(projection.Items)
	if err != nil || len(addresses) != p.IPQuantity || nativePriceHash("channel-ip-old/v1", p.Context.EntitlementNo, strings.Join(addresses, ",")) == p.OldIPSetDigest {
		return ErrOperationConflict
	}
	consumed, err := capacity.ConsumeIP(ctx, serveroperationresourcecap.IPReservationConsumeRequest{ReservationNo: p.Reservation.ReservationNo, ProviderTaskNo: input.TaskNo})
	if err != nil {
		return err
	}
	if consumed == nil || consumed.ReservationNo != p.Reservation.ReservationNo || consumed.CapacityObservationNo != p.ResourceSnapshotNo || consumed.Quantity != p.IPQuantity || consumed.Status != serveroperationresourcecap.IPReservationConsumed {
		return ErrOperationConflict
	}
	return nil
}
