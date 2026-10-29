// This file freezes owned recurring-service renewal quotes using the effective
// paid price period. Trials must use formal conversion instead of this path.

package renewal

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/commerce"
	"lina-plugin-idc-core/backend/internal/service/marketing"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

// maximumRenewalBatchRows keeps every collection insert safely below the
// PostgreSQL parameter limit even for the widest price-component row.
const maximumRenewalBatchRows = 250

type entitlementQuoteRow struct {
	TrialConversion              bool
	EntitlementNo                string     `orm:"entitlement_no"`
	CustomerNo                   string     `orm:"customer_no"`
	CustomerStatus               string     `orm:"customer_status_code"`
	CustomerSubjectVersion       int64      `orm:"customer_subject_version"`
	ProductNo                    string     `orm:"product_no"`
	VersionNo                    string     `orm:"version_no"`
	BillingRuleNo                string     `orm:"billing_rule_no"`
	ConfigurationSnapshotNo      string     `orm:"configuration_snapshot_no"`
	PriceConfigurationSnapshotNo string     `orm:"price_configuration_snapshot_no"`
	CurrencyCode                 string     `orm:"currency_code"`
	ChargeTypeCode               string     `orm:"charge_type_code"`
	EntitlementStatusCode        string     `orm:"entitlement_status_code"`
	RenewalModeCode              string     `orm:"renewal_mode_code"`
	RenewalMethod                string     `orm:"renewal_method"`
	EntitlementOwnerID           string     `orm:"entitlement_owner_id"`
	PluginID                     string     `orm:"plugin_id"`
	PluginReleaseVersion         string     `orm:"plugin_release_version"`
	ContractVersion              string     `orm:"contract_version"`
	DeclarationDigest            string     `orm:"declaration_digest"`
	PeriodUnitCode               string     `orm:"period_unit_code"`
	PeriodCount                  int        `orm:"period_count"`
	EntitlementExpiresAt         *time.Time `orm:"entitlement_expires_at"`
	Version                      int64      `orm:"version"`
}

type renewalQuoteRequestRow struct {
	RequestNo       string `orm:"request_no"`
	RequestDigest   string `orm:"request_digest"`
	QuoteNo         string `orm:"quote_no"`
	PriceSnapshotNo string `orm:"price_snapshot_no"`
}

type persistedQuoteCharge struct {
	QuoteChargeNo string
	Charge        catalog.PurchaseQuoteCharge
}

type persistedQuoteAdjustment struct {
	QuoteAdjustmentNo string
	Adjustment        commerce.QuoteAdjustment
}

type renewalPriceComponent struct {
	ComponentNo                 string
	LineNumber                  int
	TypeCode                    string
	OptionNo                    string
	ConfigurationPriceDimension string
	ConfigurationSnapshotNo     string
	SourceReference             string
	AmountMinor                 int64
	SourceDigest                string
	ContentDigest               string
}

const renewalQuoteItemInsertSQL = `
INSERT INTO plugin_idc_core_commerce_quote_item_snapshot
    (tenant_id, quote_item_no, quote_no, line_number, currency_code,
     product_no, version_no, billing_rule_no, configuration_snapshot_no,
     product_name_zh_cn, product_name_en_us,
     purchase_cycles, charge_type_code, period_unit_code,
     period_count, fulfillment_mode_code,
     risk_decision_code,
     inventory_decision_code, route_decision_digest, tax_policy_no,
     price_includes_tax, tax_treatment_code, tax_rate_basis_points,
     tax_category_code, rounding_scope_code, rounding_mode_code,
     rounding_quantum_minor, setup_subtotal_minor, period_subtotal_minor,
     subtotal_minor, discount_minor, net_amount_minor, tax_amount_minor,
     total_amount_minor, content_digest)
VALUES (0, ?, ?, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
        'automatic', 'approved', 'not_required', ?, ?, ?, ?, ?, ?, ?, ?, ?,
        ?, ?, ?, ?, ?, ?, ?, ?)`

func (s *serviceImpl) QuoteCurrentCustomer(
	ctx context.Context,
	input QuoteInput,
) (*Quote, error) {
	audience, err := s.principal.Current(ctx)
	if err != nil || audience == nil || audience.Code != principal.AudienceCustomer ||
		audience.Status != principal.AudienceStatusActive || !validReference(audience.CustomerNo, 40) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return s.quoteOwned(ctx, audience.CustomerNo, RequestSourceCustomer, input)
}

// QuoteForStaff reuses the same immutable quote path without manufacturing a
// customer login. The management caller has already authorized the employee;
// this method still locks and verifies the supplied service ownership.
func (s *serviceImpl) QuoteForStaff(
	ctx context.Context,
	customerNo string,
	input QuoteInput,
) (*Quote, error) {
	return s.quoteOwned(ctx, customerNo, RequestSourceSystem, input)
}

func (s *serviceImpl) QuoteForChannel(
	ctx context.Context,
	serviceOwnerCustomerNo string,
	input QuoteInput,
) (*Quote, error) {
	return s.quoteOwned(ctx, serviceOwnerCustomerNo, RequestSourceSystem, input)
}

