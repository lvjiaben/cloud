package serveroperation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

const resizeQuoteTTL = 15 * time.Minute

type ResizeOptions struct {
	PublicSlug        string
	CurrencyCode      string
	Revision          string
	Groups            []catalog.PublicConfigurationGroup
	Rules             []catalog.PublicCompatibilityRule
	CurrentSelections []catalog.PublicConfigurationSelection
}

type ResizeQuoteInput struct {
	ServiceNo, Revision, TermNo, CurrencyCode, IdempotencyKey string
	Selections                                                []catalog.PublicConfigurationSelection
}

type ResizeQuote struct {
	QuoteNo, ServiceNo, Direction, CurrencyCode                                string
	CurrentRecurringAmountMinor, TargetRecurringAmountMinor, ChargeAmountMinor int64
	EffectiveAt, IssuedAt, ExpiresAt                                           time.Time
	Reused                                                                     bool
}

type ResizeCheckoutInput struct{ QuoteNo, IdempotencyKey string }
type ResizeCheckout struct {
	OperationNo, OrderNo, InvoiceNo, ServiceNo, Direction, Status, CurrencyCode string
	ChargeAmountMinor                                                           int64
	EffectiveAt                                                                 time.Time
	Reused                                                                      bool
}

type resizeServiceRow struct {
	ServiceNo, CustomerNo, CurrencyCode, PoolNo, ChannelNo, RuntimeStatusCode         string
	EntitlementStatusCode, ProviderTaskStatus, CurrentConfigurationNo, CurrentPriceNo string
	PublicSlug, ProductNo, VersionNo, BillingRuleNo, PeriodUnitCode                   string
	PeriodCount                                                                       int
	CurrentRecurringMinor                                                             int64
	ServiceVersion, CustomerVersion                                                   int64
	PeriodStartedAt, PeriodExpiresAt                                                  time.Time
}

type resizeQuoteRow struct {
	QuoteNo, CustomerNo, ServiceNo, PublicSlug, SourceConfigurationNo, TargetConfigurationNo         string
	TargetVersionNo, TargetBillingRuleNo, Direction, CurrencyCode, ProviderParameters                string
	RequestDigest, ContentDigest                                                                     string
	CurrentRecurringMinor, TargetRecurringMinor, TargetSubtotalMinor, TargetNetMinor, TargetTaxMinor int64
	ChargeAmountMinor, ChargeSubtotalMinor, ChargeNetMinor, ChargeTaxMinor                           int64
	ExpectedServiceVersion, CustomerSubjectVersion                                                   int64
	PeriodStartedAt, PeriodExpiresAt, EffectiveAt, IssuedAt, ExpiresAt                               time.Time
}

func (s *accessoryPricingImpl) ResizeOptionsCurrentCustomer(ctx context.Context, serviceNo string) (*ResizeOptions, error) {
	subject, err := s.customers.CurrentCustomer(ctx)
	if err != nil || subject == nil {
		return nil, ErrServiceUnavailable
	}
	return s.resizeOptions(ctx, subject.CustomerNo, serviceNo)
}

// resizeOptions shares the same owned configuration projection across customer and staff entries.
func (s *accessoryPricingImpl) resizeOptions(ctx context.Context, customerNo, serviceNo string) (*ResizeOptions, error) {
	if s.catalog == nil || !validReference(serviceNo, 40) {
		return nil, ErrServiceUnavailable
	}
	row, err := s.loadResizeService(ctx, customerNo, serviceNo, false)
	if err != nil || row == nil {
		return nil, ErrServiceUnavailable
	}
	graph, err := s.catalog.GetConfiguration(ctx, row.PublicSlug)
	if err != nil || graph == nil {
		return nil, ErrServiceUnavailable
	}
	var selections []catalog.PublicConfigurationSelection
	if err = s.database.GetScan(ctx, &selections, `SELECT option_no,quantity
FROM plugin_idc_core_commerce_configuration_snapshot_item
WHERE tenant_id=0 AND configuration_snapshot_no=? ORDER BY sort_order,id`, row.CurrentConfigurationNo); err != nil {
		return nil, gerror.Wrap(err, "load resize current selections")
	}
	return &ResizeOptions{PublicSlug: row.PublicSlug, CurrencyCode: row.CurrencyCode,
		Revision: graph.Revision, Groups: graph.Groups, Rules: graph.Rules, CurrentSelections: selections}, nil
}

func (s *accessoryPricingImpl) PreviewResizeCurrentCustomer(ctx context.Context, input ResizeQuoteInput) (*ResizeQuote, error) {
	subject, err := s.customers.CurrentCustomer(ctx)
	if err != nil || subject == nil {
		return nil, ErrServiceUnavailable
	}
	return s.previewResize(ctx, subject.CustomerNo, input)
}

// previewResize freezes the existing pricing and compatibility rules for an authorized customer.
func (s *accessoryPricingImpl) previewResize(ctx context.Context, customerNo string, input ResizeQuoteInput) (*ResizeQuote, error) {
	service, customerNo, requestDigest, err := s.prepareResizePreview(ctx, customerNo, input)
	if err != nil {
		return nil, err
	}
	if existing, loadErr := s.loadResizeQuoteReplay(ctx, customerNo, requestDigest); loadErr != nil {
		return nil, loadErr
	} else if existing != nil {
		return projectResizeQuote(existing, true), nil
	}
	publicQuote, term, resolved, err := s.resolveResizeTarget(ctx, service, input)
	if err != nil {
		return nil, err
	}
	row, err := s.buildResizeQuote(ctx, customerNo, requestDigest, service, publicQuote, term, resolved)
	if err != nil {
		return nil, err
	}
	if err = s.persistResizeQuote(ctx, row, resolved); err != nil {
		return nil, gerror.Wrap(err, "persist server resize quote")
	}
	return projectResizeQuote(row, false), nil
}

