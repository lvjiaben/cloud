// This file defines the explicit shared dependencies and customer-facing upgrade contracts.

package entitlementupgrade

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

func (s *serviceImpl) customer(ctx context.Context) (string, error) {
	a, err := s.principal.Current(ctx)
	if err != nil || a == nil || a.Code != principal.AudienceCustomer || a.Status != principal.AudienceStatusActive {
		return "", bizerr.NewCode(CodeInvalid)
	}
	return a.CustomerNo, nil
}
func digest(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", gerror.Wrap(err, "encode upgrade evidence")
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}
func view(r *entity.EntitlementUpgrade) *View {
	return &View{UpgradeNo: r.UpgradeNo, EntitlementNo: r.EntitlementNo, CurrencyCode: r.CurrencyCode, StatusCode: r.StatusCode, InvoiceNo: r.InvoiceNo, SourcePeriodMinor: r.SourcePeriodNetMinor, TargetPeriodMinor: r.TargetPeriodNetMinor, NetMinor: r.ChargeNetMinor, TaxMinor: r.ChargeTaxMinor, TotalMinor: r.ChargeTotalMinor, ExpiresAt: r.QuoteExpiresAt.UnixMilli()}
}
func (s *serviceImpl) load(ctx context.Context, no, customer string) (*entity.EntitlementUpgrade, error) {
	var r *entity.EntitlementUpgrade
	err := s.database.Model(do.EntitlementUpgrade{}).Ctx(ctx).Where(do.EntitlementUpgrade{TenantId: 0, UpgradeNo: no, CustomerNo: customer}).LockUpdate().Scan(&r)
	return r, err
}

// List performs two bounded projections rather than querying once per returned service.
func (s *serviceImpl) List(ctx context.Context) (*Page, error) {
	customer, err := s.customer(ctx)
	if err != nil {
		return nil, err
	}
	out := &Page{Targets: []Target{}, Items: []View{}}
	err = s.database.GetScan(ctx, &out.Targets, `SELECT e.entitlement_no,p.public_slug,p.name_zh_cn,p.name_en_us,e.currency_code
 FROM plugin_idc_core_service_entitlement e JOIN plugin_idc_core_catalog_product p ON p.tenant_id=e.tenant_id AND p.product_no=e.product_no
 WHERE e.tenant_id=0 AND e.customer_no=? AND e.status_code='active' AND e.charge_type_code='recurring'
 AND e.entitlement_owner_id='idc-resource' AND e.upstream_service_no='' AND e.expires_at>CURRENT_TIMESTAMP
 AND p.status_code='active' AND p.deleted_at IS NULL ORDER BY e.id DESC LIMIT 100`, customer)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	var rows []*entity.EntitlementUpgrade
	err = s.database.Model(do.EntitlementUpgrade{}).Ctx(ctx).Where(do.EntitlementUpgrade{TenantId: 0, CustomerNo: customer}).OrderDesc("id").Limit(100).Scan(&rows)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	for _, r := range rows {
		out.Items = append(out.Items, *view(r))
	}
	return out, nil
}

// paidState is loaded and locked before quote, confirmation and the first resource dispatch.
type paidState struct {
	EntitlementNo, CustomerNo, ProductNo, VersionNo, ConfigurationSnapshotNo, CurrencyCode, BillingRuleNo, PriceNo, QuoteItemNo string
	StatusCode, ChargeTypeCode, ProductStatus, CustomerStatus                                                                   string
	CustomerPresentation                                                                                                        string
	PeriodUnitCode, ProductConfiguration, CurrentConfiguration                                                                  string
	Version                                                                                                                     int64
	PeriodCount, PurchaseCycles                                                                                                 int
	StartedAt, ExpiresAt                                                                                                        time.Time
}

func (s *serviceImpl) state(ctx context.Context, no, customer string) (*paidState, error) {
	var r *paidState
	err := s.database.GetScan(ctx, &r, `SELECT e.entitlement_no,e.customer_no,e.product_no,e.version_no,e.configuration_snapshot_no,e.currency_code,
 COALESCE(price.billing_rule_no,e.billing_rule_no) AS billing_rule_no,price.price_snapshot_no AS price_no,COALESCE(price.source_quote_item_no,oi.quote_item_no) AS quote_item_no,e.status_code,e.charge_type_code,
 p.status_code AS product_status,c.status_code AS customer_status,COALESCE(price.period_unit_code,e.period_unit_code) AS period_unit_code,COALESCE(price.period_count,e.period_count) AS period_count,COALESCE(price.purchase_cycles,oi.purchase_cycles) AS purchase_cycles,
 COALESCE((SELECT period_started_at FROM plugin_idc_core_entitlement_upgrade u WHERE u.target_price_snapshot_no=price.price_snapshot_no AND u.entitlement_no=e.entitlement_no AND u.customer_no=e.customer_no AND u.status_code='succeeded' LIMIT 1),price.original_expires_at,e.started_at) AS started_at,e.expires_at,e.version,
 vc.customer_presentation_snapshot::text AS customer_presentation,vc.merchant_value_snapshot::text AS product_configuration,sc.typed_value_snapshot::text AS current_configuration
 FROM plugin_idc_core_service_entitlement e
 JOIN plugin_idc_core_customer_subject c ON c.tenant_id=e.tenant_id AND c.customer_no=e.customer_no
 JOIN plugin_idc_core_catalog_product p ON p.tenant_id=e.tenant_id AND p.product_no=e.product_no AND p.current_version_no=e.version_no AND p.deleted_at IS NULL
 JOIN plugin_idc_core_commerce_order_item oi ON oi.tenant_id=e.tenant_id AND oi.order_item_no=e.order_item_no AND oi.order_no=e.order_no
 LEFT JOIN plugin_idc_core_service_price_snapshot price ON price.tenant_id=e.tenant_id AND price.price_snapshot_no=e.current_price_snapshot_no AND price.subject_no=e.entitlement_no AND price.customer_no=e.customer_no
 JOIN plugin_idc_core_catalog_product_version_configuration vc ON vc.tenant_id=e.tenant_id AND vc.version_no=e.version_no
 JOIN plugin_idc_core_commerce_structured_configuration_snapshot sc ON sc.tenant_id=e.tenant_id AND sc.snapshot_no=COALESCE(price.structured_configuration_snapshot_no,e.configuration_snapshot_no)
 WHERE e.tenant_id=0 AND e.entitlement_no=? AND e.customer_no=? AND e.entitlement_owner_id='idc-resource' AND e.upstream_service_no=''
 FOR UPDATE OF e,c,p`, no, customer)
	if err != nil {
		return nil, err
	}
	if r == nil || r.StatusCode != "active" || r.CustomerStatus != "active" || r.ChargeTypeCode != "recurring" || r.ProductStatus != "active" || !r.ExpiresAt.After(time.Now()) {
		return nil, bizerr.NewCode(CodeConflict)
	}
	return r, nil
}