func (s *serviceImpl) quoteOwned(
	ctx context.Context,
	customerNo string,
	requestSource string,
	input QuoteInput,
) (*Quote, error) {
	normalized, err := normalizeQuoteInput(input)
	if err != nil || !validReference(customerNo, 40) ||
		(requestSource != RequestSourceCustomer && requestSource != RequestSourceSystem) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	idempotency := idempotencyDigest(normalized.IdempotencyKey)
	requestDigest, err := contentDigest(struct {
		CustomerNo, EntitlementNo, RequestSource string
		FormalBillingRuleNo                      string `json:",omitempty"`
		RenewalCycles                            int
		AssetNumbers                             []string
	}{customerNo, normalized.EntitlementNo, requestSource, normalized.FormalBillingRuleNo, normalized.RenewalCycles,
		normalized.AssetNumbers})
	if err != nil {
		return nil, normalizeError(err)
	}
	var result *Quote
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, lockErr := s.database.Exec(ctx, `
SELECT pg_advisory_xact_lock(hashtextextended(?, 0))`,
			"renewal.quote:"+customerNo+":"+requestSource+":"+idempotency); lockErr != nil {
			return gerror.Wrap(lockErr, "lock renewal quote idempotency")
		}
		existing, loadErr := loadQuoteRequestByIdempotency(
			ctx, s.database, customerNo, requestSource, idempotency,
		)
		if loadErr != nil {
			return loadErr
		}
		if existing != nil {
			if existing.RequestDigest != requestDigest {
				return bizerr.NewCode(CodeConflict)
			}
			result, loadErr = loadRenewalQuoteProjection(
				ctx, s.database, existing.RequestNo, existing.QuoteNo,
				existing.PriceSnapshotNo, true,
			)
			return loadErr
		}
		service, loadErr := lockOwnedEntitlementForQuote(
			ctx, s.database, customerNo, normalized.EntitlementNo,
		)
		if loadErr != nil {
			return loadErr
		}
		if service == nil {
			return bizerr.NewCode(CodeNotAccessible)
		}
		now, nowErr := renewalDatabaseNow(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		if normalized.FormalBillingRuleNo != "" {
			if err := s.prepareTrialConversion(ctx, service, normalized.FormalBillingRuleNo); err != nil {
				return err
			}
		}
		if eligibilityErr := validateEntitlementQuoteEligibility(service); eligibilityErr != nil {
			return eligibilityErr
		}
		cycleNumber, cycleErr := loadRenewalCycleNumber(ctx, s.database, service.EntitlementNo)
		if cycleErr != nil {
			return cycleErr
		}
		catalogLine, assembleErr := s.assembleRenewalCatalogLine(
			ctx, service, normalized.RenewalCycles,
		)
		if assembleErr != nil {
			return assembleErr
		}
		quoteNo, numberErr := newNumber(s.random, "QTE_")
		if numberErr != nil {
			return numberErr
		}
		quoteItemNo, numberErr := newNumber(s.random, "QIT_")
		if numberErr != nil {
			return numberErr
		}
		initialAmounts, amountErr := commerce.CalculatePricingAmounts(catalogLine, 0)
		if amountErr != nil {
			return bizerr.NewCode(CodePriceUnavailable)
		}
		adjustmentResult, adjustmentErr := s.adjustments.EvaluateAndReserveQuote(
			ctx, commerce.QuoteAdjustmentInput{
				QuoteNo: quoteNo, CustomerNo: customerNo, CurrencyCode: service.CurrencyCode,
				PurchasePhase: marketing.PurchasePhaseRenewal, CycleNumber: cycleNumber,
				IssuedAt: now, AssetNumbers: append([]string(nil), normalized.AssetNumbers...),
				Lines: []commerce.QuoteAdjustmentLineInput{{
					ReferenceNo: service.EntitlementNo, ProductNo: service.ProductNo,
					CategoryNo:     catalogLine.CategoryNo,
					SubtotalMinor:  initialAmounts.SubtotalMinor,
					ChargeTypeCode: catalogLine.ChargeTypeCode, PeriodUnitCode: catalogLine.PeriodUnitCode,
					PeriodCount: catalogLine.PeriodCount, Quantity: catalogLine.Quantity,
					PurchaseCycles: catalogLine.PurchaseCycles, PeriodSubtotalMinor: initialAmounts.PeriodSubtotalMinor,
				}},
			},
		)
		if adjustmentErr != nil || adjustmentResult == nil ||
			len(adjustmentResult.Adjustments) > maximumAdjustments {
			return bizerr.NewCode(CodePriceUnavailable)
		}
		_, adjustmentErr = validateRenewalAdjustments(
			service.EntitlementNo, adjustmentResult.Adjustments,
		)
		if adjustmentErr != nil {
			return adjustmentErr
		}
		catalogLine, amountErr = commerce.ApplyAgentPrice(catalogLine, adjustmentResult.Adjustments)
		if amountErr != nil {
			return bizerr.NewCode(CodePriceUnavailable)
		}
		amounts, amountErr := commerce.CalculatePricingAmountsWithAdjustments(catalogLine, adjustmentResult.Adjustments)
		if amountErr != nil {
			return bizerr.NewCode(CodePriceUnavailable)
		}
		projected, expiryErr := projectedExpiry(
			*service.EntitlementExpiresAt, now, service.PeriodUnitCode,
			service.PeriodCount, normalized.RenewalCycles,
		)
		if expiryErr != nil {
			return expiryErr
		}
		quoteLifetime := defaultQuoteLifetime
		if requestSource == RequestSourceSystem {
			quoteLifetime = automaticQuoteLifetime
		}
		expiresAt := now.Add(quoteLifetime)
		if adjustmentResult.ExpiresAt != nil {
			expiresAt = earlierTime(expiresAt, adjustmentResult.ExpiresAt.UTC())
		}
		if !expiresAt.After(now) {
			return bizerr.NewCode(CodeCapabilityUnavailable)
		}
		requestNo, numberErr := newNumber(s.random, "RRQ_")
		if numberErr != nil {
			return numberErr
		}
		priceSnapshotNo, numberErr := newNumber(s.random, "RPS_")
		if numberErr != nil {
			return numberErr
		}
		result, loadErr = s.persistRenewalQuote(
			ctx, service, catalogLine, adjustmentResult.Adjustments,
			amounts, requestNo, quoteNo, quoteItemNo, priceSnapshotNo,
			requestSource, idempotency, requestDigest, now, expiresAt, projected,
		)
		return loadErr
	})
	return result, normalizeError(err)
}