func (s *accessoryPricingImpl) prepareResizePreview(ctx context.Context, customerNo string, input ResizeQuoteInput) (*resizeServiceRow, string, string, error) {
	if s.catalog == nil || !validReference(strings.TrimSpace(input.ServiceNo), 40) || len(input.IdempotencyKey) < 8 || len(input.IdempotencyKey) > 256 || len(input.Selections) == 0 {
		return nil, "", "", ErrInvalidInput
	}
	service, err := s.loadResizeService(ctx, customerNo, input.ServiceNo, false)
	if err != nil || service == nil || input.CurrencyCode != service.CurrencyCode {
		return nil, "", "", ErrServiceUnavailable
	}
	digest, err := digestValue(struct {
		CustomerNo, ServiceNo, Revision, TermNo, Currency, Key string
		Selections                                             []catalog.PublicConfigurationSelection
	}{customerNo, input.ServiceNo, input.Revision, input.TermNo, input.CurrencyCode, input.IdempotencyKey, input.Selections})
	return service, customerNo, digest, err
}

func (s *accessoryPricingImpl) loadResizeQuoteReplay(ctx context.Context, customerNo, digest string) (*resizeQuoteRow, error) {
	var row *resizeQuoteRow
	err := s.database.GetScan(ctx, &row, `SELECT quote_no,service_no,direction_code AS direction,currency_code,
current_recurring_amount_minor AS current_recurring_minor,target_recurring_amount_minor AS target_recurring_minor,
charge_amount_minor,effective_at,issued_at,expires_at FROM plugin_idc_core_server_resize_quote
WHERE tenant_id=0 AND customer_no=? AND request_digest=?`, customerNo, digest)
	return row, err
}

func (s *accessoryPricingImpl) resolveResizeTarget(ctx context.Context, service *resizeServiceRow, input ResizeQuoteInput) (*catalog.PublicConfigurationQuote, *catalog.PublicQuoteTerm, *catalog.ServerResizeConfiguration, error) {
	quote, err := s.catalog.QuoteConfiguration(ctx, catalog.PublicConfigurationQuoteInput{PublicSlug: service.PublicSlug, Revision: input.Revision, CurrencyCode: input.CurrencyCode, Selections: input.Selections})
	if err != nil || quote == nil {
		return nil, nil, nil, ErrActionUnavailable
	}
	var term *catalog.PublicQuoteTerm
	for i := range quote.Terms {
		candidate := &quote.Terms[i]
		explicit := input.TermNo != "" && candidate.TermNo == input.TermNo
		current := input.TermNo == "" && string(candidate.PeriodUnit) == service.PeriodUnitCode && candidate.PeriodCount == service.PeriodCount
		if explicit || current {
			term = candidate
			break
		}
	}
	if !validResizeTerm(term) {
		return nil, nil, nil, ErrActionUnavailable
	}
	resolver, err := catalog.ServerResizeResolverFromPublicReader(s.catalog)
	if err != nil {
		return nil, nil, nil, ErrServiceUnavailable
	}
	resolved, err := resolver.ResolveServerResizeConfiguration(ctx, catalog.ServerResizeConfigurationInput{PublicSlug: service.PublicSlug, Revision: input.Revision, TermNo: term.TermNo, CurrencyCode: input.CurrencyCode, PurchaseCycles: 1, Selections: input.Selections})
	if err != nil || resolved == nil || resolved.ProductNo != service.ProductNo {
		return nil, nil, nil, ErrActionUnavailable
	}
	return quote, term, resolved, nil
}

func validResizeTerm(term *catalog.PublicQuoteTerm) bool {
	return term != nil && term.TotalAmountMinor != nil && term.NetAmountMinor != nil && term.TaxAmountMinor != nil && (term.Status == catalog.PublicQuoteFinal || term.Status == catalog.PublicQuoteRequiresCustomerContext)
}

