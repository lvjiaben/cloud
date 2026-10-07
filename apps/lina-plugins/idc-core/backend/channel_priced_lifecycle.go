package backend

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/commerce"
	"lina-plugin-idc-core/backend/internal/service/serveroperation"
)

const channelPricedQuoteTTL = 15 * time.Minute

const channelPricedBusinessOperationInsertSQL = `INSERT INTO plugin_idc_core_business_operation(
tenant_id,operation_no,operation_type_code,customer_no,payer_customer_no,
service_no,subject_type_code,subject_no,currency_code,price_snapshot_no,quote_no,order_no,invoice_no,
status_code,financial_settlement_required,execution_mode_code,resource_owner_code,channel_no,pool_no,
provider_task_no,capability_evidence_no,capability_evidence_digest,idempotency_digest,request_digest,
content_digest,expected_service_version,expected_subject_version,last_error_code,version,created_at,updated_at)
VALUES(0,?,?,?, ?,NULL,'entitlement',?,?,?,?,?,?, ?,?,'provider_confirmed','idc-resource',?,?,
NULL,?,?,?,?,?,NULL,?,'',1,?,?)`

const channelPricedServiceOperationInsertSQL = `INSERT INTO plugin_idc_core_channel_service_operation(
tenant_id,operation_no,channel_id,payer_customer_no,service_owner_customer_no,service_no,
action_code,parameters,resource_channel_no,resource_pool_no,expected_service_version,
expected_expires_at,provider_task_no,status_code,failure_code,request_digest,
task_evidence_digest,output_bindings,observed_at,completed_at,version,created_at,updated_at)
VALUES(0,?,?,?,?,?,?,?::JSONB,?,?,?,NULL,NULL,?,'',?,?,'[]'::JSONB,?,NULL,1,?,?)
ON CONFLICT (operation_no) DO NOTHING`

const channelPricedQuoteInsertSQL = `INSERT INTO plugin_idc_core_channel_service_quote(
tenant_id,quote_no,channel_id,payer_customer_no,service_owner_customer_no,service_no,
action_code,currency_code,direction_code,current_configuration_snapshot_no,
target_configuration_snapshot_no,target_version_no,target_billing_rule_no,
current_recurring_amount_minor,target_recurring_amount_minor,target_subtotal_minor,
target_net_amount_minor,target_tax_amount_minor,subtotal_minor,net_amount_minor,
tax_amount_minor,amount_minor,provider_parameters,resource_channel_no,resource_pool_no,
resource_reservation_no,resource_snapshot_no,ip_quantity,maximum_snapshot_age_seconds,
policy_no,policy_version,old_ip_set_digest,expected_service_version,customer_subject_version,
payer_customer_subject_version,idempotency_digest,request_digest,pricing_rule_digest,
content_digest,effective_at,issued_at,expires_at)
VALUES(0,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?::JSONB,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`

type channelResizePreviewParameters struct {
	Revision     string                                 `json:"revision"`
	TermNo       string                                 `json:"termNo"`
	CurrencyCode string                                 `json:"currencyCode"`
	Selections   []catalog.PublicConfigurationSelection `json:"selections"`
}

type channelPricedConfirmParameters struct {
	QuoteNo          string `json:"quoteNo"`
	SettlementMethod string `json:"settlementMethod"`
}

type channelPricedContext struct {
	ServiceNo, OwnerCustomerNo, PayerCustomerNo, CurrencyCode                            string
	ProductNo, VersionNo, BillingRuleNo, PriceConfigurationNo, StructuredConfigurationNo string
	PublicSlug, ProductNameZhCN, ProductNameEnUS, PeriodUnitCode                         string
	OwnershipMode, ExternalCustomerID, SourceOrderNo                                     string
	PeriodCount                                                                          int
	ServiceVersion, CustomerVersion, PayerCustomerVersion                                int64
	StartedAt                                                                            time.Time
	ExpiresAt                                                                            *time.Time
}

type channelTaxContext struct {
	PolicyNo, Treatment, Category, Scope, Mode string
	Rate                                       *int
	Quantum                                    int64
	PriceIncludesTax                           bool
}

type channelPricedQuoteRow struct {
	QuoteNo, ChannelID, PayerCustomerNo, OwnerCustomerNo, ServiceNo, ActionCode  string
	CurrencyCode, Direction, CurrentConfigurationNo, TargetConfigurationNo       string
	TargetVersionNo, TargetBillingRuleNo, ProviderParameters                     string
	ResourceChannelNo, ResourcePoolNo, ReservationNo, ResourceSnapshotNo         string
	PolicyNo, OldIPSetDigest, IdempotencyDigest, RequestDigest                   string
	PricingRuleDigest, ContentDigest                                             string
	CurrentRecurringMinor, TargetRecurringMinor                                  int64
	TargetSubtotalMinor, TargetNetMinor, TargetTaxMinor                          int64
	SubtotalMinor, NetAmountMinor, TaxAmountMinor, AmountMinor                   int64
	IPQuantity, MaximumSnapshotAgeSeconds                                        int
	PolicyVersion, ExpectedServiceVersion, CustomerVersion, PayerCustomerVersion int64
	EffectiveAt, IssuedAt, ExpiresAt                                             time.Time
}

func (runtime *channelCommerceRuntime) channelPricedLifecycle(
	ctx context.Context,
	input channelcommercecap.LifecycleInput,
	service *channelServiceRow,
) (*channelcommercecap.LifecycleResult, error) {
	if runtime == nil || runtime.catalog == nil || runtime.serverSource == nil ||
		!channelOperationStatusAllowed(input.Operation, service.StatusCode) {
		return nil, bizerr.NewCode(commerce.CodeConflict)
	}
	if input.Confirmed {
		return runtime.confirmChannelPricedOperation(ctx, input, service)
	}
	if input.Operation == "reconfigure" {
		return runtime.previewChannelResize(ctx, input, service)
	}
	return runtime.previewChannelIPChange(ctx, input, service)
}

func (runtime *channelCommerceRuntime) loadChannelPricedContext(
	ctx context.Context,
	service *channelServiceRow,
	channelID string,
	lock bool,
) (*channelPricedContext, error) {
	query := `
SELECT entitlement.entitlement_no AS service_no,
 entitlement.customer_no AS owner_customer_no,attribution.payer_customer_no,
 entitlement.currency_code,entitlement.product_no,entitlement.version_no,
 entitlement.billing_rule_no,
 COALESCE(current_price.configuration_snapshot_no,order_item.configuration_snapshot_no) AS price_configuration_no,
 entitlement.configuration_snapshot_no AS structured_configuration_no,
 product.public_slug,product.name_zh_cn AS product_name_zh_cn,product.name_en_us AS product_name_en_us,
 entitlement.period_unit_code,entitlement.period_count,entitlement.version AS service_version,
 customer.version AS customer_version,payer.version AS payer_customer_version,
 entitlement.started_at,entitlement.expires_at,
 attribution.ownership_mode,attribution.external_customer_id,entitlement.order_no AS source_order_no
FROM plugin_idc_core_service_entitlement entitlement
JOIN plugin_idc_core_commerce_order_item order_item
 ON order_item.tenant_id=entitlement.tenant_id AND order_item.order_no=entitlement.order_no
 AND order_item.order_item_no=entitlement.order_item_no
JOIN plugin_idc_core_catalog_product product
 ON product.tenant_id=entitlement.tenant_id AND product.product_no=entitlement.product_no
JOIN plugin_idc_core_customer_subject customer
 ON customer.tenant_id=entitlement.tenant_id AND customer.customer_no=entitlement.customer_no
JOIN plugin_idc_core_channel_order_attribution attribution
 ON attribution.tenant_id=entitlement.tenant_id AND attribution.order_no=entitlement.order_no
JOIN plugin_idc_core_customer_subject payer
 ON payer.tenant_id=attribution.tenant_id AND payer.customer_no=attribution.payer_customer_no
LEFT JOIN plugin_idc_core_service_price_snapshot current_price
 ON current_price.tenant_id=entitlement.tenant_id
 AND current_price.price_snapshot_no=entitlement.current_price_snapshot_no
 AND current_price.subject_type_code='entitlement'
 AND current_price.subject_no=entitlement.entitlement_no
WHERE entitlement.tenant_id=0 AND entitlement.entitlement_no=?
 AND entitlement.customer_no=? AND attribution.channel_id=?`
	if lock {
		query += " FOR UPDATE OF entitlement,customer"
	}
	var row *channelPricedContext
	if err := runtime.database.GetScan(ctx, &row, query, service.ServiceNo,
		service.ServiceOwnerCustomerNo, channelID); err != nil {
		return nil, gerror.Wrap(err, "load channel priced service")
	}
	if row == nil || row.ServiceVersion != service.Version || row.PayerCustomerNo != service.PayerCustomerNo ||
		row.CustomerVersion < 1 || row.PayerCustomerVersion < 1 ||
		row.PriceConfigurationNo == "" || row.PublicSlug == "" || row.PeriodUnitCode == "once" ||
		row.ExpiresAt == nil || !row.ExpiresAt.After(row.StartedAt) {
		return nil, bizerr.NewCode(commerce.CodeConflict)
	}
	return row, nil
}