func loadQuoteRequestByIdempotency(
	ctx context.Context,
	database gdb.DB,
	customerNo string,
	requestSource string,
	idempotency string,
) (*renewalQuoteRequestRow, error) {
	var row *renewalQuoteRequestRow
	if err := database.GetScan(ctx, &row, `
SELECT request_no, request_digest, quote_no, price_snapshot_no
FROM plugin_idc_core_service_renewal_quote_request
WHERE tenant_id = 0 AND customer_no = ? AND request_source_code = ?
  AND idempotency_digest = ?`, customerNo, requestSource, idempotency); err != nil {
		return nil, gerror.Wrap(err, "load renewal quote request replay")
	}
	return row, nil
}

func lockOwnedEntitlementForQuote(
	ctx context.Context,
	database gdb.DB,
	customerNo string,
	entitlementNo string,
) (*entitlementQuoteRow, error) {
	var row *entitlementQuoteRow
	if err := database.GetScan(ctx, &row, `
SELECT service.entitlement_no AS entitlement_no, service.customer_no,
	       customer.status_code AS customer_status_code,
	       customer.version AS customer_subject_version,
	       service.product_no,
	       COALESCE(current_price.version_no, service.version_no) AS version_no,
	       COALESCE(current_price.billing_rule_no, service.billing_rule_no) AS billing_rule_no,
	       service.configuration_snapshot_no,
	       COALESCE(current_price.configuration_snapshot_no,
	                order_item.configuration_snapshot_no) AS price_configuration_snapshot_no,
       service.currency_code, service.charge_type_code, service.status_code AS entitlement_status_code,
       service.renewal_mode_code,service.renewal_method,service.entitlement_owner_id,service.plugin_id,
       service.plugin_release_version,service.contract_version,service.declaration_digest,
       COALESCE(current_price.period_unit_code, service.period_unit_code) AS period_unit_code,
       COALESCE(current_price.period_count, service.period_count) AS period_count,
       service.expires_at AS entitlement_expires_at, service.version
FROM plugin_idc_core_service_entitlement service
INNER JOIN plugin_idc_core_customer_subject customer
    ON customer.tenant_id = service.tenant_id
   AND customer.customer_no = service.customer_no
INNER JOIN plugin_idc_core_commerce_order_item order_item
    ON order_item.tenant_id = service.tenant_id
   AND order_item.order_item_no = service.order_item_no
   AND order_item.order_no = service.order_no
LEFT JOIN plugin_idc_core_service_price_snapshot current_price
    ON current_price.tenant_id = service.tenant_id
   AND current_price.price_snapshot_no = service.current_price_snapshot_no
   AND current_price.subject_type_code = 'entitlement'
   AND current_price.subject_no = service.entitlement_no
WHERE service.tenant_id = 0 AND service.entitlement_no = ?
  AND service.customer_no = ?
FOR UPDATE OF service, customer`, entitlementNo, customerNo); err != nil {
		return nil, gerror.Wrap(err, "lock owned renewal entitlement")
	}
	return row, nil
}

func validateEntitlementQuoteEligibility(service *entitlementQuoteRow) error {
	if service == nil || service.CustomerStatus != "active" || service.Version < 1 ||
		service.EntitlementExpiresAt == nil || service.PeriodUnitCode == "once" ||
		(service.EntitlementStatusCode != "active" && service.EntitlementStatusCode != "expired") {
		return bizerr.NewCode(CodeConflict)
	}
	// Trial conversion must select a formal period and resume the existing
	// resource after payment; ordinary renewal cannot perform that transition.
	if service.ChargeTypeCode != string(catalog.ChargeTypeRecurring) {
		return bizerr.NewCode(CodeCapabilityUnavailable)
	}
	if service.RenewalModeCode != "provider_confirmed" &&
		service.RenewalModeCode != "local_entitlement" {
		return bizerr.NewCode(CodeCapabilityUnavailable)
	}
	if service.EntitlementOwnerID == "" || service.PluginID == "" || service.PluginReleaseVersion == "" || service.ContractVersion == "" ||
		service.ConfigurationSnapshotNo == "" || service.PriceConfigurationSnapshotNo == "" ||
		!canonicalDigestPattern.MatchString(service.DeclarationDigest) ||
		(service.RenewalModeCode == "provider_confirmed" && service.RenewalMethod == "") ||
		(service.RenewalModeCode == "local_entitlement" && service.RenewalMethod != "") {
		return bizerr.NewCode(CodeCapabilityUnavailable)
	}
	return nil
}

func loadRenewalCycleNumber(ctx context.Context, database gdb.DB, entitlementNo string) (int, error) {
	value, err := database.GetValue(ctx, `
SELECT 2 + COUNT(*)
FROM plugin_idc_core_business_operation
WHERE tenant_id = 0 AND subject_type_code = 'entitlement' AND subject_no = ? AND operation_type_code = 'renewal'
  AND status_code = 'succeeded'`, entitlementNo)
	if err != nil {
		return 0, gerror.Wrap(err, "load renewal marketing cycle")
	}
	cycle := value.Int64()
	if cycle < 2 || cycle > int64(^uint(0)>>1) {
		return 0, bizerr.NewCode(CodeConflict)
	}
	return int(cycle), nil
}