func (s *accessoryPricingImpl) buildResizeQuote(ctx context.Context, customerNo, requestDigest string, service *resizeServiceRow, publicQuote *catalog.PublicConfigurationQuote, term *catalog.PublicQuoteTerm, resolved *catalog.ServerResizeConfiguration) (*resizeQuoteRow, error) {
	direction, err := classifyResize(service.CurrentRecurringMinor, *term.TotalAmountMinor)
	if err != nil {
		return nil, err
	}
	current, err := s.loadResizeConfiguration(ctx, service.CurrentConfigurationNo)
	if err != nil {
		return nil, err
	}
	if resizeDiskShrink(current, resolved.Items) {
		return nil, bizerr.NewCode(CodeResizeDiskShrink)
	}
	parameters, err := resizeProviderParameters(current, resolved.Items)
	if err != nil {
		return nil, err
	}
	now, err := databaseNow(ctx, s.database)
	if err != nil {
		return nil, err
	}
	charge, effective := int64(0), service.PeriodExpiresAt
	if direction == resizeDirectionUpgrade {
		charge, err = proratedUpgradeAmount(*term.TotalAmountMinor-service.CurrentRecurringMinor, service.PeriodStartedAt, service.PeriodExpiresAt, now)
		if err != nil {
			return nil, ErrActionUnavailable
		}
		effective = now
	}
	amounts, err := commerce.CalculatePricingAmounts(catalog.PurchaseQuoteLine{PeriodUnitCode: string(term.PeriodUnit), PriceIncludesTax: publicQuote.Tax.PriceIncludesTax, TaxTreatmentCode: string(publicQuote.Tax.TaxTreatment), TaxRateBasisPoints: publicQuote.Tax.TaxRateBasisPoints, TaxCategoryCode: publicQuote.Tax.TaxCategoryCode, RoundingScopeCode: string(publicQuote.Tax.RoundingScope), RoundingModeCode: string(publicQuote.Tax.RoundingMode), RoundingQuantumMinor: publicQuote.Tax.RoundingQuantumMinor, Charges: []catalog.PurchaseQuoteCharge{{KindCode: "base", Quantity: 1, ChargeAmountMinor: charge, PricedAmountMinor: charge}}}, 0)
	if err != nil {
		return nil, ErrServiceUnavailable
	}
	quoteNo, _ := deterministicReference("rsq_", requestDigest)
	configNo, _ := deterministicReference("rsc_", quoteNo)
	content, _ := digestValue(struct {
		QuoteNo, TargetConfig, Parameters string
		Current, Target, Charge           int64
	}{quoteNo, configNo, string(parameters), service.CurrentRecurringMinor, *term.TotalAmountMinor, charge})
	return &resizeQuoteRow{QuoteNo: quoteNo, CustomerNo: customerNo, ServiceNo: service.ServiceNo, PublicSlug: service.PublicSlug, SourceConfigurationNo: service.CurrentConfigurationNo, TargetConfigurationNo: configNo, TargetVersionNo: resolved.VersionNo, TargetBillingRuleNo: resolved.BillingRuleNo, Direction: string(direction), CurrencyCode: service.CurrencyCode, ProviderParameters: string(parameters), RequestDigest: requestDigest, ContentDigest: content, CurrentRecurringMinor: service.CurrentRecurringMinor, TargetRecurringMinor: *term.TotalAmountMinor, TargetSubtotalMinor: *term.NetAmountMinor, TargetNetMinor: *term.NetAmountMinor, TargetTaxMinor: *term.TaxAmountMinor, ChargeAmountMinor: charge, ChargeSubtotalMinor: amounts.SubtotalMinor, ChargeNetMinor: amounts.NetAmountMinor, ChargeTaxMinor: amounts.TaxAmountMinor, ExpectedServiceVersion: service.ServiceVersion, CustomerSubjectVersion: service.CustomerVersion, PeriodStartedAt: service.PeriodStartedAt, PeriodExpiresAt: service.PeriodExpiresAt, EffectiveAt: effective, IssuedAt: now, ExpiresAt: now.Add(resizeQuoteTTL)}, nil
}