func (runtime *channelCommerceRuntime) resolveChannelRoute(
	ctx context.Context,
	service *channelServiceRow,
) (*serveroperationresourcecap.EntitlementRoute, error) {
	directory, ok := runtime.serverSource.(serveroperationresourcecap.EntitlementRouteDirectory)
	if !ok {
		return nil, serveroperationresourcecap.ErrSourceUnavailable
	}
	route, err := directory.ResolveEntitlementRoute(ctx, serveroperationresourcecap.EntitlementRouteRequest{
		ServiceNo: service.ServiceNo, CustomerNo: service.ServiceOwnerCustomerNo,
	})
	if err != nil || route == nil || route.ServiceNo != service.ServiceNo ||
		route.ChannelNo == "" || route.PoolNo == "" {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	return route, nil
}

func decodeStrictObject(value json.RawMessage, target any) error {
	decoder := json.NewDecoder(strings.NewReader(string(value)))
	decoder.DisallowUnknownFields()
	if len(value) == 0 || decoder.Decode(target) != nil {
		return bizerr.NewCode(commerce.CodeInvalid)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return bizerr.NewCode(commerce.CodeInvalid)
	}
	return nil
}

func (runtime *channelCommerceRuntime) previewChannelResize(
	ctx context.Context,
	input channelcommercecap.LifecycleInput,
	service *channelServiceRow,
) (*channelcommercecap.LifecycleResult, error) {
	var parameters channelResizePreviewParameters
	if decodeStrictObject(input.Parameters, &parameters) != nil ||
		parameters.CurrencyCode != service.CurrencyCode || len(parameters.Selections) == 0 {
		return nil, bizerr.NewCode(commerce.CodeInvalid)
	}
	requestDigest, err := channelDigest(struct {
		ChannelID, ServiceNo, Operation, Revision, TermNo, Currency string
		Selections                                                  []catalog.PublicConfigurationSelection
	}{input.ChannelID, service.ServiceNo, input.Operation, parameters.Revision,
		parameters.TermNo, parameters.CurrencyCode, parameters.Selections})
	if err != nil {
		return nil, err
	}
	idempotencyDigest := channelHash(input.ChannelID, input.Operation, input.IdempotencyKey)
	if replay, replayErr := runtime.loadChannelPricedQuoteReplay(ctx, input.ChannelID,
		idempotencyDigest, requestDigest); replayErr != nil || replay != nil {
		return projectChannelPricedQuote(replay, replay != nil), replayErr
	}
	contextRow, err := runtime.loadChannelPricedContext(ctx, service, input.ChannelID, false)
	if err != nil {
		return nil, err
	}
	route, err := runtime.resolveChannelRoute(ctx, service)
	if err != nil {
		return nil, err
	}
	publicQuote, err := runtime.catalog.QuoteConfiguration(ctx, catalog.PublicConfigurationQuoteInput{
		PublicSlug: contextRow.PublicSlug, Revision: parameters.Revision,
		CurrencyCode: parameters.CurrencyCode, Selections: parameters.Selections,
	})
	if err != nil || publicQuote == nil {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	term := selectChannelResizeTerm(publicQuote.Terms, parameters.TermNo,
		contextRow.PeriodUnitCode, contextRow.PeriodCount)
	if term == nil || term.TotalAmountMinor == nil || term.NetAmountMinor == nil ||
		term.TaxAmountMinor == nil || term.Status != catalog.PublicQuoteFinal {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	resolver, err := catalog.ServerResizeResolverFromPublicReader(runtime.catalog)
	if err != nil {
		return nil, err
	}
	resolved, err := resolver.ResolveServerResizeConfiguration(ctx,
		catalog.ServerResizeConfigurationInput{PublicSlug: contextRow.PublicSlug,
			Revision: parameters.Revision, TermNo: term.TermNo,
			CurrencyCode: parameters.CurrencyCode, PurchaseCycles: 1,
			Selections: parameters.Selections})
	if err != nil || resolved == nil || resolved.ProductNo != contextRow.ProductNo {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	var currentItems []catalog.PurchaseConfigurationItem
	if err = runtime.database.GetScan(ctx, &currentItems, `SELECT group_no,option_no,group_code,option_code,
component_type_code,group_name_zh_cn,group_name_en_us,option_name_zh_cn,option_name_en_us,
normalized_value::TEXT AS normalized_value,quantity,sort_order
FROM plugin_idc_core_commerce_configuration_snapshot_item
WHERE tenant_id=0 AND configuration_snapshot_no=? ORDER BY sort_order,id`, contextRow.PriceConfigurationNo); err != nil {
		return nil, err
	}
	providerParameters, err := serveroperation.PrepareChannelResize(currentItems, resolved.Items)
	if err != nil {
		return nil, bizerr.NewCode(commerce.CodeConflict)
	}
	validator, ok := runtime.serverSource.(serveroperationresourcecap.ParameterValidator)
	if !ok || validator.ValidateParameters(ctx, serveroperationresourcecap.CapabilityRoute{
		ServiceNo: service.ServiceNo, ChannelNo: route.ChannelNo, PoolNo: route.PoolNo,
	}, serveroperationresourcecap.ActionResize, providerParameters) != nil {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	currentRecurring, err := runtime.currentChannelRecurringAmount(ctx, contextRow)
	if err != nil {
		return nil, err
	}
	targetRecurring := *term.TotalAmountMinor
	direction := "upgrade"
	if targetRecurring < currentRecurring {
		direction = "downgrade"
	} else if targetRecurring == currentRecurring {
		return nil, bizerr.NewCode(commerce.CodeConflict)
	}
	now, err := channelDatabaseNow(ctx, runtime.database)
	if err != nil {
		return nil, err
	}
	amount, effectiveAt := int64(0), contextRow.ExpiresAt.UTC()
	if direction == "upgrade" {
		amount, err = serveroperation.ProrateChannelResize(targetRecurring-currentRecurring,
			contextRow.StartedAt, contextRow.ExpiresAt.UTC(), now)
		if err != nil {
			return nil, bizerr.NewCode(commerce.CodeConflict)
		}
		effectiveAt = now
	}
	amounts, err := commerce.CalculatePricingAmounts(catalog.PurchaseQuoteLine{
		PeriodUnitCode: string(term.PeriodUnit), PriceIncludesTax: publicQuote.Tax.PriceIncludesTax,
		TaxTreatmentCode:     string(publicQuote.Tax.TaxTreatment),
		TaxRateBasisPoints:   publicQuote.Tax.TaxRateBasisPoints,
		TaxCategoryCode:      publicQuote.Tax.TaxCategoryCode,
		RoundingScopeCode:    string(publicQuote.Tax.RoundingScope),
		RoundingModeCode:     string(publicQuote.Tax.RoundingMode),
		RoundingQuantumMinor: publicQuote.Tax.RoundingQuantumMinor,
		Charges: []catalog.PurchaseQuoteCharge{{KindCode: "base", Quantity: 1,
			ChargeAmountMinor: amount, PricedAmountMinor: amount}},
	}, 0)
	if err != nil {
		return nil, err
	}
	quoteNo := channelPricedReference("CQS_", input.ChannelID, input.Operation, input.IdempotencyKey)
	targetConfigurationNo := channelPricedReference("CFG_", quoteNo, "target")
	pricingDigest := channelHash(contextRow.PriceConfigurationNo, targetConfigurationNo,
		contextRow.BillingRuleNo, resolved.BillingRuleNo, direction)
	contentDigest, err := channelDigest(struct {
		QuoteNo, RequestDigest, PricingDigest, Parameters string
		Current, Target, Charge                           int64
	}{quoteNo, requestDigest, pricingDigest, string(providerParameters),
		currentRecurring, targetRecurring, amount})
	if err != nil {
		return nil, err
	}
	quoteRow := &channelPricedQuoteRow{
		QuoteNo: quoteNo, ChannelID: input.ChannelID, PayerCustomerNo: contextRow.PayerCustomerNo,
		OwnerCustomerNo: contextRow.OwnerCustomerNo, ServiceNo: service.ServiceNo,
		ActionCode: "resize", CurrencyCode: contextRow.CurrencyCode, Direction: direction,
		CurrentConfigurationNo: contextRow.PriceConfigurationNo,
		TargetConfigurationNo:  targetConfigurationNo, TargetVersionNo: resolved.VersionNo,
		TargetBillingRuleNo: resolved.BillingRuleNo, ProviderParameters: string(providerParameters),
		ResourceChannelNo: route.ChannelNo, ResourcePoolNo: route.PoolNo,
		ResourceSnapshotNo: channelPricedReference("CRS_", quoteNo, parameters.Revision),
		IdempotencyDigest:  idempotencyDigest, RequestDigest: requestDigest,
		PricingRuleDigest: pricingDigest, ContentDigest: contentDigest,
		CurrentRecurringMinor: currentRecurring, TargetRecurringMinor: targetRecurring,
		TargetSubtotalMinor: *term.NetAmountMinor, TargetNetMinor: *term.NetAmountMinor,
		TargetTaxMinor: *term.TaxAmountMinor,
		SubtotalMinor:  amounts.SubtotalMinor, NetAmountMinor: amounts.NetAmountMinor,
		TaxAmountMinor: amounts.TaxAmountMinor, AmountMinor: amounts.TotalAmountMinor,
		ExpectedServiceVersion: contextRow.ServiceVersion, CustomerVersion: contextRow.CustomerVersion,
		PayerCustomerVersion: contextRow.PayerCustomerVersion,
		EffectiveAt:          effectiveAt, IssuedAt: now, ExpiresAt: now.Add(channelPricedQuoteTTL),
	}
	if err = runtime.persistChannelPricedQuote(ctx, quoteRow, contextRow, resolved, publicQuote.Tax); err != nil {
		if replay, replayErr := runtime.loadChannelPricedQuoteReplay(ctx, input.ChannelID,
			idempotencyDigest, requestDigest); replayErr == nil && replay != nil {
			return projectChannelPricedQuote(replay, true), nil
		}
		return nil, err
	}
	return projectChannelPricedQuote(quoteRow, false), nil
}

func selectChannelResizeTerm(terms []catalog.PublicQuoteTerm, requested, unit string, count int) *catalog.PublicQuoteTerm {
	for index := range terms {
		term := &terms[index]
		if (requested != "" && term.TermNo == requested) ||
			(requested == "" && string(term.PeriodUnit) == unit && term.PeriodCount == count) {
			return term
		}
	}
	return nil
}

func (runtime *channelCommerceRuntime) currentChannelRecurringAmount(
	ctx context.Context,
	service *channelPricedContext,
) (int64, error) {
	selections, err := runtime.catalog.LoadConfigurationPriceSelections(ctx,
		[]string{service.PriceConfigurationNo})
	if err != nil {
		return 0, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	assembler, err := catalog.RenewalQuoteAssemblerFromPublicReader(runtime.catalog)
	if err != nil {
		return 0, err
	}
	lines, err := assembler.AssembleRenewalQuote(ctx, catalog.RenewalQuoteInput{
		CurrencyCode: service.CurrencyCode,
		Lines: []catalog.RenewalQuoteLineInput{{ReferenceNo: service.ServiceNo,
			ProductNo: service.ProductNo, VersionNo: service.VersionNo,
			BillingRuleNo: service.BillingRuleNo, Quantity: 1, PurchaseCycles: 1,
			PriceSelections: selections[service.PriceConfigurationNo]}},
	})
	if err != nil || len(lines) != 1 {
		return 0, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	amounts, err := commerce.CalculatePricingAmounts(lines[0], 0)
	if err != nil || amounts.SetupSubtotalMinor != 0 || amounts.TotalAmountMinor < 0 {
		return 0, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	return amounts.TotalAmountMinor, nil
}

func channelDatabaseNow(ctx context.Context, database gdb.DB) (time.Time, error) {
	var row struct {
		Now time.Time `orm:"now"`
	}
	if err := database.GetScan(ctx, &row, `SELECT CURRENT_TIMESTAMP AS now`); err != nil {
		return time.Time{}, err
	}
	return row.Now.UTC(), nil
}

func channelHash(parts ...string) string {
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(digest[:])
}

func channelPricedReference(prefix string, parts ...string) string {
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return prefix + hex.EncodeToString(digest[:16])
}

func projectChannelPricedQuote(row *channelPricedQuoteRow, reused bool) *channelcommercecap.LifecycleResult {
	if row == nil {
		return nil
	}
	effective, expires := row.EffectiveAt.UTC(), row.ExpiresAt.UTC()
	return &channelcommercecap.LifecycleResult{QuoteNo: row.QuoteNo, Status: "quoted",
		CurrencyCode: row.CurrencyCode, Direction: row.Direction, AmountMinor: row.AmountMinor,
		EffectiveAt: &effective, ExpiresAt: &expires, PaymentRequired: row.AmountMinor > 0,
		Reused: reused}
}

func validateChannelIPProjection(
	projection *serveroperationresourcecap.AccessoryProjection,
	serviceNo string,
) error {
	if projection == nil || projection.ServiceNo != serviceNo || projection.SnapshotNo == "" ||
		projection.ExpiresAt.Before(time.Now().UTC()) ||
		!channelContainsAction(projection.Actions, serveroperationresourcecap.ActionChangeIP) {
		return serveroperationresourcecap.ErrCapabilityUnavailable
	}
	return nil
}

func validateChannelIPReservation(
	reservation *serveroperationresourcecap.IPReservation,
	quantity int,
	expiresAt time.Time,
) error {
	if reservation == nil || serveroperationresourcecap.ValidateIPReservation(*reservation) != nil ||
		reservation.Status != serveroperationresourcecap.IPReservationReserved ||
		reservation.Quantity != quantity || reservation.ExpiresAt.Before(expiresAt) {
		return serveroperationresourcecap.ErrCapabilityUnavailable
	}
	return nil
}

func (runtime *channelCommerceRuntime) previewChannelIPChange(
	ctx context.Context,
	input channelcommercecap.LifecycleInput,
	service *channelServiceRow,
) (*channelcommercecap.LifecycleResult, error) {
	var empty struct{}
	if decodeStrictObject(input.Parameters, &empty) != nil {
		return nil, bizerr.NewCode(commerce.CodeInvalid)
	}
	requestDigest, err := channelDigest(struct{ ChannelID, ServiceNo, Operation string }{
		input.ChannelID, service.ServiceNo, input.Operation})
	if err != nil {
		return nil, err
	}
	idempotencyDigest := channelHash(input.ChannelID, input.Operation, input.IdempotencyKey)
	if replay, replayErr := runtime.loadChannelPricedQuoteReplay(ctx, input.ChannelID,
		idempotencyDigest, requestDigest); replayErr != nil || replay != nil {
		return projectChannelPricedQuote(replay, replay != nil), replayErr
	}
	contextRow, err := runtime.loadChannelPricedContext(ctx, service, input.ChannelID, false)
	if err != nil {
		return nil, err
	}
	route, err := runtime.resolveChannelRoute(ctx, service)
	if err != nil {
		return nil, err
	}
	directory, directoryOK := runtime.serverSource.(serveroperationresourcecap.AccessoryDirectory)
	capacity, capacityOK := runtime.serverSource.(serveroperationresourcecap.IPCapacity)
	if !directoryOK || !capacityOK {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	projection, err := directory.GetAccessories(ctx, serveroperationresourcecap.CapabilityRoute{
		ServiceNo: service.ServiceNo, ChannelNo: route.ChannelNo, PoolNo: route.PoolNo,
	})
	if err != nil || validateChannelIPProjection(projection, service.ServiceNo) != nil {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	addresses, err := channelCurrentIPAddresses(projection.Items)
	if err != nil {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	var policy *struct {
		PolicyNo                                    string
		PolicyVersion                               int64
		Enabled                                     bool
		FreeChangesPerWindow, WindowDays, MaxPerDay int
		AmountMinor                                 int64
		MaximumSnapshotAgeSeconds                   int
	}
	if err = runtime.database.GetScan(ctx, &policy, `SELECT policy.policy_no,
 version.policy_version,version.enabled,version.free_changes_per_window,
 version.window_days,version.max_changes_per_day AS max_per_day,version.amount_minor,
 version.maximum_snapshot_age_seconds
FROM plugin_idc_core_server_ip_change_policy policy
JOIN plugin_idc_core_server_ip_change_policy_version version
 ON version.tenant_id=policy.tenant_id AND version.policy_no=policy.policy_no
 AND version.policy_version=policy.current_version
WHERE policy.tenant_id=0 AND policy.currency_code=?`, contextRow.CurrencyCode); err != nil {
		return nil, err
	}
	if policy == nil || !policy.Enabled || policy.PolicyVersion < 1 {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	var frequency struct{ WindowCount, DayCount int }
	if err = runtime.database.GetScan(ctx, &frequency, `SELECT
 COUNT(*) FILTER (WHERE created_at>=CURRENT_TIMESTAMP-(?*INTERVAL '1 day'))::INT AS window_count,
 COUNT(*) FILTER (WHERE created_at>=date_trunc('day',CURRENT_TIMESTAMP))::INT AS day_count
FROM plugin_idc_core_business_operation
WHERE tenant_id=0 AND subject_type_code='entitlement' AND subject_no=?
 AND operation_type_code='ip_change'`, policy.WindowDays, service.ServiceNo); err != nil {
		return nil, err
	}
	if frequency.DayCount >= policy.MaxPerDay {
		return nil, bizerr.NewCode(commerce.CodeConflict)
	}
	amount := policy.AmountMinor
	if frequency.WindowCount < policy.FreeChangesPerWindow {
		amount = 0
	}
	now, err := channelDatabaseNow(ctx, runtime.database)
	if err != nil {
		return nil, err
	}
	expiresAt := now.Add(channelPricedQuoteTTL)
	reservationRequest := serveroperationresourcecap.IPReservationRequest{
		Route: serveroperationresourcecap.CapabilityRoute{ServiceNo: service.ServiceNo,
			ChannelNo: route.ChannelNo, PoolNo: route.PoolNo},
		Quantity: len(addresses), MaximumCapacityAgeSeconds: policy.MaximumSnapshotAgeSeconds,
		ExpiresAt: expiresAt, IdempotencyKey: "channel-ip:" + idempotencyDigest,
	}
	reservationRequest.RequestDigest, err = serveroperationresourcecap.IPReservationRequestDigest(reservationRequest)
	if err != nil {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	reservation, err := capacity.ReserveIP(ctx, reservationRequest)
	if err != nil || validateChannelIPReservation(reservation, len(addresses), expiresAt) != nil {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	providerParameters, err := json.Marshal(map[string]int{"ip_num": len(addresses)})
	if err != nil {
		return nil, err
	}
	validator, ok := runtime.serverSource.(serveroperationresourcecap.ParameterValidator)
	if !ok || validator.ValidateParameters(ctx, serveroperationresourcecap.CapabilityRoute{
		ServiceNo: service.ServiceNo, ChannelNo: route.ChannelNo, PoolNo: route.PoolNo,
	}, serveroperationresourcecap.ActionChangeIP, providerParameters) != nil {
		_, _ = capacity.ReleaseIP(ctx, serveroperationresourcecap.IPReservationReleaseRequest{
			ReservationNo: reservation.ReservationNo, ReasonCode: "core_quote_invalid"})
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	tax, err := runtime.loadChannelTaxContext(ctx, contextRow.VersionNo, contextRow.CurrencyCode)
	if err != nil {
		_, _ = capacity.ReleaseIP(ctx, serveroperationresourcecap.IPReservationReleaseRequest{
			ReservationNo: reservation.ReservationNo, ReasonCode: "core_quote_failed"})
		return nil, err
	}
	amounts, err := commerce.CalculatePricingAmounts(catalog.PurchaseQuoteLine{
		PeriodUnitCode: string(catalog.PeriodUnitOnce), PriceIncludesTax: true,
		TaxTreatmentCode: tax.Treatment, TaxRateBasisPoints: tax.Rate,
		TaxCategoryCode: tax.Category, RoundingScopeCode: tax.Scope,
		RoundingModeCode: tax.Mode, RoundingQuantumMinor: tax.Quantum,
		Charges: []catalog.PurchaseQuoteCharge{{KindCode: "base", Quantity: 1,
			ChargeAmountMinor: amount, PricedAmountMinor: amount}},
	}, 0)
	if err != nil || amounts.TotalAmountMinor != amount {
		_, _ = capacity.ReleaseIP(ctx, serveroperationresourcecap.IPReservationReleaseRequest{
			ReservationNo: reservation.ReservationNo, ReasonCode: "core_quote_failed"})
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	oldDigest := channelHash("channel-ip-old/v1", service.ServiceNo, strings.Join(addresses, ","))
	quoteNo := channelPricedReference("CQS_", input.ChannelID, input.Operation, input.IdempotencyKey)
	pricingDigest := channelHash(policy.PolicyNo, strconv.FormatInt(policy.PolicyVersion, 10),
		contextRow.CurrencyCode, strconv.FormatInt(amount, 10))
	contentDigest, err := channelDigest(struct {
		QuoteNo, RequestDigest, ReservationNo, SnapshotNo, OldDigest string
		Quantity                                                     int
		Amount, PolicyVersion                                        int64
	}{quoteNo, requestDigest, reservation.ReservationNo,
		reservation.CapacityObservationNo, oldDigest, len(addresses), amount, policy.PolicyVersion})
	if err != nil {
		_, _ = capacity.ReleaseIP(ctx, serveroperationresourcecap.IPReservationReleaseRequest{
			ReservationNo: reservation.ReservationNo, ReasonCode: "core_quote_failed"})
		return nil, err
	}
	quoteRow := &channelPricedQuoteRow{
		QuoteNo: quoteNo, ChannelID: input.ChannelID, PayerCustomerNo: contextRow.PayerCustomerNo,
		OwnerCustomerNo: contextRow.OwnerCustomerNo, ServiceNo: service.ServiceNo,
		ActionCode: "ip_change", CurrencyCode: contextRow.CurrencyCode,
		CurrentConfigurationNo: contextRow.PriceConfigurationNo,
		TargetConfigurationNo:  contextRow.PriceConfigurationNo,
		TargetVersionNo:        contextRow.VersionNo, TargetBillingRuleNo: contextRow.BillingRuleNo,
		ProviderParameters: string(providerParameters), ResourceChannelNo: route.ChannelNo,
		ResourcePoolNo: route.PoolNo, ReservationNo: reservation.ReservationNo,
		ResourceSnapshotNo: reservation.CapacityObservationNo, PolicyNo: policy.PolicyNo,
		OldIPSetDigest: oldDigest, IdempotencyDigest: idempotencyDigest,
		RequestDigest: requestDigest, PricingRuleDigest: pricingDigest, ContentDigest: contentDigest,
		SubtotalMinor: amounts.SubtotalMinor, NetAmountMinor: amounts.NetAmountMinor,
		TaxAmountMinor: amounts.TaxAmountMinor, AmountMinor: amount, IPQuantity: len(addresses),
		MaximumSnapshotAgeSeconds: policy.MaximumSnapshotAgeSeconds,
		PolicyVersion:             policy.PolicyVersion, ExpectedServiceVersion: contextRow.ServiceVersion,
		CustomerVersion: contextRow.CustomerVersion, PayerCustomerVersion: contextRow.PayerCustomerVersion,
		EffectiveAt: now, IssuedAt: now, ExpiresAt: expiresAt,
	}
	if err = runtime.persistChannelPricedQuoteWithTax(ctx, quoteRow, contextRow, nil, tax); err != nil {
		_, _ = capacity.ReleaseIP(ctx, serveroperationresourcecap.IPReservationReleaseRequest{
			ReservationNo: reservation.ReservationNo, ReasonCode: "core_quote_failed"})
		if replay, replayErr := runtime.loadChannelPricedQuoteReplay(ctx, input.ChannelID,
			idempotencyDigest, requestDigest); replayErr == nil && replay != nil {
			return projectChannelPricedQuote(replay, true), nil
		}
		return nil, err
	}
	return projectChannelPricedQuote(quoteRow, false), nil
}

func channelContainsAction(actions []serveroperationresourcecap.Action, target serveroperationresourcecap.Action) bool {
	for _, action := range actions {
		if action == target {
			return true
		}
	}
	return false
}

func channelCurrentIPAddresses(items []serveroperationresourcecap.AccessoryItem) ([]string, error) {
	addresses := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item.Kind != serveroperationresourcecap.AccessoryKindIP || item.State != "active" {
			continue
		}
		var projection struct {
			Address string `json:"address"`
		}
		if json.Unmarshal(item.Projection, &projection) != nil {
			return nil, serveroperationresourcecap.ErrCapabilityUnavailable
		}
		address := net.ParseIP(strings.TrimSpace(projection.Address))
		if address == nil {
			return nil, serveroperationresourcecap.ErrCapabilityUnavailable
		}
		normalized := address.String()
		if _, exists := seen[normalized]; exists {
			return nil, serveroperationresourcecap.ErrCapabilityUnavailable
		}
		seen[normalized] = struct{}{}
		addresses = append(addresses, normalized)
	}
	if len(addresses) < 1 || len(addresses) > 64 {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	sort.Strings(addresses)
	return addresses, nil
}

func (runtime *channelCommerceRuntime) loadChannelTaxContext(
	ctx context.Context,
	versionNo, currency string,
) (*channelTaxContext, error) {
	var rows []channelTaxContext
	if err := runtime.database.GetScan(ctx, &rows, `SELECT tax_policy_no AS policy_no,
 tax_treatment_code AS treatment,tax_rate_basis_points AS rate,tax_category_code AS category,
 rounding_scope_code AS scope,rounding_mode_code AS mode,rounding_quantum_minor AS quantum,
 price_includes_tax
FROM plugin_idc_core_catalog_tax_policy
WHERE tenant_id=0 AND version_no=? AND currency_code=? AND status_code='active'
ORDER BY id LIMIT 2`, versionNo, currency); err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	return &rows[0], nil
}

func (runtime *channelCommerceRuntime) persistChannelPricedQuote(
	ctx context.Context,
	quote *channelPricedQuoteRow,
	service *channelPricedContext,
	resolved *catalog.ServerResizeConfiguration,
	taxSnapshot catalog.PublicQuoteTaxSnapshot,
) error {
	tax, err := runtime.loadChannelTaxContext(ctx, quote.TargetVersionNo, quote.CurrencyCode)
	if err != nil {
		return err
	}
	if tax.PriceIncludesTax != taxSnapshot.PriceIncludesTax ||
		tax.Treatment != string(taxSnapshot.TaxTreatment) ||
		tax.Category != taxSnapshot.TaxCategoryCode || tax.Scope != string(taxSnapshot.RoundingScope) ||
		tax.Mode != string(taxSnapshot.RoundingMode) || tax.Quantum != taxSnapshot.RoundingQuantumMinor ||
		!equalOptionalInt(tax.Rate, taxSnapshot.TaxRateBasisPoints) {
		return serveroperationresourcecap.ErrCapabilityUnavailable
	}
	return runtime.persistChannelPricedQuoteWithTax(ctx, quote, service, resolved, tax)
}

func equalOptionalInt(left, right *int) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func (runtime *channelCommerceRuntime) persistChannelPricedQuoteWithTax(
	ctx context.Context,
	quote *channelPricedQuoteRow,
	service *channelPricedContext,
	resolved *catalog.ServerResizeConfiguration,
	tax *channelTaxContext,
) error {
	if quote == nil || service == nil || tax == nil {
		return serveroperationresourcecap.ErrCapabilityUnavailable
	}
	return runtime.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		locked, err := runtime.loadChannelPricedContext(ctx, &channelServiceRow{
			ServiceNo: service.ServiceNo, ServiceOwnerCustomerNo: service.OwnerCustomerNo,
			PayerCustomerNo: service.PayerCustomerNo, CurrencyCode: service.CurrencyCode,
			Version: service.ServiceVersion,
		}, quote.ChannelID, true)
		if err != nil || locked == nil || locked.ServiceVersion != quote.ExpectedServiceVersion ||
			locked.CustomerVersion != quote.CustomerVersion ||
			locked.PayerCustomerVersion != quote.PayerCustomerVersion ||
			locked.PriceConfigurationNo != quote.CurrentConfigurationNo {
			return bizerr.NewCode(commerce.CodeConflict)
		}
		active, err := runtime.database.GetValue(ctx, `SELECT EXISTS(
SELECT 1 FROM plugin_idc_core_channel_service_operation
WHERE tenant_id=0 AND service_no=?
 AND status_code IN ('scheduled','ready','pending','running','result_unknown'))`, quote.ServiceNo)
		if err != nil || active.Bool() {
			return bizerr.NewCode(commerce.CodeConflict)
		}
		if resolved != nil {
			if err = serveroperation.PersistChannelResizeConfiguration(ctx, runtime.database,
				resolved, quote.TargetConfigurationNo); err != nil {
				return err
			}
		}
		if err = runtime.persistChannelCommerceQuote(ctx, quote, service, tax); err != nil {
			return err
		}
		var reservation, policy, policyVersion, oldDigest any
		if quote.ActionCode == "ip_change" {
			reservation, policy, policyVersion, oldDigest = quote.ReservationNo,
				quote.PolicyNo, quote.PolicyVersion, quote.OldIPSetDigest
		}
		_, err = runtime.database.Exec(ctx, channelPricedQuoteInsertSQL,
			quote.QuoteNo, quote.ChannelID, quote.PayerCustomerNo, quote.OwnerCustomerNo,
			quote.ServiceNo, quote.ActionCode, quote.CurrencyCode, quote.Direction,
			quote.CurrentConfigurationNo, quote.TargetConfigurationNo, quote.TargetVersionNo,
			quote.TargetBillingRuleNo, quote.CurrentRecurringMinor, quote.TargetRecurringMinor,
			quote.TargetSubtotalMinor, quote.TargetNetMinor, quote.TargetTaxMinor,
			quote.SubtotalMinor, quote.NetAmountMinor, quote.TaxAmountMinor, quote.AmountMinor,
			quote.ProviderParameters, quote.ResourceChannelNo, quote.ResourcePoolNo,
			reservation, quote.ResourceSnapshotNo, quote.IPQuantity, quote.MaximumSnapshotAgeSeconds,
			policy, policyVersion, oldDigest, quote.ExpectedServiceVersion, quote.CustomerVersion,
			quote.PayerCustomerVersion, quote.IdempotencyDigest, quote.RequestDigest,
			quote.PricingRuleDigest, quote.ContentDigest, quote.EffectiveAt, quote.IssuedAt, quote.ExpiresAt)
		return gerror.Wrap(err, "insert channel priced quote")
	})
}

func (runtime *channelCommerceRuntime) persistChannelCommerceQuote(
	ctx context.Context,
	quote *channelPricedQuoteRow,
	service *channelPricedContext,
	tax *channelTaxContext,
) error {
	itemNo := channelPricedReference("CQI_", quote.QuoteNo)
	chargeNo := channelPricedReference("CQC_", quote.QuoteNo)
	purpose, chargeType, periodUnit, chargePhase := "change", "recurring", service.PeriodUnitCode, "periodic"
	setupSubtotal, periodSubtotal := int64(0), quote.SubtotalMinor
	if quote.ActionCode == "ip_change" {
		purpose, chargeType, periodUnit, chargePhase = "ip_change", "one_time", "once", "setup"
		setupSubtotal, periodSubtotal = quote.SubtotalMinor, 0
	}
	if _, err := runtime.database.Exec(ctx, `INSERT INTO plugin_idc_core_commerce_quote_snapshot(
tenant_id,quote_no,customer_no,customer_subject_version,purpose_code,source_code,currency_code,
item_count,subtotal_minor,discount_minor,net_amount_minor,tax_amount_minor,total_amount_minor,
input_digest,pricing_rule_digest,eligibility_digest,inventory_digest,content_digest,
issued_at,expires_at,created_by)
VALUES(0,?,?,?,?,'channel',?,1,?,0,?,?,?,?,?,?,?,?,?,?,0)`, quote.QuoteNo,
		quote.PayerCustomerNo, service.PayerCustomerVersion, purpose, quote.CurrencyCode,
		quote.SubtotalMinor, quote.NetAmountMinor, quote.TaxAmountMinor, quote.AmountMinor,
		quote.RequestDigest, quote.PricingRuleDigest, quote.ContentDigest, quote.ContentDigest,
		quote.ContentDigest, quote.IssuedAt, quote.ExpiresAt); err != nil {
		return gerror.Wrap(err, "insert channel commerce quote")
	}
	itemDigest, err := channelDigest(struct {
		QuoteNo, ItemNo, ProductNo, VersionNo, ConfigurationNo string
		Amount                                                 int64
	}{quote.QuoteNo, itemNo, service.ProductNo, quote.TargetVersionNo,
		quote.TargetConfigurationNo, quote.AmountMinor})
	if err != nil {
		return err
	}
	item := do.CommerceQuoteItemSnapshot{
		TenantId: 0, QuoteItemNo: itemNo, QuoteNo: quote.QuoteNo, LineNumber: 1,
		CurrencyCode: quote.CurrencyCode, ProductNo: service.ProductNo,
		VersionNo: quote.TargetVersionNo, BillingRuleNo: quote.TargetBillingRuleNo,
		ConfigurationSnapshotNo: quote.TargetConfigurationNo,
		ProductNameZhCn:         service.ProductNameZhCN, ProductNameEnUs: service.ProductNameEnUS,
		PurchaseCycles: 1, ChargeTypeCode: chargeType, PeriodUnitCode: periodUnit,
		PeriodCount: service.PeriodCount, RequiresContract: false,
		FulfillmentModeCode: "automatic", RiskDecisionCode: "approved",
		InventoryDecisionCode: "manual_confirmation", RouteDecisionDigest: quote.ContentDigest,
		TaxPolicyNo: tax.PolicyNo, PriceIncludesTax: tax.PriceIncludesTax,
		TaxTreatmentCode: tax.Treatment, TaxRateBasisPoints: tax.Rate,
		TaxCategoryCode: tax.Category, RoundingScopeCode: tax.Scope,
		RoundingModeCode: tax.Mode, RoundingQuantumMinor: tax.Quantum,
		SetupSubtotalMinor: setupSubtotal, PeriodSubtotalMinor: periodSubtotal,
		SubtotalMinor: quote.SubtotalMinor, DiscountMinor: 0, NetAmountMinor: quote.NetAmountMinor,
		TaxAmountMinor: quote.TaxAmountMinor, TotalAmountMinor: quote.AmountMinor,
		ContentDigest: itemDigest,
	}
	if _, err = runtime.database.Model(dao.CommerceQuoteItemSnapshot.Table()).Ctx(ctx).Data(item).Insert(); err != nil {
		return err
	}
	quantityDigest, _ := channelDigest(struct {
		ItemNo   string
		Quantity int
	}{itemNo, 1})
	if _, err = runtime.database.Model("plugin_idc_core_commerce_quote_item_quantity_snapshot").Ctx(ctx).Data(map[string]any{
		"tenant_id": 0, "quote_item_no": itemNo, "quantity": 1, "content_digest": quantityDigest,
	}).Insert(); err != nil {
		return err
	}
	inventoryDigest, _ := channelDigest(struct {
		ItemNo, VersionNo string
		Tracking          bool
	}{itemNo, quote.TargetVersionNo, false})
	if _, err = runtime.database.Model("plugin_idc_core_commerce_quote_item_inventory_snapshot").Ctx(ctx).Data(map[string]any{
		"quote_item_no": itemNo, "version_no": quote.TargetVersionNo,
		"tracking_enabled": false, "inventory_version": 0, "sellable_quantity": 0,
		"content_digest": inventoryDigest, "created_at": quote.IssuedAt,
	}).Insert(); err != nil {
		return err
	}
	_, err = runtime.database.Exec(ctx, `INSERT INTO plugin_idc_core_commerce_quote_charge_snapshot(
tenant_id,quote_charge_no,quote_item_no,currency_code,line_number,charge_kind_code,
charge_phase_code,quantity,charge_amount_minor,setup_fee_minor,priced_amount_minor,price_source_digest)
VALUES(0,?,?,?,1,'base',?,1,?,0,?,?)`, chargeNo, itemNo, quote.CurrencyCode,
		chargePhase, quote.AmountMinor, quote.AmountMinor, quote.PricingRuleDigest)
	return err
}

func (runtime *channelCommerceRuntime) loadChannelPricedQuoteReplay(
	ctx context.Context,
	channelID, idempotencyDigest, requestDigest string,
) (*channelPricedQuoteRow, error) {
	var row *channelPricedQuoteRow
	if err := runtime.database.GetScan(ctx, &row, `SELECT quote_no,channel_id,payer_customer_no,
service_owner_customer_no AS owner_customer_no,service_no,action_code,currency_code,
direction_code AS direction,current_configuration_snapshot_no AS current_configuration_no,
target_configuration_snapshot_no AS target_configuration_no,target_version_no,target_billing_rule_no,
provider_parameters::TEXT AS provider_parameters,resource_channel_no,resource_pool_no,
COALESCE(resource_reservation_no,'') AS reservation_no,resource_snapshot_no,
COALESCE(policy_no,'') AS policy_no,COALESCE(old_ip_set_digest,'') AS old_ip_set_digest,
idempotency_digest,request_digest,pricing_rule_digest,content_digest,
current_recurring_amount_minor AS current_recurring_minor,
 target_recurring_amount_minor AS target_recurring_minor,
 target_subtotal_minor,target_net_amount_minor AS target_net_minor,
 target_tax_amount_minor AS target_tax_minor,subtotal_minor,net_amount_minor,
tax_amount_minor,amount_minor,ip_quantity,maximum_snapshot_age_seconds,
COALESCE(policy_version,0) AS policy_version,expected_service_version,
customer_subject_version AS customer_version,payer_customer_subject_version,
effective_at,issued_at,expires_at
FROM plugin_idc_core_channel_service_quote
WHERE tenant_id=0 AND channel_id=? AND idempotency_digest=?`, channelID, idempotencyDigest); err != nil {
		return nil, err
	}
	if row != nil && row.RequestDigest != requestDigest {
		return nil, bizerr.NewCode(commerce.CodeConflict)
	}
	return row, nil
}

type channelPricedOperationProjection struct {
	OperationNo, OrderNo, InvoiceNo, Status, IdempotencyDigest, RequestDigest string
	SettlementNo                                                              string
}

type channelPricedConfirmation struct {
	operationExisted bool
	projection       *channelPricedOperationProjection
	quote            *channelPricedQuoteRow
}

func (runtime *channelCommerceRuntime) persistChannelPricedConfirmation(
	ctx context.Context,
	input channelcommercecap.LifecycleInput,
	service *channelServiceRow,
	quoteNo, idempotencyDigest, requestDigest string,
) (*channelPricedConfirmation, error) {
	confirmation := &channelPricedConfirmation{}
	err := runtime.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		quote, err := runtime.lockChannelPricedQuote(ctx, input.ChannelID,
			service, quoteNo, input.Operation)
		if err != nil {
			return err
		}
		confirmation.quote = quote
		projection, err := runtime.loadChannelPricedOperationByQuote(ctx, quote.QuoteNo)
		if err != nil {
			return err
		}
		if projection != nil {
			confirmation.operationExisted = true
			confirmation.projection = projection
			if projection.IdempotencyDigest != idempotencyDigest ||
				projection.RequestDigest != requestDigest {
				return bizerr.NewCode(commerce.CodeConflict)
			}
			return nil
		}
		now, err := channelDatabaseNow(ctx, runtime.database)
		if err != nil {
			return err
		}
		contextRow, err := runtime.loadChannelPricedContext(ctx, service, input.ChannelID, true)
		if err != nil {
			return err
		}
		if !quote.ExpiresAt.After(now) || quote.ExpectedServiceVersion != contextRow.ServiceVersion ||
			quote.CustomerVersion != contextRow.CustomerVersion ||
			quote.PayerCustomerVersion != contextRow.PayerCustomerVersion ||
			quote.CurrentConfigurationNo != contextRow.PriceConfigurationNo ||
			quote.PayerCustomerNo != contextRow.PayerCustomerNo ||
			quote.OwnerCustomerNo != contextRow.OwnerCustomerNo {
			return bizerr.NewCode(commerce.CodeConflict)
		}
		active, err := runtime.database.GetValue(ctx, `SELECT EXISTS(
SELECT 1 FROM plugin_idc_core_channel_service_operation
WHERE tenant_id=0 AND service_no=?
 AND status_code IN ('scheduled','ready','pending','running','result_unknown'))`, quote.ServiceNo)
		if err != nil || active.Bool() {
			return bizerr.NewCode(commerce.CodeConflict)
		}
		operationNo := channelPricedReference("CPO_", quote.QuoteNo)
		orderNo, invoiceNo, priceNo := "", "", ""
		financial := quote.AmountMinor > 0
		status := "ready"
		if quote.Direction == "downgrade" {
			status = "quoted"
		}
		if financial {
			status = "awaiting_financial_settlement"
			orderNo = channelPricedReference("COR_", quote.QuoteNo)
			invoiceNo = channelPricedReference("CIN_", quote.QuoteNo)
			priceNo = channelPricedReference("CPS_", quote.QuoteNo)
			if err = runtime.persistChannelChargePrice(ctx, quote, contextRow, priceNo); err != nil {
				return err
			}
			if err = runtime.persistChannelPricedOrder(ctx, quote, contextRow,
				orderNo, invoiceNo, idempotencyDigest, requestDigest, now); err != nil {
				return err
			}
		}
		var price, order, invoice any
		if financial {
			price, order, invoice = priceNo, orderNo, invoiceNo
		}
		operationContent, err := channelDigest(struct {
			OperationNo, QuoteNo, ServiceNo, Action, Direction, OrderNo, InvoiceNo string
			Financial                                                              bool
			Expected                                                               int64
		}{operationNo, quote.QuoteNo, quote.ServiceNo, quote.ActionCode, quote.Direction,
			orderNo, invoiceNo, financial, quote.ExpectedServiceVersion})
		if err != nil {
			return err
		}
		if _, err = runtime.database.Exec(ctx, channelPricedBusinessOperationInsertSQL,
			operationNo, quote.ActionCode,
			quote.OwnerCustomerNo, quote.PayerCustomerNo, quote.ServiceNo,
			quote.CurrencyCode, price, quote.QuoteNo, order, invoice, status, financial,
			quote.ResourceChannelNo, quote.ResourcePoolNo, quote.QuoteNo, quote.ContentDigest,
			idempotencyDigest, requestDigest, operationContent, quote.ExpectedServiceVersion,
			now, now); err != nil {
			return gerror.Wrap(err, "insert channel priced business operation")
		}
		if err = runtime.insertChannelPricedEvent(ctx, operationNo, quote.ServiceNo, 1,
			"", status, "channel.priced_operation.confirmed", input.ChannelID,
			quote.ContentDigest, now); err != nil {
			return err
		}
		if !financial {
			if err = runtime.insertReadyChannelPricedExecution(ctx, quote, operationNo, now); err != nil {
				return err
			}
		}
		if runtime.audit == nil {
			return serveroperationresourcecap.ErrSourceUnavailable
		}
		if err = runtime.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "channel.priced_operation.confirm",
			IdempotencyKey:   operationNo, OperationCode: "channel.lifecycle." + input.Operation,
			TargetType: "service_entitlement", TargetNo: quote.ServiceNo,
			CustomerNo: quote.OwnerCustomerNo, ResultCode: status,
			ReasonCode: "approved_channel_quote", RiskCode: "high", CorrelationNo: operationNo,
			BeforeSnapshot: map[string]any{"serviceVersion": quote.ExpectedServiceVersion,
				"quoteNo": quote.QuoteNo},
			AfterSnapshot: map[string]any{"status": status, "payerCustomerNo": quote.PayerCustomerNo,
				"amountMinor": quote.AmountMinor, "effectiveAt": quote.EffectiveAt},
		}); err != nil {
			return err
		}
		confirmation.projection = &channelPricedOperationProjection{OperationNo: operationNo,
			OrderNo: orderNo, InvoiceNo: invoiceNo, Status: status,
			IdempotencyDigest: idempotencyDigest, RequestDigest: requestDigest}
		return nil
	})
	return confirmation, err
}

func (runtime *channelCommerceRuntime) confirmChannelPricedOperation(
	ctx context.Context,
	input channelcommercecap.LifecycleInput,
	service *channelServiceRow,
) (*channelcommercecap.LifecycleResult, error) {
	var parameters channelPricedConfirmParameters
	if decodeStrictObject(input.Parameters, &parameters) != nil {
		return nil, bizerr.NewCode(commerce.CodeInvalid)
	}
	parameters.QuoteNo = strings.TrimSpace(parameters.QuoteNo)
	parameters.SettlementMethod = strings.TrimSpace(parameters.SettlementMethod)
	if parameters.QuoteNo == "" ||
		(parameters.SettlementMethod != "balance" && parameters.SettlementMethod != "credit") {
		return nil, bizerr.NewCode(commerce.CodeInvalid)
	}
	idempotencyDigest := channelHash(input.ChannelID, service.ServiceOwnerCustomerNo,
		parameters.QuoteNo, parameters.SettlementMethod, input.IdempotencyKey)
	requestDigest := channelHash(input.ChannelID, service.ServiceNo, input.Operation,
		parameters.QuoteNo, parameters.SettlementMethod)
	if input.Operation == "change_ip" {
		if err := runtime.revalidateChannelIPReservation(ctx, input.ChannelID,
			service, parameters.QuoteNo); err != nil {
			return nil, err
		}
	}
	confirmation, err := runtime.persistChannelPricedConfirmation(ctx, input, service,
		parameters.QuoteNo, idempotencyDigest, requestDigest)
	if err != nil {
		return nil, err
	}
	projection, quote, operationExisted := confirmation.projection, confirmation.quote,
		confirmation.operationExisted
	if quote.AmountMinor > 0 && projection.Status == "awaiting_financial_settlement" {
		if err = runtime.settleChannelPricedOperation(ctx, quote, projection,
			parameters.SettlementMethod); err != nil {
			return nil, err
		}
		projection, err = runtime.loadChannelPricedOperationByQuote(ctx, quote.QuoteNo)
		if err != nil || projection == nil {
			return nil, bizerr.NewCode(commerce.CodeConflict)
		}
	}
	effective, expires := quote.EffectiveAt.UTC(), quote.ExpiresAt.UTC()
	return &channelcommercecap.LifecycleResult{OperationNo: projection.OperationNo,
		QuoteNo: quote.QuoteNo, Status: projection.Status, CurrencyCode: quote.CurrencyCode,
		Direction: quote.Direction, AmountMinor: quote.AmountMinor, EffectiveAt: &effective,
		ExpiresAt: &expires, PaymentRequired: quote.AmountMinor > 0, Reused: operationExisted}, nil
}

func (runtime *channelCommerceRuntime) lockChannelPricedQuote(
	ctx context.Context,
	channelID string,
	service *channelServiceRow,
	quoteNo, operation string,
) (*channelPricedQuoteRow, error) {
	action := "resize"
	if operation == "change_ip" {
		action = "ip_change"
	}
	var row *channelPricedQuoteRow
	if err := runtime.database.GetScan(ctx, &row, `SELECT quote_no,channel_id,payer_customer_no,
service_owner_customer_no AS owner_customer_no,service_no,action_code,currency_code,
direction_code AS direction,current_configuration_snapshot_no AS current_configuration_no,
target_configuration_snapshot_no AS target_configuration_no,target_version_no,target_billing_rule_no,
provider_parameters::TEXT AS provider_parameters,resource_channel_no,resource_pool_no,
COALESCE(resource_reservation_no,'') AS reservation_no,resource_snapshot_no,
COALESCE(policy_no,'') AS policy_no,COALESCE(old_ip_set_digest,'') AS old_ip_set_digest,
idempotency_digest,request_digest,pricing_rule_digest,content_digest,
 current_recurring_amount_minor AS current_recurring_minor,target_recurring_amount_minor AS target_recurring_minor,
 target_subtotal_minor,target_net_amount_minor AS target_net_minor,target_tax_amount_minor AS target_tax_minor,
 subtotal_minor,net_amount_minor,tax_amount_minor,amount_minor,ip_quantity,maximum_snapshot_age_seconds,
COALESCE(policy_version,0) AS policy_version,expected_service_version,
customer_subject_version AS customer_version,payer_customer_subject_version,
effective_at,issued_at,expires_at
FROM plugin_idc_core_channel_service_quote
WHERE tenant_id=0 AND quote_no=? AND channel_id=? AND service_no=?
 AND payer_customer_no=? AND service_owner_customer_no=? AND action_code=? FOR UPDATE`,
		quoteNo, channelID, service.ServiceNo, service.PayerCustomerNo,
		service.ServiceOwnerCustomerNo, action); err != nil {
		return nil, err
	}
	if row == nil {
		return nil, bizerr.NewCode(commerce.CodeNotAccessible)
	}
	return row, nil
}

func (runtime *channelCommerceRuntime) loadChannelPricedOperationByQuote(
	ctx context.Context,
	quoteNo string,
) (*channelPricedOperationProjection, error) {
	var row *channelPricedOperationProjection
	if err := runtime.database.GetScan(ctx, &row, `SELECT operation_no,
COALESCE(order_no,'') AS order_no,COALESCE(invoice_no,'') AS invoice_no,status_code AS status,
idempotency_digest,request_digest,COALESCE(settlement_no,'') AS settlement_no
FROM plugin_idc_core_business_operation
WHERE tenant_id=0 AND subject_type_code='entitlement' AND quote_no=?`, quoteNo); err != nil {
		return nil, err
	}
	return row, nil
}

func (runtime *channelCommerceRuntime) revalidateChannelIPReservation(
	ctx context.Context,
	channelID string,
	service *channelServiceRow,
	quoteNo string,
) error {
	capacity, ok := runtime.serverSource.(serveroperationresourcecap.IPCapacity)
	if !ok {
		return serveroperationresourcecap.ErrCapabilityUnavailable
	}
	var quote *channelPricedQuoteRow
	if err := runtime.database.GetScan(ctx, &quote, `SELECT quote_no,resource_channel_no,resource_pool_no,
resource_reservation_no AS reservation_no,resource_snapshot_no,ip_quantity,
maximum_snapshot_age_seconds,expires_at,idempotency_digest
FROM plugin_idc_core_channel_service_quote
WHERE tenant_id=0 AND quote_no=? AND channel_id=? AND service_no=? AND action_code='ip_change'`,
		quoteNo, channelID, service.ServiceNo); err != nil {
		return err
	}
	if quote == nil {
		return bizerr.NewCode(commerce.CodeNotAccessible)
	}
	request := serveroperationresourcecap.IPReservationRequest{
		Route: serveroperationresourcecap.CapabilityRoute{ServiceNo: service.ServiceNo,
			ChannelNo: quote.ResourceChannelNo, PoolNo: quote.ResourcePoolNo},
		Quantity: quote.IPQuantity, MaximumCapacityAgeSeconds: quote.MaximumSnapshotAgeSeconds,
		ExpiresAt: quote.ExpiresAt, IdempotencyKey: "channel-ip:" + quote.IdempotencyDigest,
	}
	request.RequestDigest, _ = serveroperationresourcecap.IPReservationRequestDigest(request)
	reservation, err := capacity.ReserveIP(ctx, request)
	if err != nil || reservation == nil || reservation.ReservationNo != quote.ReservationNo ||
		reservation.CapacityObservationNo != quote.ResourceSnapshotNo ||
		reservation.Status != serveroperationresourcecap.IPReservationReserved {
		return serveroperationresourcecap.ErrCapabilityUnavailable
	}
	return nil
}

func (runtime *channelCommerceRuntime) persistChannelChargePrice(
	ctx context.Context,
	quote *channelPricedQuoteRow,
	service *channelPricedContext,
	priceNo string,
) error {
	var item *entity.CommerceQuoteItemSnapshot
	if err := runtime.database.Model(dao.CommerceQuoteItemSnapshot.Table()).Ctx(ctx).
		Where(do.CommerceQuoteItemSnapshot{TenantId: 0, QuoteNo: quote.QuoteNo}).Scan(&item); err != nil {
		return err
	}
	if item == nil {
		return bizerr.NewCode(commerce.CodeConflict)
	}
	phase, periodUnit, periodCount := "resize_charge", service.PeriodUnitCode, service.PeriodCount
	if quote.ActionCode == "ip_change" {
		phase, periodUnit, periodCount = "ip_change", "once", 1
	}
	componentCount := 1
	if quote.TaxAmountMinor > 0 {
		componentCount++
	}
	contentDigest, err := channelDigest(struct {
		PriceNo, QuoteNo, ItemNo, Evidence string
		Amount                             int64
	}{priceNo, quote.QuoteNo, item.QuoteItemNo, quote.ContentDigest, quote.AmountMinor})
	if err != nil {
		return err
	}
	if _, err = runtime.database.Exec(ctx, `INSERT INTO plugin_idc_core_service_price_snapshot(
tenant_id,price_snapshot_no,service_no,subject_type_code,subject_no,customer_no,phase_code,
source_quote_no,source_quote_item_no,product_no,version_no,billing_rule_no,configuration_snapshot_no,
currency_code,purchase_cycles,period_unit_code,period_count,renewal_mode_code,
capability_evidence_no,capability_evidence_digest,capability_evidence_observed_at,
capability_evidence_expires_at,tax_policy_no,price_includes_tax,tax_treatment_code,
tax_rate_basis_points,tax_category_code,rounding_scope_code,rounding_mode_code,
rounding_quantum_minor,component_count,subtotal_minor,discount_minor,net_amount_minor,
tax_amount_minor,total_amount_minor,pricing_rule_digest,content_digest)
VALUES(0,?,NULL,'entitlement',?,?,?, ?,?,?,?,?,?,?,1,?,?,'provider_confirmed',
?,?,?,?,?,?,?,?,?,?,?,?,?,0,?,?,?,?,?,?)`, priceNo, quote.ServiceNo, quote.OwnerCustomerNo,
		phase, quote.QuoteNo, item.QuoteItemNo, service.ProductNo, quote.TargetVersionNo,
		quote.TargetBillingRuleNo, quote.TargetConfigurationNo, quote.CurrencyCode,
		periodUnit, periodCount, quote.QuoteNo, quote.ContentDigest, quote.IssuedAt,
		quote.ExpiresAt, item.TaxPolicyNo, item.PriceIncludesTax, item.TaxTreatmentCode,
		item.TaxRateBasisPoints, item.TaxCategoryCode, item.RoundingScopeCode,
		item.RoundingModeCode, item.RoundingQuantumMinor, componentCount,
		quote.SubtotalMinor, quote.NetAmountMinor, quote.TaxAmountMinor,
		quote.AmountMinor, quote.PricingRuleDigest, contentDigest); err != nil {
		return gerror.Wrap(err, "insert channel charge price snapshot")
	}
	baseNo := channelPricedReference("CPM_", priceNo, "base")
	if _, err = runtime.database.Exec(ctx, `INSERT INTO plugin_idc_core_service_price_component_snapshot(
tenant_id,component_snapshot_no,price_snapshot_no,service_no,subject_type_code,subject_no,
line_number,component_type_code,charge_phase_code,source_reference,quantity,
unit_amount_minor,amount_minor,source_digest,content_digest)
VALUES(0,?,?,NULL,'entitlement',?,1,'base',?, ?,1,?,?,?,?)`, baseNo, priceNo,
		quote.ServiceNo, map[bool]string{true: "setup", false: "periodic"}[quote.ActionCode == "ip_change"],
		quote.QuoteNo, quote.NetAmountMinor, quote.NetAmountMinor,
		quote.PricingRuleDigest, contentDigest); err != nil {
		return err
	}
	if quote.TaxAmountMinor > 0 {
		taxNo := channelPricedReference("CPM_", priceNo, "tax")
		_, err = runtime.database.Exec(ctx, `INSERT INTO plugin_idc_core_service_price_component_snapshot(
tenant_id,component_snapshot_no,price_snapshot_no,service_no,subject_type_code,subject_no,
line_number,component_type_code,charge_phase_code,source_reference,quantity,
unit_amount_minor,amount_minor,source_digest,content_digest)
VALUES(0,?,?,NULL,'entitlement',?,2,'tax','total',?,1,?,?,?,?)`, taxNo, priceNo,
			quote.ServiceNo, item.TaxPolicyNo, quote.TaxAmountMinor, quote.TaxAmountMinor,
			quote.PricingRuleDigest, contentDigest)
	}
	return err
}

func (runtime *channelCommerceRuntime) persistChannelPricedOrder(
	ctx context.Context,
	quote *channelPricedQuoteRow,
	service *channelPricedContext,
	orderNo, invoiceNo, idempotencyDigest, requestDigest string,
	now time.Time,
) error {
	var quoteItem *entity.CommerceQuoteItemSnapshot
	if err := runtime.database.Model(dao.CommerceQuoteItemSnapshot.Table()).Ctx(ctx).
		Where(do.CommerceQuoteItemSnapshot{TenantId: 0, QuoteNo: quote.QuoteNo}).Scan(&quoteItem); err != nil {
		return err
	}
	if quoteItem == nil {
		return bizerr.NewCode(commerce.CodeConflict)
	}
	orderItemNo := channelPricedReference("COI_", quote.QuoteNo)
	invoiceItemNo := channelPricedReference("CII_", quote.QuoteNo)
	itemDigest, _ := channelDigest(struct{ OrderNo, QuoteItemNo, QuoteDigest string }{
		orderNo, quoteItem.QuoteItemNo, quoteItem.ContentDigest})
	order := do.CommerceOrder{TenantId: 0, OrderNo: orderNo,
		CustomerNo: quote.PayerCustomerNo, QuoteNo: quote.QuoteNo,
		CurrencyCode: quote.CurrencyCode, SourceCode: "channel", StatusCode: "pending_payment",
		RiskDecisionCode: "approved", RequiresContract: false, ItemCount: 1,
		SubtotalMinor: quote.SubtotalMinor, DiscountMinor: 0, NetAmountMinor: quote.NetAmountMinor,
		TaxAmountMinor: quote.TaxAmountMinor, TotalAmountMinor: quote.AmountMinor,
		CheckoutIdempotencyDigest: idempotencyDigest, CheckoutRequestDigest: requestDigest,
		PaymentDueAt: &quote.ExpiresAt, Version: 1, CreatedAt: &now, UpdatedAt: &now}
	item := do.CommerceOrderItem{TenantId: 0, OrderItemNo: orderItemNo, OrderNo: orderNo,
		QuoteNo: quote.QuoteNo, QuoteItemNo: quoteItem.QuoteItemNo, LineNumber: 1,
		CurrencyCode: quoteItem.CurrencyCode, ProductNo: quoteItem.ProductNo,
		VersionNo: quoteItem.VersionNo, BillingRuleNo: quoteItem.BillingRuleNo,
		ConfigurationSnapshotNo: quoteItem.ConfigurationSnapshotNo,
		ProductNameZhCn:         quoteItem.ProductNameZhCn, ProductNameEnUs: quoteItem.ProductNameEnUs,
		PurchaseCycles: 1, ChargeTypeCode: quoteItem.ChargeTypeCode,
		PeriodUnitCode: quoteItem.PeriodUnitCode, PeriodCount: quoteItem.PeriodCount,
		RequiresContract: false, FulfillmentModeCode: "automatic", RiskDecisionCode: "approved",
		InventoryDecisionCode: "manual_confirmation", RouteDecisionDigest: quoteItem.RouteDecisionDigest,
		TaxPolicyNo: quoteItem.TaxPolicyNo, PriceIncludesTax: quoteItem.PriceIncludesTax,
		TaxTreatmentCode: quoteItem.TaxTreatmentCode, TaxRateBasisPoints: quoteItem.TaxRateBasisPoints,
		TaxCategoryCode: quoteItem.TaxCategoryCode, RoundingScopeCode: quoteItem.RoundingScopeCode,
		RoundingModeCode: quoteItem.RoundingModeCode, RoundingQuantumMinor: quoteItem.RoundingQuantumMinor,
		SetupSubtotalMinor: quoteItem.SetupSubtotalMinor, PeriodSubtotalMinor: quoteItem.PeriodSubtotalMinor,
		SubtotalMinor: quote.SubtotalMinor, DiscountMinor: 0, NetAmountMinor: quote.NetAmountMinor,
		TaxAmountMinor: quote.TaxAmountMinor, TotalAmountMinor: quote.AmountMinor,
		ContentDigest: itemDigest, CreatedAt: &now}
	orderCopy := orderNo
	invoiceContent, _ := channelDigest(struct{ InvoiceNo, OrderNo, QuoteDigest string }{
		invoiceNo, orderNo, quote.ContentDigest})
	invoice := do.BillingInvoice{TenantId: 0, InvoiceNo: invoiceNo,
		CustomerNo: quote.PayerCustomerNo, OrderNo: &orderCopy, SourceCode: "order",
		CurrencyCode: quote.CurrencyCode, StatusCode: "pending_payment", ItemCount: 1,
		SubtotalMinor: quote.SubtotalMinor, DiscountMinor: 0, NetAmountMinor: quote.NetAmountMinor,
		TaxAmountMinor: quote.TaxAmountMinor, TotalAmountMinor: quote.AmountMinor,
		PaidAmountMinor: 0, RefundableAmountMinor: 0, RefundedAmountMinor: 0,
		SourceSnapshotDigest: quote.ContentDigest, ContentDigest: invoiceContent,
		PaymentDueAt: &quote.ExpiresAt, IssuedAt: &now, Version: 1, CreatedAt: &now, UpdatedAt: &now}
	orderItemCopy := orderItemNo
	invoiceItemContent, _ := channelDigest(struct{ InvoiceNo, OrderItemNo, ItemDigest string }{
		invoiceNo, orderItemNo, itemDigest})
	invoiceItem := do.BillingInvoiceItem{TenantId: 0, InvoiceItemNo: invoiceItemNo,
		InvoiceNo: invoiceNo, CustomerNo: quote.PayerCustomerNo, CurrencyCode: quote.CurrencyCode,
		LineNumber: 1, ItemTypeCode: "order_item", OrderNo: &orderCopy,
		OrderItemNo: &orderItemCopy, SourceReference: orderItemNo,
		DescriptionZhCn: quoteItem.ProductNameZhCn, DescriptionEnUs: quoteItem.ProductNameEnUs,
		SubtotalMinor: quote.SubtotalMinor, DiscountMinor: 0, NetAmountMinor: quote.NetAmountMinor,
		TaxAmountMinor: quote.TaxAmountMinor, TotalAmountMinor: quote.AmountMinor,
		RefundableCapMinor: quote.AmountMinor, ContentDigest: invoiceItemContent, CreatedAt: &now}
	quantityDigest, _ := channelDigest(struct {
		OrderItemNo, QuoteItemNo string
		Quantity                 int
	}{
		orderItemNo, quoteItem.QuoteItemNo, 1})
	for _, insertion := range []struct {
		table string
		data  any
	}{
		{dao.CommerceOrder.Table(), order}, {dao.CommerceOrderItem.Table(), item},
		{"plugin_idc_core_commerce_order_item_quantity_snapshot", map[string]any{
			"tenant_id": 0, "order_item_no": orderItemNo, "order_no": orderNo,
			"quote_item_no": quoteItem.QuoteItemNo, "quantity": 1, "content_digest": quantityDigest}},
		{dao.BillingInvoice.Table(), invoice}, {dao.BillingInvoiceItem.Table(), invoiceItem},
	} {
		if _, err := runtime.database.Model(insertion.table).Ctx(ctx).Data(insertion.data).Insert(); err != nil {
			return gerror.Wrap(err, "insert channel priced order")
		}
	}
	var quoteInventory struct {
		VersionNo        string
		TrackingEnabled  bool
		InventoryVersion int64
		SellableQuantity int64
		ContentDigest    string
	}
	if err := runtime.database.GetScan(ctx, &quoteInventory, `SELECT version_no,tracking_enabled,
inventory_version,sellable_quantity,content_digest
FROM plugin_idc_core_commerce_quote_item_inventory_snapshot WHERE quote_item_no=?`,
		quoteItem.QuoteItemNo); err != nil {
		return err
	}
	orderInventoryDigest, _ := channelDigest(struct {
		OrderItemNo, QuoteItemNo, QuoteDigest string
	}{orderItemNo, quoteItem.QuoteItemNo, quoteInventory.ContentDigest})
	if _, err := runtime.database.Model("plugin_idc_core_commerce_order_item_inventory_snapshot").Ctx(ctx).Data(map[string]any{
		"order_item_no": orderItemNo, "quote_item_no": quoteItem.QuoteItemNo,
		"version_no": quoteInventory.VersionNo, "tracking_enabled": quoteInventory.TrackingEnabled,
		"inventory_version": quoteInventory.InventoryVersion,
		"sellable_quantity": quoteInventory.SellableQuantity,
		"content_digest":    orderInventoryDigest, "created_at": now,
	}).Insert(); err != nil {
		return err
	}
	_, err := runtime.database.Exec(ctx, `INSERT INTO plugin_idc_core_channel_order_attribution(
tenant_id,order_no,channel_id,external_customer_id,ownership_mode,payer_customer_no,
service_owner_customer_no,created_at)
SELECT 0,?,channel_id,external_customer_id,ownership_mode,payer_customer_no,
service_owner_customer_no,? FROM plugin_idc_core_channel_order_attribution
WHERE tenant_id=0 AND order_no=? AND payer_customer_no=? AND service_owner_customer_no=?`,
		orderNo, now, service.SourceOrderNo, quote.PayerCustomerNo, quote.OwnerCustomerNo)
	return gerror.Wrap(err, "insert channel priced order attribution")
}

func (runtime *channelCommerceRuntime) insertReadyChannelPricedExecution(
	ctx context.Context,
	quote *channelPricedQuoteRow,
	operationNo string,
	now time.Time,
) error {
	status := "ready"
	if quote.Direction == "downgrade" {
		status = "scheduled"
	}
	request := serveroperationresourcecap.TaskRequest{OperationNo: operationNo,
		ServiceNo: quote.ServiceNo, ChannelNo: quote.ResourceChannelNo,
		PoolNo: quote.ResourcePoolNo, Action: serveroperationresourcecap.Action(quote.ActionCode),
		ExpectedServiceVersion: quote.ExpectedServiceVersion,
		Parameters:             json.RawMessage(quote.ProviderParameters)}
	requestDigest, err := serveroperationresourcecap.TaskRequestDigest(request)
	if err != nil {
		return err
	}
	_, err = runtime.database.Exec(ctx, channelPricedServiceOperationInsertSQL,
		operationNo, quote.ChannelID,
		quote.PayerCustomerNo, quote.OwnerCustomerNo, quote.ServiceNo, quote.ActionCode,
		quote.ProviderParameters, quote.ResourceChannelNo, quote.ResourcePoolNo,
		quote.ExpectedServiceVersion, status, requestDigest, quote.ContentDigest, now, now, now)
	return err
}

func (runtime *channelCommerceRuntime) settleChannelPricedOperation(
	ctx context.Context,
	quote *channelPricedQuoteRow,
	operation *channelPricedOperationProjection,
	method string,
) error {
	if runtime.invoices == nil || runtime.settlements == nil || operation == nil ||
		operation.OrderNo == "" || operation.InvoiceNo == "" {
		return bizerr.NewCode(commerce.CodeConflict)
	}
	var settlement *billing.FinancialSettlement
	var err error
	if method == "balance" {
		_, err = runtime.invoices.PayFromBalance(ctx, billing.InvoiceBalancePaymentInput{
			InvoiceNo:            operation.InvoiceNo,
			SourceReference:      channelPricedReference("CBP_", operation.OperationNo),
			SourceSnapshotDigest: quote.ContentDigest,
		})
		if err == nil {
			settlement, err = runtime.settlements.SettleOrder(ctx, operation.OrderNo)
		}
	} else {
		settlement, err = runtime.settlements.SettleOrderWithCredit(ctx,
			billing.CreditOrderSettlementInput{OrderNo: operation.OrderNo,
				ActorType: "system", ActorID: 0})
	}
	if err != nil || settlement == nil || settlement.OrderNo != operation.OrderNo ||
		settlement.InvoiceNo != operation.InvoiceNo ||
		settlement.CustomerNo != quote.PayerCustomerNo ||
		settlement.CurrencyCode != quote.CurrencyCode {
		if err != nil {
			return err
		}
		return bizerr.NewCode(commerce.CodeConflict)
	}
	return runtime.finalizeChannelPricedSettlement(ctx, quote, operation, settlement)
}

func (runtime *channelCommerceRuntime) finalizeChannelPricedSettlement(
	ctx context.Context,
	quote *channelPricedQuoteRow,
	operation *channelPricedOperationProjection,
	settlement *billing.FinancialSettlement,
) error {
	return runtime.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var current *channelPricedOperationProjection
		if err := runtime.database.GetScan(ctx, &current, `SELECT operation_no,
COALESCE(order_no,'') AS order_no,COALESCE(invoice_no,'') AS invoice_no,status_code AS status,
idempotency_digest,request_digest,COALESCE(settlement_no,'') AS settlement_no
FROM plugin_idc_core_business_operation WHERE tenant_id=0 AND operation_no=? FOR UPDATE`,
			operation.OperationNo); err != nil {
			return err
		}
		if current == nil || current.OrderNo != settlement.OrderNo ||
			current.InvoiceNo != settlement.InvoiceNo {
			return bizerr.NewCode(commerce.CodeConflict)
		}
		if current.Status == "ready" || current.Status == "executing" ||
			current.Status == "succeeded" {
			return runtime.insertReadyChannelPricedExecution(ctx, quote, current.OperationNo,
				settlement.SettledAt.UTC())
		}
		if current.Status != "awaiting_financial_settlement" {
			return bizerr.NewCode(commerce.CodeConflict)
		}
		result, err := runtime.database.Exec(ctx, `UPDATE plugin_idc_core_business_operation
SET settlement_no=?,status_code='ready',financially_settled_at=?,version=version+1,updated_at=?
WHERE tenant_id=0 AND operation_no=? AND status_code='awaiting_financial_settlement'`,
			settlement.SettlementNo, settlement.SettledAt, settlement.SettledAt, current.OperationNo)
		if err != nil {
			return err
		}
		if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
			return bizerr.NewCode(commerce.CodeConflict)
		}
		if err = runtime.insertChannelPricedEvent(ctx, current.OperationNo, quote.ServiceNo, 2,
			"awaiting_financial_settlement", "ready", "channel.priced_operation.settled",
			quote.ChannelID, settlement.SourceSnapshotDigest, settlement.SettledAt.UTC()); err != nil {
			return err
		}
		return runtime.insertReadyChannelPricedExecution(ctx, quote, current.OperationNo,
			settlement.SettledAt.UTC())
	})
}

func (runtime *channelCommerceRuntime) insertChannelPricedEvent(
	ctx context.Context,
	operationNo, serviceNo string,
	sequence int,
	from, to, reason, channelID, evidence string,
	now time.Time,
) error {
	eventNo := channelPricedReference("CPE_", operationNo, strconv.Itoa(sequence))
	contentDigest, err := channelDigest(struct {
		OperationNo, From, To, Reason, Evidence string
		Sequence                                int
	}{operationNo, from, to, reason, evidence, sequence})
	if err != nil {
		return err
	}
	var fromValue any
	if from != "" {
		fromValue = from
	}
	_, err = runtime.database.Exec(ctx, `INSERT INTO plugin_idc_core_business_operation_event(
tenant_id,event_no,operation_no,service_no,subject_type_code,subject_no,event_sequence,
from_status_code,to_status_code,reason_code,actor_type_code,actor_reference,
evidence_digest,content_digest,created_at)
VALUES(0,?,?,NULL,'entitlement',?,?,?,?, 'system',?,?,?,?)`, eventNo, operationNo,
		serviceNo, sequence, fromValue, to, reason, channelActorReference(channelID),
		evidence, contentDigest, now)
	return err
}