func (s *serviceImpl) assembleRenewalCatalogLine(
	ctx context.Context,
	service *entitlementQuoteRow,
	cycles int,
) (catalog.PurchaseQuoteLine, error) {
	priceSelectionsBySnapshot, err := s.priceChoices.LoadConfigurationPriceSelections(
		ctx, []string{service.PriceConfigurationSnapshotNo},
	)
	if err != nil {
		return catalog.PurchaseQuoteLine{}, bizerr.NewCode(CodePriceUnavailable)
	}
	priceSelections, found := priceSelectionsBySnapshot[service.PriceConfigurationSnapshotNo]
	if !found {
		return catalog.PurchaseQuoteLine{}, bizerr.NewCode(CodePriceUnavailable)
	}
	requested := catalog.RenewalQuoteLineInput{
		ReferenceNo: service.EntitlementNo, ProductNo: service.ProductNo,
		VersionNo: service.VersionNo, BillingRuleNo: service.BillingRuleNo,
		Quantity: 1, PurchaseCycles: cycles,
		PriceSelections: append([]catalog.ConfigurationPriceSelection(nil), priceSelections...),
	}
	lines, err := s.catalog.AssembleRenewalQuote(ctx, catalog.RenewalQuoteInput{
		CurrencyCode: service.CurrencyCode, Lines: []catalog.RenewalQuoteLineInput{requested},
		TrialConversion: service.TrialConversion,
	})
	if err != nil || len(lines) != 1 {
		return catalog.PurchaseQuoteLine{}, bizerr.NewCode(CodePriceUnavailable)
	}
	line := lines[0]
	if line.ReferenceNo != service.EntitlementNo || line.ProductNo != service.ProductNo ||
		line.VersionNo != service.VersionNo || line.BillingRuleNo != service.BillingRuleNo ||
		line.Quantity != 1 ||
		line.PurchaseCycles != cycles || line.PeriodUnitCode != service.PeriodUnitCode ||
		line.PeriodCount != service.PeriodCount || len(line.Charges) != len(priceSelections)+1 {
		return catalog.PurchaseQuoteLine{}, bizerr.NewCode(CodePriceUnavailable)
	}
	for index, selection := range priceSelections {
		charge := line.Charges[index+1]
		if charge.KindCode != "option" ||
			charge.ConfigurationPriceDimensionNo != selection.DimensionNo ||
			charge.OptionNo != "" ||
			charge.Quantity != selection.Quantity {
			return catalog.PurchaseQuoteLine{}, bizerr.NewCode(CodePriceUnavailable)
		}
	}
	line.InventoryDecisionCode = "not_required"
	line.FulfillmentModeCode = "automatic"
	line.RiskDecisionCode = "approved"
	routeDigest, err := contentDigest(struct {
		EntitlementNo, EntitlementOwnerID, PluginID, PluginReleaseVersion, ContractVersion, DeclarationDigest, RenewalMode, RenewalMethod string
	}{service.EntitlementNo, service.EntitlementOwnerID, service.PluginID, service.PluginReleaseVersion, service.ContractVersion,
		service.DeclarationDigest, service.RenewalModeCode, service.RenewalMethod})
	if err != nil {
		return catalog.PurchaseQuoteLine{}, err
	}
	line.RouteDecisionDigest = routeDigest
	return line, nil
}

func validateRenewalAdjustments(
	entitlementNo string,
	adjustments []commerce.QuoteAdjustment,
) (int64, error) {
	var total int64
	for _, adjustment := range adjustments {
		validType := adjustment.TypeCode == commerce.QuoteAdjustmentCampaignDiscount ||
			adjustment.TypeCode == commerce.QuoteAdjustmentCoupon ||
			adjustment.TypeCode == commerce.QuoteAdjustmentVoucher ||
			adjustment.TypeCode == commerce.QuoteAdjustmentCustomerPrice ||
			adjustment.TypeCode == commerce.QuoteAdjustmentAgentPrice ||
			adjustment.TypeCode == commerce.QuoteAdjustmentManualDiscount
		if adjustment.ReferenceNo != entitlementNo || !validType || (adjustment.AmountMinor < 0 || (adjustment.AmountMinor == 0 && adjustment.TypeCode != commerce.QuoteAdjustmentAgentPrice)) ||
			!validReference(adjustment.SourceReference, 128) ||
			!canonicalDigestPattern.MatchString(adjustment.SourceVersionDigest) ||
			(adjustment.ReservationReference != "" &&
				!validReference(adjustment.ReservationReference, 128)) ||
			total > int64(^uint64(0)>>1)-adjustment.AmountMinor {
			return 0, bizerr.NewCode(CodePriceUnavailable)
		}
		total += adjustment.AmountMinor
	}
	return total, nil
}

func renewalDatabaseNow(ctx context.Context, database gdb.DB) (time.Time, error) {
	value, err := database.GetValue(ctx, `SELECT CURRENT_TIMESTAMP`)
	if err != nil {
		return time.Time{}, gerror.Wrap(err, "load renewal database time")
	}
	now := value.Time()
	if now.IsZero() {
		return time.Time{}, gerror.New("renewal database time is empty")
	}
	return now.UTC().Truncate(time.Microsecond), nil
}