func (s *accessoryPricingImpl) persistResizeQuote(ctx context.Context, row *resizeQuoteRow, resolved *catalog.ServerResizeConfiguration) error {
	return s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if err := persistResizeConfiguration(ctx, s.database, resolved, row.TargetConfigurationNo); err != nil {
			return err
		}
		_, err := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_server_resize_quote
(tenant_id,quote_no,customer_no,service_no,public_slug,source_configuration_snapshot_no,target_configuration_snapshot_no,target_version_no,target_billing_rule_no,direction_code,currency_code,current_recurring_amount_minor,target_recurring_amount_minor,target_subtotal_minor,target_net_amount_minor,target_tax_amount_minor,charge_amount_minor,charge_subtotal_minor,charge_net_amount_minor,charge_tax_amount_minor,period_started_at,period_expires_at,effective_at,provider_parameters,request_digest,content_digest,expected_service_version,customer_subject_version,issued_at,expires_at)
VALUES (0,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?::JSONB,?,?,?,?,?,?)`, row.QuoteNo, row.CustomerNo, row.ServiceNo, row.PublicSlug, row.SourceConfigurationNo, row.TargetConfigurationNo, row.TargetVersionNo, row.TargetBillingRuleNo, row.Direction, row.CurrencyCode, row.CurrentRecurringMinor, row.TargetRecurringMinor, row.TargetSubtotalMinor, row.TargetNetMinor, row.TargetTaxMinor, row.ChargeAmountMinor, row.ChargeSubtotalMinor, row.ChargeNetMinor, row.ChargeTaxMinor, row.PeriodStartedAt, row.PeriodExpiresAt, row.EffectiveAt, row.ProviderParameters, row.RequestDigest, row.ContentDigest, row.ExpectedServiceVersion, row.CustomerSubjectVersion, row.IssuedAt, row.ExpiresAt)
		return err
	})
}

func projectResizeQuote(row *resizeQuoteRow, reused bool) *ResizeQuote {
	return &ResizeQuote{QuoteNo: row.QuoteNo, ServiceNo: row.ServiceNo, Direction: row.Direction, CurrencyCode: row.CurrencyCode,
		CurrentRecurringAmountMinor: row.CurrentRecurringMinor, TargetRecurringAmountMinor: row.TargetRecurringMinor,
		ChargeAmountMinor: row.ChargeAmountMinor, EffectiveAt: row.EffectiveAt.UTC(), IssuedAt: row.IssuedAt.UTC(), ExpiresAt: row.ExpiresAt.UTC(), Reused: reused}
}

func (s *accessoryPricingImpl) loadResizeService(ctx context.Context, customerNo, serviceNo string, lock bool) (*resizeServiceRow, error) {
	query := `SELECT service.service_no,service.customer_no,service.currency_code,service.pool_no,COALESCE(service.channel_no,'') AS channel_no,
service.runtime_status_code,service.entitlement_status_code,service.provider_task_status_code,
service.configuration_snapshot_no AS current_configuration_no,service.current_price_snapshot_no AS current_price_no,
product.public_slug,price.product_no,price.version_no,price.billing_rule_no,price.period_unit_code,price.period_count,
price.total_amount_minor AS current_recurring_minor,service.version AS service_version,customer.version AS customer_version,
service.entitlement_started_at AS period_started_at,service.entitlement_expires_at AS period_expires_at
FROM plugin_idc_core_service_instance service
JOIN plugin_idc_core_customer_subject customer ON customer.tenant_id=service.tenant_id AND customer.customer_no=service.customer_no
JOIN plugin_idc_core_service_price_snapshot price ON price.tenant_id=service.tenant_id AND price.price_snapshot_no=service.current_price_snapshot_no
JOIN plugin_idc_core_catalog_product product ON product.tenant_id=price.tenant_id AND product.product_no=price.product_no
WHERE service.tenant_id=0 AND service.customer_no=? AND service.service_no=?`
	if lock {
		query += " FOR UPDATE OF service,customer"
	}
	var row *resizeServiceRow
	if err := s.database.GetScan(ctx, &row, query, customerNo, serviceNo); err != nil {
		return nil, gerror.Wrap(err, "load resize service")
	}
	if row == nil || row.ChannelNo == "" || row.EntitlementStatusCode != "active" || row.ProviderTaskStatus == "result_unknown" ||
		row.PeriodStartedAt.IsZero() || row.PeriodExpiresAt.IsZero() {
		return nil, nil
	}
	return row, nil
}

func (s *accessoryPricingImpl) loadResizeConfiguration(ctx context.Context, snapshotNo string) ([]catalog.PurchaseConfigurationItem, error) {
	var rows []catalog.PurchaseConfigurationItem
	err := s.database.GetScan(ctx, &rows, `SELECT group_no,option_no,group_code,option_code,component_type_code,
group_name_zh_cn,group_name_en_us,option_name_zh_cn,option_name_en_us,normalized_value::TEXT AS normalized_value,
quantity,sort_order FROM plugin_idc_core_commerce_configuration_snapshot_item
WHERE tenant_id=0 AND configuration_snapshot_no=? ORDER BY sort_order,id`, snapshotNo)
	return rows, err
}

func persistResizeConfiguration(ctx context.Context, db gdb.DB, resolved *catalog.ServerResizeConfiguration, snapshotNo string) error {
	selection, content := make([]struct {
		OptionNo string
		Quantity int
	}, 0, len(resolved.Items)), resolved.Items
	var total int64
	for _, item := range resolved.Items {
		selection = append(selection, struct {
			OptionNo string
			Quantity int
		}{item.OptionNo, item.Quantity})
		total += int64(item.Quantity)
	}
	selectionDigest, _ := digestValue(selection)
	contentDigest, _ := digestValue(content)
	if _, err := db.Exec(ctx, `INSERT INTO plugin_idc_core_commerce_configuration_snapshot
(tenant_id,configuration_snapshot_no,product_no,version_no,source_revision_digest,selection_digest,selected_option_count,total_selected_quantity,content_digest)
VALUES (0,?,?,?,?,?,?,?,?)`, snapshotNo, resolved.ProductNo, resolved.VersionNo, resolved.SourceRevisionDigest, selectionDigest, len(resolved.Items), total, contentDigest); err != nil {
		return err
	}
	for i, item := range resolved.Items {
		itemNo, _ := deterministicReference("rsi_", fmt.Sprintf("%s:%d", snapshotNo, i))
		if _, err := db.Exec(ctx, `INSERT INTO plugin_idc_core_commerce_configuration_snapshot_item
(tenant_id,snapshot_item_no,configuration_snapshot_no,version_no,group_no,option_no,group_code,option_code,component_type_code,
group_name_zh_cn,group_name_en_us,option_name_zh_cn,option_name_en_us,normalized_value,quantity,sort_order)
VALUES (0,?,?,?,?,?,?,?,?,?,?,?,?,?::JSONB,?,?)`, itemNo, snapshotNo, resolved.VersionNo, item.GroupNo, item.OptionNo, item.GroupCode, item.OptionCode,
			item.ComponentTypeCode, item.GroupNameZhCN, item.GroupNameEnUS, item.OptionNameZhCN, item.OptionNameEnUS, item.NormalizedValue, item.Quantity, item.SortOrder); err != nil {
			return err
		}
	}
	return nil
}

func resizeDiskShrink(current, target []catalog.PurchaseConfigurationItem) bool {
	a, b := map[string]diskSelection{}, map[string]diskSelection{}
	for _, item := range current {
		if item.ComponentTypeCode == "system_disk" || item.ComponentTypeCode == "data_disk" {
			a[item.GroupCode] = diskSelection{Quantity: item.Quantity, SizeGiB: resizeNumeric(item.NormalizedValue, "gigabytes", "size_gib", "size_gb")}
		}
	}
	for _, item := range target {
		if item.ComponentTypeCode == "system_disk" || item.ComponentTypeCode == "data_disk" {
			b[item.GroupCode] = diskSelection{Quantity: item.Quantity, SizeGiB: resizeNumeric(item.NormalizedValue, "gigabytes", "size_gib", "size_gb")}
		}
	}
	return diskShrinkRequested(a, b)
}

func resizeProviderParameters(current, target []catalog.PurchaseConfigurationItem) (json.RawMessage, error) {
	before := map[string]catalog.PurchaseConfigurationItem{}
	for _, i := range current {
		before[i.GroupCode] = i
	}
	params := map[string]any{}
	for _, item := range target {
		old, exists := before[item.GroupCode]
		changed := !exists || old.OptionNo != item.OptionNo || old.Quantity != item.Quantity
		if !changed {
			continue
		}
		switch item.ComponentTypeCode {
		case "cpu":
			params["cpu"] = resizeNumeric(item.NormalizedValue, "cores", "cpu")
		case "memory":
			params["memory"] = resizeNumeric(item.NormalizedValue, "megabytes", "memory_mb", "memory")
		case "bandwidth":
			params["in_bw"] = resizeNumeric(item.NormalizedValue, "in_mbps", "in_bw", "megabits_per_second")
			params["out_bw"] = resizeNumeric(item.NormalizedValue, "out_mbps", "out_bw", "megabits_per_second")
		case "system_disk":
			params["system_disk_size"] = resizeNumeric(item.NormalizedValue, "gigabytes", "size_gib", "size_gb")
		case "data_disk":
			return nil, ErrActionUnavailable
		default:
			return nil, ErrActionUnavailable
		}
	}
	if len(params) == 0 {
		return nil, ErrActionUnavailable
	}
	for _, v := range params {
		if n, ok := v.(int64); !ok || n <= 0 {
			return nil, ErrActionUnavailable
		}
	}
	encoded, err := json.Marshal(params)
	return encoded, err
}

func resizeNumeric(raw string, keys ...string) int64 {
	var value map[string]any
	if json.Unmarshal([]byte(raw), &value) != nil {
		return 0
	}
	for _, key := range keys {
		if v, ok := value[key]; ok {
			switch n := v.(type) {
			case float64:
				return int64(n)
			case json.Number:
				i, _ := n.Int64()
				return i
			}
		}
	}
	return 0
}

func (s *accessoryPricingImpl) CheckoutResizeCurrentCustomer(ctx context.Context, input ResizeCheckoutInput) (*ResizeCheckout, error) {
	subject, err := s.customers.CurrentCustomer(ctx)
	if err != nil || subject == nil {
		return nil, ErrServiceUnavailable
	}
	return s.checkoutResize(ctx, subject.CustomerNo, input, "customer", subject.CustomerNo)
}

// checkoutResize preserves pricing, mutex, version checks, payment and immutable order evidence.
func (s *accessoryPricingImpl) checkoutResize(ctx context.Context, customerNo string, input ResizeCheckoutInput, actorType, actorReference string) (*ResizeCheckout, error) {
	input.QuoteNo = strings.TrimSpace(input.QuoteNo)
	if !validReference(input.QuoteNo, 40) || len(input.IdempotencyKey) < 8 || len(input.IdempotencyKey) > 256 {
		return nil, ErrInvalidInput
	}
	idem, _ := digestValue(struct{ CustomerNo, Key string }{customerNo, input.IdempotencyKey})
	request, _ := digestValue(struct{ CustomerNo, QuoteNo string }{customerNo, input.QuoteNo})
	var result *ResizeCheckout
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var quote *resizeQuoteRow
		if err := s.database.GetScan(ctx, &quote, `SELECT quote_no,customer_no,service_no,public_slug,
source_configuration_snapshot_no,target_configuration_snapshot_no,target_version_no,target_billing_rule_no,
direction_code AS direction,currency_code,provider_parameters::TEXT AS provider_parameters,request_digest,content_digest,
current_recurring_amount_minor AS current_recurring_minor,target_recurring_amount_minor AS target_recurring_minor,
target_subtotal_minor,target_net_amount_minor AS target_net_minor,target_tax_amount_minor AS target_tax_minor,
charge_amount_minor,charge_subtotal_minor,charge_net_amount_minor AS charge_net_minor,charge_tax_amount_minor AS charge_tax_minor,
expected_service_version,customer_subject_version,period_started_at,period_expires_at,effective_at,issued_at,expires_at
FROM plugin_idc_core_server_resize_quote WHERE tenant_id=0 AND customer_no=? AND quote_no=? FOR UPDATE`, customerNo, input.QuoteNo); err != nil {
			return err
		}
		if quote == nil {
			return ErrActionUnavailable
		}
		var replay *struct{ OperationNo, OrderNo, InvoiceNo, ServiceNo, StatusCode, IdempotencyDigest, RequestDigest string }
		if err := s.database.GetScan(ctx, &replay, `SELECT operation_no,COALESCE(order_no,'') AS order_no,COALESCE(invoice_no,'') AS invoice_no,
service_no,status_code,idempotency_digest,request_digest FROM plugin_idc_core_business_operation
WHERE tenant_id=0 AND operation_type_code='resize' AND quote_no=?`, quote.QuoteNo); err != nil {
			return err
		}
		if replay != nil {
			if replay.IdempotencyDigest != idem || replay.RequestDigest != request {
				return ErrOperationConflict
			}
			result = &ResizeCheckout{OperationNo: replay.OperationNo, OrderNo: replay.OrderNo, InvoiceNo: replay.InvoiceNo, ServiceNo: replay.ServiceNo,
				Direction: quote.Direction, Status: replay.StatusCode, CurrencyCode: quote.CurrencyCode, ChargeAmountMinor: quote.ChargeAmountMinor, EffectiveAt: quote.EffectiveAt, Reused: true}
			return nil
		}
		now, err := databaseNow(ctx, s.database)
		if err != nil {
			return err
		}
		service, err := s.loadResizeService(ctx, customerNo, quote.ServiceNo, true)
		if err != nil {
			return err
		}
		if service == nil || !quote.ExpiresAt.After(now) || service.ServiceVersion != quote.ExpectedServiceVersion ||
			service.CustomerVersion != quote.CustomerSubjectVersion || service.CurrentConfigurationNo != quote.SourceConfigurationNo {
			return ErrOperationConflict
		}
		active, err := s.database.GetValue(ctx, `SELECT EXISTS(SELECT 1 FROM plugin_idc_core_server_operation_execution
WHERE tenant_id=0 AND service_no=? AND status_code IN ('ready','pending','running','result_unknown') UNION ALL
SELECT 1 FROM plugin_idc_core_server_resize_intent WHERE tenant_id=0 AND service_no=?
AND status_code IN ('awaiting_payment','scheduled','ready','executing','result_unknown','manual_review','refund_pending'))`, service.ServiceNo, service.ServiceNo)
		if err != nil {
			return err
		}
		if active.Bool() {
			return ErrOperationConflict
		}
		nextVersion := service.ServiceVersion + 1
		operationNo, _ := deterministicReference("rso_", quote.QuoteNo)
		targetPriceNo, _ := deterministicReference("rsp_", quote.QuoteNo+":target")
		status, intentStatus := "quoted", "scheduled"
		orderNo, invoiceNo, chargePriceNo := "", "", ""
		financial := quote.Direction == string(resizeDirectionUpgrade) && quote.ChargeAmountMinor > 0
		if financial {
			status, intentStatus = "awaiting_financial_settlement", "awaiting_payment"
			orderNo, _ = deterministicReference("ror_", quote.QuoteNo)
			invoiceNo, _ = deterministicReference("rin_", quote.QuoteNo)
			chargePriceNo, _ = deterministicReference("rsp_", quote.QuoteNo+":charge")
			if err = persistResizeCommerceQuote(ctx, s.database, quote, service, chargePriceNo); err != nil {
				return err
			}
			accessoryRow := &accessoryCheckoutRow{QuoteNo: quote.QuoteNo, CustomerNo: quote.CustomerNo, ServiceNo: quote.ServiceNo,
				CurrencyCode: quote.CurrencyCode, QuoteContentDigest: quote.ContentDigest, SubtotalMinor: quote.ChargeSubtotalMinor,
				NetAmountMinor: quote.ChargeNetMinor, TaxAmountMinor: quote.ChargeTaxMinor, TotalAmountMinor: quote.ChargeAmountMinor, ExpiresAt: quote.ExpiresAt}
			if err = s.persistAccessoryOrderAndInvoice(ctx, accessoryRow, orderNo, invoiceNo, idem, request, now); err != nil {
				return err
			}
		}
		var price, order, invoice any
		if financial {
			price, order, invoice = chargePriceNo, orderNo, invoiceNo
		}
		content, _ := digestValue(struct {
			OperationNo, QuoteNo, ServiceNo, Direction, TargetConfig string
			Expected                                                 int64
		}{operationNo, quote.QuoteNo, service.ServiceNo, quote.Direction, quote.TargetConfigurationNo, nextVersion})
		if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_business_operation
(tenant_id,operation_no,operation_type_code,customer_no,service_no,currency_code,price_snapshot_no,quote_no,order_no,invoice_no,
status_code,financial_settlement_required,execution_mode_code,resource_owner_code,pool_no,idempotency_digest,request_digest,
content_digest,expected_service_version,last_error_code,version,created_at,updated_at)
VALUES (0,?,'resize',?,?,?,?,?,?,?, ?,?,'unresolved','idc-resource',?,?,?,?,?,'',1,?,?)`, operationNo, quote.CustomerNo,
			quote.ServiceNo, quote.CurrencyCode, price, quote.QuoteNo, order, invoice, status, financial, service.PoolNo, idem, request, content, nextVersion, now, now); err != nil {
			return err
		}
		if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_server_resize_intent
(tenant_id,operation_no,quote_no,service_no,direction_code,target_configuration_snapshot_no,target_version_no,
target_billing_rule_no,target_price_snapshot_no,target_recurring_amount_minor,effective_at,status_code,created_at,updated_at)
VALUES (0,?,?,?,?,?,?,?,?,?,?,?,?,?)`, operationNo, quote.QuoteNo, quote.ServiceNo, quote.Direction, quote.TargetConfigurationNo,
			quote.TargetVersionNo, quote.TargetBillingRuleNo, targetPriceNo, quote.TargetRecurringMinor, quote.EffectiveAt, intentStatus, now, now); err != nil {
			return err
		}
		if err = execOne(ctx, s.database, `UPDATE plugin_idc_core_service_instance SET last_business_operation_no=?,version=version+1,updated_at=?
WHERE tenant_id=0 AND service_no=? AND customer_no=? AND version=?`, operationNo, now, service.ServiceNo, service.CustomerNo, service.ServiceVersion); err != nil {
			return ErrOperationConflict
		}
		if err = insertEvent(ctx, s.database, operationNo, service.ServiceNo, 1, nil, status, "server_resize.checkout_created", actorType, actorReference, quote.ContentDigest, now); err != nil {
			return err
		}
		if err = s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{IdempotencyScope: "server.resize.checkout", IdempotencyKey: input.IdempotencyKey,
			OperationCode: "server.resize.checkout", TargetType: "server_service", TargetNo: service.ServiceNo, CustomerNo: service.CustomerNo,
			ResultCode: idccap.BusinessAuditResultSucceeded, ReasonCode: actorType + "_resize_checkout", RiskCode: "medium", CorrelationNo: operationNo,
			BeforeSnapshot: map[string]any{"configurationSnapshotNo": service.CurrentConfigurationNo, "serviceVersion": service.ServiceVersion},
			AfterSnapshot:  map[string]any{"configurationSnapshotNo": quote.TargetConfigurationNo, "serviceVersion": nextVersion, "direction": quote.Direction, "effectiveAt": quote.EffectiveAt}}); err != nil {
			return err
		}
		result = &ResizeCheckout{OperationNo: operationNo, OrderNo: orderNo, InvoiceNo: invoiceNo, ServiceNo: service.ServiceNo, Direction: quote.Direction,
			Status: status, CurrencyCode: quote.CurrencyCode, ChargeAmountMinor: quote.ChargeAmountMinor, EffectiveAt: quote.EffectiveAt, Reused: false}
		return nil
	})
	if err != nil {
		return nil, PublicError(err)
	}
	return result, nil
}

type resizeCommerceContext struct {
	NameZhCN, NameEnUS, TaxPolicyNo, TaxTreatment, TaxCategory, RoundingScope, RoundingMode string
	TaxRate                                                                                 *int
	RoundingQuantum                                                                         int64
}

func persistResizeCommerceQuote(ctx context.Context, db gdb.DB, quote *resizeQuoteRow, service *resizeServiceRow, priceNo string) error {
	var value *resizeCommerceContext
	if err := db.GetScan(ctx, &value, `SELECT product.name_zh_cn,product.name_en_us,