func (s *serviceImpl) persistRenewalQuote(
	ctx context.Context,
	service *entitlementQuoteRow,
	line catalog.PurchaseQuoteLine,
	adjustments []commerce.QuoteAdjustment,
	amounts commerce.PricingAmounts,
	requestNo string,
	quoteNo string,
	quoteItemNo string,
	priceSnapshotNo string,
	requestSource string,
	idempotency string,
	requestDigest string,
	issuedAt time.Time,
	expiresAt time.Time,
	projectedExpiresAt time.Time,
) (*Quote, error) {
	chargeAmounts, err := commerce.CalculatePricingChargeAmounts(line)
	if err != nil || len(chargeAmounts) != len(line.Charges) {
		return nil, bizerr.NewCode(CodePriceUnavailable)
	}
	charges := make([]persistedQuoteCharge, 0, len(line.Charges))
	for _, charge := range line.Charges {
		chargeNo, numberErr := newNumber(s.random, "QCH_")
		if numberErr != nil {
			return nil, numberErr
		}
		charges = append(charges, persistedQuoteCharge{QuoteChargeNo: chargeNo, Charge: charge})
	}
	persistedAdjustments := make([]persistedQuoteAdjustment, 0, len(adjustments))
	for _, adjustment := range adjustments {
		adjustmentNo, numberErr := newNumber(s.random, "QAD_")
		if numberErr != nil {
			return nil, numberErr
		}
		persistedAdjustments = append(persistedAdjustments,
			persistedQuoteAdjustment{QuoteAdjustmentNo: adjustmentNo, Adjustment: adjustment})
	}
	components, err := s.buildRenewalPriceComponents(
		priceSnapshotNo, service.EntitlementNo, service.PriceConfigurationSnapshotNo,
		charges, chargeAmounts, persistedAdjustments,
		amounts.TaxAmountMinor,
	)
	if err != nil {
		return nil, err
	}
	pricingDigest, err := contentDigest(struct {
		Line        catalog.PurchaseQuoteLine
		Amounts     commerce.PricingAmounts
		Adjustments []commerce.QuoteAdjustment
	}{line, amounts, adjustments})
	if err != nil {
		return nil, err
	}
	eligibilityDigest, err := contentDigest(struct {
		CustomerNo                                                                     string
		CustomerSubjectVersion                                                         int64
		CustomerConfigurationSnapshotNo, PriceConfigurationSnapshotNo                  string
		RenewalMode, RenewalMethod, EntitlementOwnerID, PluginID, PluginReleaseVersion string
		ContractVersion, DeclarationDigest                                             string
	}{service.CustomerNo, service.CustomerSubjectVersion,
		service.ConfigurationSnapshotNo, service.PriceConfigurationSnapshotNo, service.RenewalModeCode,
		service.RenewalMethod, service.EntitlementOwnerID, service.PluginID, service.PluginReleaseVersion,
		service.ContractVersion, service.DeclarationDigest})
	if err != nil {
		return nil, err
	}
	inventoryDigest, err := contentDigest(struct {
		Decision, RouteDigest string
	}{"not_required", line.RouteDecisionDigest})
	if err != nil {
		return nil, err
	}
	quoteContentDigest, err := contentDigest(struct {
		QuoteNo, CustomerNo, CurrencyCode, RequestDigest  string
		Amounts                                           commerce.PricingAmounts
		PricingDigest, EligibilityDigest, InventoryDigest string
		IssuedAt, ExpiresAt                               time.Time
	}{quoteNo, service.CustomerNo, service.CurrencyCode, requestDigest, amounts,
		pricingDigest, eligibilityDigest, inventoryDigest, issuedAt, expiresAt})
	if err != nil {
		return nil, err
	}
	itemContentDigest, err := contentDigest(struct {
		QuoteItemNo, QuoteNo, EntitlementNo                           string
		CustomerConfigurationSnapshotNo, PriceConfigurationSnapshotNo string
		Line                                                          catalog.PurchaseQuoteLine
		Amounts                                                       commerce.PricingAmounts
	}{quoteItemNo, quoteNo, service.EntitlementNo, service.ConfigurationSnapshotNo,
		service.PriceConfigurationSnapshotNo, line, amounts})
	if err != nil {
		return nil, err
	}
	priceContentDigest, err := contentDigest(struct {
		PriceSnapshotNo, EntitlementNo, QuoteNo, QuoteItemNo                string
		CustomerConfigurationSnapshotNo, PriceConfigurationSnapshotNo       string
		EntitlementOwnerID, PluginID, PluginReleaseVersion, ContractVersion string
		RenewalMode, RenewalMethod, DeclarationDigest                       string
		OriginalExpiresAt, ProjectedExpiresAt                               time.Time
		Amounts                                                             commerce.PricingAmounts
		Components                                                          []renewalPriceComponent
	}{priceSnapshotNo, service.EntitlementNo, quoteNo, quoteItemNo,
		service.ConfigurationSnapshotNo, service.PriceConfigurationSnapshotNo,
		service.EntitlementOwnerID, service.PluginID, service.PluginReleaseVersion, service.ContractVersion,
		service.RenewalModeCode, service.RenewalMethod, service.DeclarationDigest,
		service.EntitlementExpiresAt.UTC(), projectedExpiresAt, amounts, components})
	if err != nil {
		return nil, err
	}
	requestContentDigest, err := contentDigest(struct {
		RequestNo, CustomerNo, EntitlementNo, CurrencyCode, RequestSource string
		RenewalCycles                                                     int
		IdempotencyDigest, RequestDigest, QuoteNo, PriceSnapshotNo        string
		ExpectedEntitlementVersion                                        int64
	}{requestNo, service.CustomerNo, service.EntitlementNo, service.CurrencyCode, requestSource,
		line.PurchaseCycles, idempotency, requestDigest, quoteNo, priceSnapshotNo, service.Version})
	if err != nil {
		return nil, err
	}
	if _, err = s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_commerce_quote_snapshot
    (tenant_id, quote_no, customer_no, customer_subject_version,
     purpose_code, source_code, currency_code, item_count,
     subtotal_minor, discount_minor, net_amount_minor, tax_amount_minor,
     total_amount_minor, input_digest, pricing_rule_digest,
     eligibility_digest, inventory_digest, content_digest,
     issued_at, expires_at, created_by)
VALUES (0, ?, ?, ?, 'renewal', ?, ?, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0)`,
		quoteNo, service.CustomerNo, service.CustomerSubjectVersion,
		requestSource, service.CurrencyCode, amounts.SubtotalMinor, amounts.DiscountMinor,
		amounts.NetAmountMinor, amounts.TaxAmountMinor, amounts.TotalAmountMinor,
		requestDigest, pricingDigest, eligibilityDigest, inventoryDigest,
		quoteContentDigest, issuedAt, expiresAt); err != nil {
		return nil, gerror.Wrap(err, "insert renewal quote")
	}
	if _, err = s.database.Exec(ctx, renewalQuoteItemInsertSQL, quoteItemNo, quoteNo, service.CurrencyCode,
		line.ProductNo, line.VersionNo, line.BillingRuleNo, service.PriceConfigurationSnapshotNo,
		line.ProductNameZhCN, line.ProductNameEnUS,
		line.PurchaseCycles, line.ChargeTypeCode, line.PeriodUnitCode, line.PeriodCount,
		line.RouteDecisionDigest,
		line.TaxPolicyNo, line.PriceIncludesTax, line.TaxTreatmentCode,
		line.TaxRateBasisPoints, line.TaxCategoryCode, line.RoundingScopeCode,
		line.RoundingModeCode, line.RoundingQuantumMinor, amounts.SetupSubtotalMinor,
		amounts.PeriodSubtotalMinor, amounts.SubtotalMinor, amounts.DiscountMinor,
		amounts.NetAmountMinor, amounts.TaxAmountMinor, amounts.TotalAmountMinor,
		itemContentDigest); err != nil {
		return nil, gerror.Wrap(err, "insert renewal quote item")
	}
	chargeRows := make([][]any, 0, len(charges))
	for index, charge := range charges {
		var configuration any
		var option any
		var dimension any
		if charge.Charge.OptionNo != "" || charge.Charge.ConfigurationPriceDimensionNo != "" {
			configuration = service.PriceConfigurationSnapshotNo
			if charge.Charge.OptionNo != "" {
				option = charge.Charge.OptionNo
			}
			if charge.Charge.ConfigurationPriceDimensionNo != "" {
				dimension = charge.Charge.ConfigurationPriceDimensionNo
			}
		}
		chargeRows = append(chargeRows, []any{
			0, charge.QuoteChargeNo, quoteItemNo, service.CurrencyCode, index + 1,
			charge.Charge.KindCode, "periodic", configuration, option, dimension,
			charge.Charge.Quantity, charge.Charge.ChargeAmountMinor, 0,
			charge.Charge.PricedAmountMinor, charge.Charge.PriceSourceDigest,
		})
	}
	if err = insertRenewalRows(ctx, s.database, `INSERT INTO plugin_idc_core_commerce_quote_charge_snapshot
    (tenant_id, quote_charge_no, quote_item_no, currency_code, line_number,
     charge_kind_code, charge_phase_code, configuration_snapshot_no,
     option_no, configuration_price_dimension_no, quantity, charge_amount_minor, setup_fee_minor,
     priced_amount_minor, price_source_digest)
	VALUES `, chargeRows); err != nil {
		return nil, gerror.Wrap(err, "insert renewal quote charges")
	}
	adjustmentRows := make([][]any, 0, len(persistedAdjustments))
	for index, adjustment := range persistedAdjustments {
		adjustmentDigest, digestErr := contentDigest(adjustment.Adjustment)
		if digestErr != nil {
			return nil, digestErr
		}
		var reservation any
		if adjustment.Adjustment.ReservationReference != "" {
			reservation = adjustment.Adjustment.ReservationReference
		}
		adjustmentRows = append(adjustmentRows, []any{
			0, adjustment.QuoteAdjustmentNo, quoteItemNo, service.CurrencyCode,
			index + 1, adjustment.Adjustment.TypeCode, adjustment.Adjustment.SourceReference,
			adjustment.Adjustment.SourceVersionDigest, reservation,
			adjustment.Adjustment.AmountMinor, adjustmentDigest, adjustment.Adjustment.OriginalPeriodAmountMinor, adjustment.Adjustment.TargetPeriodAmountMinor,
		})
	}
	if err = insertRenewalRows(ctx, s.database, `INSERT INTO plugin_idc_core_commerce_quote_adjustment_snapshot
    (tenant_id, quote_adjustment_no, quote_item_no, currency_code,
     application_order, adjustment_type_code, source_reference,
     source_version_digest, reservation_reference, amount_minor, content_digest, original_period_amount_minor, target_period_amount_minor)
	VALUES `, adjustmentRows); err != nil {
		return nil, gerror.Wrap(err, "insert renewal quote adjustments")
	}
	if _, err = s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_service_price_snapshot
    (tenant_id, price_snapshot_no, subject_type_code, subject_no, customer_no, phase_code,
     source_quote_no, source_quote_item_no, product_no, version_no,
     billing_rule_no, configuration_snapshot_no, structured_configuration_snapshot_no, currency_code,
     purchase_cycles, period_unit_code, period_count, renewal_mode_code,
     capability_evidence_no, capability_evidence_digest,
     capability_evidence_observed_at, capability_evidence_expires_at,
     original_expires_at, projected_expires_at, tax_policy_no,
     price_includes_tax, tax_treatment_code, tax_rate_basis_points,
     tax_category_code, rounding_scope_code, rounding_mode_code,
     rounding_quantum_minor, component_count, subtotal_minor,
     discount_minor, net_amount_minor, tax_amount_minor, total_amount_minor,
     pricing_rule_digest, content_digest)
VALUES (0, ?, 'entitlement', ?, ?, 'renewal', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
        ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		priceSnapshotNo, service.EntitlementNo, service.CustomerNo, quoteNo, quoteItemNo,
		line.ProductNo, line.VersionNo, line.BillingRuleNo, service.PriceConfigurationSnapshotNo,
		service.ConfigurationSnapshotNo,
		service.CurrencyCode, line.PurchaseCycles, line.PeriodUnitCode, line.PeriodCount,
		service.RenewalModeCode, service.PluginID+"@"+service.PluginReleaseVersion,
		service.DeclarationDigest, issuedAt, expiresAt,
		service.EntitlementExpiresAt.UTC(), projectedExpiresAt, line.TaxPolicyNo,
		line.PriceIncludesTax, line.TaxTreatmentCode, line.TaxRateBasisPoints,
		line.TaxCategoryCode, line.RoundingScopeCode, line.RoundingModeCode,
		line.RoundingQuantumMinor, len(components), amounts.SubtotalMinor,
		amounts.DiscountMinor, amounts.NetAmountMinor, amounts.TaxAmountMinor,
		amounts.TotalAmountMinor, pricingDigest, priceContentDigest); err != nil {
		return nil, gerror.Wrap(err, "insert renewal entitlement price snapshot")
	}
	componentRows := make([][]any, 0, len(components))
	for _, component := range components {
		var option any
		var dimension any
		var configuration any
		if component.OptionNo != "" {
			option = component.OptionNo
		}
		if component.ConfigurationPriceDimension != "" {
			dimension = component.ConfigurationPriceDimension
			configuration = component.ConfigurationSnapshotNo
		}
		componentRows = append(componentRows, []any{
			0, component.ComponentNo, priceSnapshotNo, "entitlement", service.EntitlementNo,
			component.LineNumber, component.TypeCode, priceComponentPhase(component.TypeCode),
			option, dimension, configuration, component.SourceReference, 1,
			component.AmountMinor, component.AmountMinor, component.SourceDigest,
			component.ContentDigest,
		})
	}
	if err = insertRenewalRows(ctx, s.database, `INSERT INTO plugin_idc_core_service_price_component_snapshot
    (tenant_id, component_snapshot_no, price_snapshot_no, subject_type_code, subject_no,
     line_number, component_type_code, charge_phase_code, option_no,
     configuration_price_dimension_no, configuration_snapshot_no,
     source_reference, quantity, unit_amount_minor, amount_minor,
     source_digest, content_digest)
	VALUES `, componentRows); err != nil {
		return nil, gerror.Wrap(err, "insert renewal price components")
	}
	if _, err = s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_service_renewal_quote_request
    (tenant_id, request_no, customer_no, entitlement_no, currency_code,
     request_source_code, renewal_cycles, idempotency_digest, request_digest,
     quote_no, price_snapshot_no, expected_entitlement_version, content_digest)
VALUES (0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, requestNo, service.CustomerNo,
		service.EntitlementNo, service.CurrencyCode, requestSource, line.PurchaseCycles,
		idempotency, requestDigest, quoteNo, priceSnapshotNo, service.Version,
		requestContentDigest); err != nil {
		return nil, gerror.Wrap(err, "insert renewal quote request")
	}
	return &Quote{
		RequestNo: requestNo, QuoteNo: quoteNo, PriceSnapshotNo: priceSnapshotNo,
		EntitlementNo: service.EntitlementNo, CurrencyCode: service.CurrencyCode,
		RenewalCycles: line.PurchaseCycles, RenewalMode: service.RenewalModeCode,
		OriginalExpiresAt:  service.EntitlementExpiresAt.UTC(),
		ProjectedExpiresAt: projectedExpiresAt, SubtotalMinor: amounts.SubtotalMinor,
		DiscountMinor: amounts.DiscountMinor, NetAmountMinor: amounts.NetAmountMinor,
		TaxAmountMinor: amounts.TaxAmountMinor, TotalAmountMinor: amounts.TotalAmountMinor,
		IssuedAt: issuedAt, ExpiresAt: expiresAt,
	}, nil
}

// insertRenewalRows persists quote details in fixed-size collection writes; an
// empty optional collection does not issue a database statement.
func insertRenewalRows(
	ctx context.Context,
	database gdb.DB,
	prefix string,
	rows [][]any,
) error {
	batches, err := buildRenewalBatchStatements(prefix, rows)
	if err != nil {
		return err
	}
	for _, batch := range batches {
		if _, err = database.Exec(ctx, batch.statement, batch.args...); err != nil {
			return err
		}
	}
	return nil
}

type renewalBatchStatement struct {
	statement string
	args      []any
}

// buildRenewalBatchStatements applies the same fixed boundary used by the
// database writer. Keeping the partitioning pure makes the round-trip count
// directly testable without replacing the database layer with a fragile mock.
func buildRenewalBatchStatements(prefix string, rows [][]any) ([]renewalBatchStatement, error) {
	batches := make([]renewalBatchStatement, 0, (len(rows)+maximumRenewalBatchRows-1)/maximumRenewalBatchRows)
	for start := 0; start < len(rows); start += maximumRenewalBatchRows {
		end := start + maximumRenewalBatchRows
		if end > len(rows) {
			end = len(rows)
		}
		statement, args, err := buildRenewalBatchStatement(prefix, rows[start:end])
		if err != nil {
			return nil, err
		}
		batches = append(batches, renewalBatchStatement{statement: statement, args: args})
	}
	return batches, nil
}

// buildRenewalBatchStatement creates one parameterized VALUES statement for a
// homogeneous bounded row set and rejects inconsistent widths before writing.
func buildRenewalBatchStatement(prefix string, rows [][]any) (string, []any, error) {
	if strings.TrimSpace(prefix) == "" || len(rows) == 0 || len(rows[0]) == 0 {
		return "", nil, bizerr.NewCode(CodeInvalid)
	}
	columnCount := len(rows[0])
	args := make([]any, 0, len(rows)*columnCount)
	var statement strings.Builder
	statement.Grow(len(prefix) + len(rows)*(columnCount*2+3))
	statement.WriteString(prefix)
	for index, row := range rows {
		if len(row) != columnCount {
			return "", nil, bizerr.NewCode(CodeInvalid)
		}
		if index > 0 {
			statement.WriteByte(',')
		}
		statement.WriteByte('(')
		for column := range row {
			if column > 0 {
				statement.WriteByte(',')
			}
			statement.WriteByte('?')
		}
		statement.WriteByte(')')
		args = append(args, row...)
	}
	return statement.String(), args, nil
}

func (s *serviceImpl) buildRenewalPriceComponents(
	priceSnapshotNo string,
	entitlementNo string,
	priceConfigurationSnapshotNo string,
	charges []persistedQuoteCharge,
	chargeAmounts []commerce.PricingChargeAmount,
	adjustments []persistedQuoteAdjustment,
	taxMinor int64,
) ([]renewalPriceComponent, error) {
	components := make([]renewalPriceComponent, 0, len(charges)+len(adjustments)+1)
	lineNumber := 1
	for index, charge := range charges {
		amount := chargeAmounts[index].SetupAmountMinor + chargeAmounts[index].PeriodicAmountMinor
		componentNo, err := newNumber(s.random, "RPC_")
		if err != nil {
			return nil, err
		}
		component := renewalPriceComponent{
			ComponentNo: componentNo, LineNumber: lineNumber,
			TypeCode:        charge.Charge.KindCode,
			SourceReference: charge.QuoteChargeNo, AmountMinor: amount,
			SourceDigest: charge.Charge.PriceSourceDigest,
		}
		component.OptionNo = charge.Charge.OptionNo
		component.ConfigurationPriceDimension = charge.Charge.ConfigurationPriceDimensionNo
		if component.ConfigurationPriceDimension != "" {
			component.ConfigurationSnapshotNo = priceConfigurationSnapshotNo
		}
		component.ContentDigest, err = contentDigest(struct {
			PriceSnapshotNo, EntitlementNo string
			Component                      renewalPriceComponent
		}{priceSnapshotNo, entitlementNo, component})
		if err != nil {
			return nil, err
		}
		components = append(components, component)
		lineNumber++
	}
	for _, adjustment := range adjustments {
		componentNo, err := newNumber(s.random, "RPC_")
		if err != nil {
			return nil, err
		}
		component := renewalPriceComponent{
			ComponentNo: componentNo, LineNumber: lineNumber, TypeCode: "discount",
			SourceReference: adjustment.Adjustment.SourceReference,
			AmountMinor:     adjustment.Adjustment.AmountMinor,
			SourceDigest:    adjustment.Adjustment.SourceVersionDigest,
		}
		component.ContentDigest, err = contentDigest(struct {
			PriceSnapshotNo, EntitlementNo string
			Component                      renewalPriceComponent
		}{priceSnapshotNo, entitlementNo, component})
		if err != nil {
			return nil, err
		}
		components = append(components, component)
		lineNumber++
	}
	if taxMinor > 0 {
		componentNo, err := newNumber(s.random, "RPC_")
		if err != nil {
			return nil, err
		}
		taxDigest, err := contentDigest(struct {
			PriceSnapshotNo, EntitlementNo string
			TaxMinor                       int64
		}{priceSnapshotNo, entitlementNo, taxMinor})
		if err != nil {
			return nil, err
		}
		component := renewalPriceComponent{
			ComponentNo: componentNo, LineNumber: lineNumber, TypeCode: "tax",
			SourceReference: "quote_tax:" + priceSnapshotNo, AmountMinor: taxMinor,
			SourceDigest: taxDigest,
		}
		component.ContentDigest, err = contentDigest(struct {
			PriceSnapshotNo, EntitlementNo string
			Component                      renewalPriceComponent
		}{priceSnapshotNo, entitlementNo, component})
		if err != nil {
			return nil, err
		}
		components = append(components, component)
	}
	return components, nil
}

func priceComponentPhase(componentType string) string {
	if componentType == "discount" || componentType == "tax" {
		return "total"
	}
	return "periodic"
}

func loadRenewalQuoteProjection(
	ctx context.Context,
	database gdb.DB,
	requestNo string,
	quoteNo string,
	priceSnapshotNo string,
	reused bool,
) (*Quote, error) {
	var row struct {
		RequestNo          string    `orm:"request_no"`
		QuoteNo            string    `orm:"quote_no"`
		PriceSnapshotNo    string    `orm:"price_snapshot_no"`
		EntitlementNo      string    `orm:"entitlement_no"`
		CurrencyCode       string    `orm:"currency_code"`
		RenewalCycles      int       `orm:"renewal_cycles"`
		RenewalMode        string    `orm:"renewal_mode_code"`
		OriginalExpiresAt  time.Time `orm:"original_expires_at"`
		ProjectedExpiresAt time.Time `orm:"projected_expires_at"`
		SubtotalMinor      int64     `orm:"subtotal_minor"`
		DiscountMinor      int64     `orm:"discount_minor"`
		NetAmountMinor     int64     `orm:"net_amount_minor"`
		TaxAmountMinor     int64     `orm:"tax_amount_minor"`
		TotalAmountMinor   int64     `orm:"total_amount_minor"`
		IssuedAt           time.Time `orm:"issued_at"`
		ExpiresAt          time.Time `orm:"expires_at"`
	}
	if err := database.GetScan(ctx, &row, `
SELECT request.request_no, quote.quote_no, price.price_snapshot_no,
       request.entitlement_no, request.currency_code,
       request.renewal_cycles, price.renewal_mode_code,
       price.original_expires_at, price.projected_expires_at,
       price.subtotal_minor, price.discount_minor, price.net_amount_minor,
       price.tax_amount_minor, price.total_amount_minor,
       quote.issued_at, quote.expires_at
FROM plugin_idc_core_service_renewal_quote_request request
INNER JOIN plugin_idc_core_commerce_quote_snapshot quote
    ON quote.tenant_id = request.tenant_id AND quote.quote_no = request.quote_no
INNER JOIN plugin_idc_core_service_price_snapshot price
    ON price.tenant_id = request.tenant_id
   AND price.price_snapshot_no = request.price_snapshot_no
WHERE request.tenant_id = 0 AND request.request_no = ?
  AND request.quote_no = ? AND request.price_snapshot_no = ?`,
		requestNo, quoteNo, priceSnapshotNo); err != nil {
		return nil, gerror.Wrap(err, "load renewal quote projection")
	}
	if row.RequestNo == "" {
		return nil, bizerr.NewCode(CodeConflict)
	}
	return &Quote{
		RequestNo: row.RequestNo, QuoteNo: row.QuoteNo, PriceSnapshotNo: row.PriceSnapshotNo,
		EntitlementNo: row.EntitlementNo, CurrencyCode: row.CurrencyCode,
		RenewalCycles: row.RenewalCycles, RenewalMode: row.RenewalMode,
		OriginalExpiresAt:  row.OriginalExpiresAt.UTC(),
		ProjectedExpiresAt: row.ProjectedExpiresAt.UTC(),
		SubtotalMinor:      row.SubtotalMinor, DiscountMinor: row.DiscountMinor,
		NetAmountMinor: row.NetAmountMinor, TaxAmountMinor: row.TaxAmountMinor,
		TotalAmountMinor: row.TotalAmountMinor, IssuedAt: row.IssuedAt.UTC(),
		ExpiresAt: row.ExpiresAt.UTC(), Reused: reused,
	}, nil
}