tax.tax_policy_no,tax.tax_treatment_code AS tax_treatment,tax.tax_rate_basis_points AS tax_rate,
tax.tax_category_code AS tax_category,tax.rounding_scope_code AS rounding_scope,tax.rounding_mode_code AS rounding_mode,
tax.rounding_quantum_minor AS rounding_quantum FROM plugin_idc_core_catalog_product_version version
JOIN plugin_idc_core_catalog_product product ON product.tenant_id=version.tenant_id AND product.product_no=version.product_no
JOIN plugin_idc_core_catalog_resource_pool_binding binding ON binding.tenant_id=version.tenant_id AND binding.version_no=version.version_no AND binding.pool_no=? AND binding.status_code='active'
JOIN plugin_idc_core_catalog_tax_policy tax ON tax.tenant_id=version.tenant_id AND tax.version_no=version.version_no AND tax.currency_code=? AND tax.status_code='active'
WHERE version.tenant_id=0 AND version.version_no=? ORDER BY binding.priority,binding.id,tax.id LIMIT 1`, service.PoolNo, quote.CurrencyCode, quote.TargetVersionNo); err != nil {
		return err
	}
	if value == nil {
		return ErrServiceUnavailable
	}
	itemNo, _ := deterministicReference("rqi_", quote.QuoteNo)
	chargeNo, _ := deterministicReference("rqc_", quote.QuoteNo)
	pricingDigest, _ := digestValue(struct{ Current, Target, Charge int64 }{quote.CurrentRecurringMinor, quote.TargetRecurringMinor, quote.ChargeAmountMinor})
	if _, err := db.Exec(ctx, `INSERT INTO plugin_idc_core_commerce_quote_snapshot
(tenant_id,quote_no,customer_no,customer_subject_version,purpose_code,source_code,currency_code,item_count,subtotal_minor,
discount_minor,net_amount_minor,tax_amount_minor,total_amount_minor,input_digest,pricing_rule_digest,eligibility_digest,
inventory_digest,content_digest,issued_at,expires_at,created_by)
VALUES (0,?,?,?,'change','customer',?,1,?,0,?,?,?,?,?,?,?,?,?,?,0)`, quote.QuoteNo, quote.CustomerNo, quote.CustomerSubjectVersion,
		quote.CurrencyCode, quote.ChargeSubtotalMinor, quote.ChargeNetMinor, quote.ChargeTaxMinor, quote.ChargeAmountMinor, quote.RequestDigest,
		pricingDigest, quote.ContentDigest, quote.ContentDigest, quote.ContentDigest, quote.IssuedAt, quote.ExpiresAt); err != nil {
		return err
	}
	itemDigest, _ := digestValue(struct {
		ItemNo, QuoteNo, ConfigNo string
		Amount                    int64
	}{itemNo, quote.QuoteNo, quote.TargetConfigurationNo, quote.ChargeAmountMinor})
	item := do.CommerceQuoteItemSnapshot{
		TenantId: 0, QuoteItemNo: itemNo, QuoteNo: quote.QuoteNo, LineNumber: 1,
		CurrencyCode: quote.CurrencyCode, ProductNo: service.ProductNo, VersionNo: quote.TargetVersionNo,
		BillingRuleNo: quote.TargetBillingRuleNo, ConfigurationSnapshotNo: quote.TargetConfigurationNo,
		ProductNameZhCn: value.NameZhCN, ProductNameEnUs: value.NameEnUS, PurchaseCycles: 1,
		ChargeTypeCode: "recurring", PeriodUnitCode: service.PeriodUnitCode, PeriodCount: service.PeriodCount,
		FulfillmentModeCode: "automatic", RiskDecisionCode: "approved",
		InventoryDecisionCode: "manual_confirmation", RouteDecisionDigest: quote.ContentDigest,
		TaxPolicyNo: value.TaxPolicyNo, PriceIncludesTax: true, TaxTreatmentCode: value.TaxTreatment,
		TaxRateBasisPoints: value.TaxRate, TaxCategoryCode: value.TaxCategory,
		RoundingScopeCode: value.RoundingScope, RoundingModeCode: value.RoundingMode,
		RoundingQuantumMinor: value.RoundingQuantum, SetupSubtotalMinor: 0,
		PeriodSubtotalMinor: quote.ChargeSubtotalMinor, SubtotalMinor: quote.ChargeSubtotalMinor,
		DiscountMinor: 0, NetAmountMinor: quote.ChargeNetMinor, TaxAmountMinor: quote.ChargeTaxMinor,
		TotalAmountMinor: quote.ChargeAmountMinor, ContentDigest: itemDigest,
	}
	if _, err := db.Model(dao.CommerceQuoteItemSnapshot.Table()).Ctx(ctx).Data(item).Insert(); err != nil {
		return err
	}
	quantityDigest, err := digestValue(struct {
		QuoteItemNo string
		Quantity    int
	}{itemNo, 1})
	if err != nil {
		return err
	}
	if _, err := db.Model("plugin_idc_core_commerce_quote_item_quantity_snapshot").Ctx(ctx).Data(map[string]any{
		"tenant_id": 0, "quote_item_no": itemNo, "quantity": 1, "content_digest": quantityDigest,
	}).Insert(); err != nil {
		return err
	}
	if _, err := db.Exec(ctx, `INSERT INTO plugin_idc_core_commerce_quote_charge_snapshot
(tenant_id,quote_charge_no,quote_item_no,currency_code,line_number,charge_kind_code,charge_phase_code,quantity,charge_amount_minor,setup_fee_minor,priced_amount_minor,price_source_digest)
VALUES (0,?,?,?,1,'base','periodic',1,?,0,?,?)`, chargeNo, itemNo, quote.CurrencyCode, quote.ChargeAmountMinor, quote.ChargeAmountMinor, pricingDigest); err != nil {
		return err
	}
	priceDigest, _ := digestValue(struct {
		PriceNo, QuoteNo, ItemNo string
		Amount                   int64
	}{priceNo, quote.QuoteNo, itemNo, quote.ChargeAmountMinor})
	_, err = db.Exec(ctx, `INSERT INTO plugin_idc_core_service_price_snapshot
(tenant_id,price_snapshot_no,service_no,customer_no,phase_code,source_quote_no,source_quote_item_no,product_no,version_no,billing_rule_no,
configuration_snapshot_no,currency_code,purchase_cycles,period_unit_code,period_count,renewal_mode_code,capability_evidence_no,
capability_evidence_digest,capability_evidence_observed_at,capability_evidence_expires_at,tax_policy_no,price_includes_tax,
tax_treatment_code,tax_rate_basis_points,tax_category_code,rounding_scope_code,rounding_mode_code,rounding_quantum_minor,
component_count,subtotal_minor,discount_minor,net_amount_minor,tax_amount_minor,total_amount_minor,pricing_rule_digest,content_digest)
VALUES (0,?,?,?,'resize_charge',?,?,?,?,?,?,?,?,?,?,'provider_confirmed',?,?,?,?,?,TRUE,?,?,?,?,?,?,1,?,0,?,?,?,?,?)`, priceNo,
		quote.ServiceNo, quote.CustomerNo, quote.QuoteNo, itemNo, service.ProductNo, quote.TargetVersionNo, quote.TargetBillingRuleNo, quote.TargetConfigurationNo,
		quote.CurrencyCode, 1, service.PeriodUnitCode, service.PeriodCount, quote.QuoteNo, quote.ContentDigest, quote.IssuedAt, quote.ExpiresAt, value.TaxPolicyNo,
		value.TaxTreatment, value.TaxRate, value.TaxCategory, value.RoundingScope, value.RoundingMode, value.RoundingQuantum, quote.ChargeSubtotalMinor,
		quote.ChargeNetMinor, quote.ChargeTaxMinor, quote.ChargeAmountMinor, pricingDigest, priceDigest)
	return err
}
