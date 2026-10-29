// Native commercial admission keeps entitlement ownership in core and accepts
// resource projections only through the startup-injected source capability.
package serveroperation

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/commerce"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

// NativeTargetQuoteSource is the existing commerce owner for one real customer
// quote and its compiler-frozen configuration; no cart or payment writer is exposed.
type NativeTargetQuoteSource interface {
	QuoteAccountTarget(context.Context, commerce.AccountTargetQuoteInput) (*commerce.Quote, error)
	QuoteStaffTarget(context.Context, string, commerce.AccountTargetQuoteInput) (*commerce.Quote, error)
	FreezeStaffTargetQuoteItemConfiguration(context.Context, string, string, commerce.FreezeQuoteItemConfigurationInput) (*commerce.FrozenQuoteItemConfiguration, error)
	FreezeAccountQuoteItemConfiguration(context.Context, commerce.FreezeQuoteItemConfigurationInput) (*commerce.FrozenQuoteItemConfiguration, error)
}

type nativeCommercialService struct {
	database     gdb.DB
	pricing      *accessoryPricingImpl
	prices       *NativePriceBuilder
	evidence     billing.SettlementEvidenceReader
	invoices     billing.InvoiceService
	refunds      billing.CompletedRefundEvidenceReader
	targetQuotes NativeTargetQuoteSource
}

type nativeCommercialTarget struct {
	NativePriceContext
	ProductNameZhCN, ProductNameEnUS, StatusCode string
	PurchaseCycles                               int
}

// NewNativeCommercialService composes the authenticated commercial seam once at
// startup. Resource authorization and confirmation precede this independent core
// owner check; background receipt methods are reachable only by that same source.
func NewNativeCommercialService(database gdb.DB, bizCtx bizctxcap.Service,
	reader principal.Reader, authorization authz.Service, audit idccap.BusinessAuditWriter,
	customers identitycap.CurrentCustomerReader, parameters serveroperationresourcecap.ParameterValidator,
	prices *NativePriceBuilder, evidence billing.SettlementEvidenceReader, invoices billing.InvoiceService, refunds billing.CompletedRefundEvidenceReader, targetQuotes NativeTargetQuoteSource) (servercommercialcap.Service, error) {
	for _, dependency := range []any{database, bizCtx, reader, authorization, audit, customers, parameters, prices, evidence, invoices, refunds, targetQuotes} {
		if nilServerOperationDependency(dependency) {
			return nil, gerror.New("native server commercial dependencies are required")
		}
	}
	return &nativeCommercialService{database: database, prices: prices, evidence: evidence, invoices: invoices, refunds: refunds, targetQuotes: targetQuotes, pricing: &accessoryPricingImpl{
		database: database, bizCtx: bizCtx, principal: reader, authorization: authorization, audit: audit, customers: customers, parameters: parameters,
	}}, nil
}

func (s *nativeCommercialService) authorizeNativeTarget(ctx context.Context, customer string, action serveroperationresourcecap.Action) error {
	if s == nil || s.pricing == nil || ctx == nil || ctx.Err() != nil || !validReference(customer, 40) {
		return bizerr.NewCode(CodeAccessoryPriceNotAccessible)
	}
	current := s.pricing.bizCtx.Current(ctx)
	if current.UserID <= 0 || current.TenantID != 0 || current.ActingUserID != 0 || current.IsImpersonation || current.ActingAsTenant {
		return bizerr.NewCode(CodeAccessoryPriceNotAccessible)
	}
	audience, err := s.pricing.principal.Current(ctx)
	if err != nil || audience == nil || audience.UserID != current.UserID || audience.Status != principal.AudienceStatusActive {
		return bizerr.NewCode(CodeAccessoryPriceNotAccessible)
	}
	if audience.Code == principal.AudienceCustomer {
		subject, readErr := s.pricing.customers.CurrentCustomer(ctx)
		if readErr != nil || subject == nil || subject.CustomerNo != customer || audience.CustomerNo != customer {
			return bizerr.NewCode(CodeAccessoryPriceNotAccessible)
		}
		return nil
	}
	permission := staffAccessoryPermission(action)
	if action == serveroperationresourcecap.ActionResize || action == serveroperationresourcecap.ActionChangeIP {
		permission = PermissionServerEdit
	}
	if permission == "" {
		return bizerr.NewCode(CodeAccessoryPriceNotAccessible)
	}
	_, err = s.pricing.authorize(ctx, permission)
	return err
}

// readNativeCommercialTarget locks the actual parent entitlement. Its version is
// never incremented for a quote or an accessory order; queued execution rechecks
// it and the refund fence. Route and local resource identity come from resource.
func (s *nativeCommercialService) readNativeCommercialTarget(ctx context.Context, target servercommercialcap.TargetSnapshot, lock bool) (*nativeCommercialTarget, error) {
	if ctx == nil || ctx.Err() != nil || !validNativeTargetSnapshot(target, time.Now().UTC()) {
		return nil, bizerr.NewCode(CodeAccessoryQuoteUnavailable)
	}
	query := `SELECT e.entitlement_no,e.customer_no,e.currency_code,e.product_no,e.version_no,e.billing_rule_no,
COALESCE(price.configuration_snapshot_no,item.configuration_snapshot_no,'') AS price_configuration_no,
COALESCE(e.configuration_snapshot_no,'') AS structured_configuration_no,p.public_slug,
p.name_zh_cn AS product_name_zh_cn,p.name_en_us AS product_name_en_us,
e.period_unit_code,e.period_count,COALESCE(price.purchase_cycles,item.purchase_cycles) AS purchase_cycles,e.version AS entitlement_version,c.version AS customer_version,
e.started_at,e.expires_at,e.status_code
FROM plugin_idc_core_service_entitlement e
JOIN plugin_idc_core_customer_subject c ON c.tenant_id=e.tenant_id AND c.customer_no=e.customer_no
JOIN plugin_idc_core_catalog_product p ON p.tenant_id=e.tenant_id AND p.product_no=e.product_no
JOIN plugin_idc_core_commerce_order_item item ON item.tenant_id=e.tenant_id AND item.order_item_no=e.order_item_no AND item.order_no=e.order_no
LEFT JOIN plugin_idc_core_service_price_snapshot price ON price.tenant_id=e.tenant_id AND price.price_snapshot_no=e.current_price_snapshot_no
 AND price.subject_type_code='entitlement' AND price.subject_no=e.entitlement_no
WHERE e.tenant_id=0 AND e.entitlement_no=? AND e.customer_no=? AND e.entitlement_owner_id='idc-resource'
AND e.upstream_service_no='' AND e.status_code='active' AND c.status_code='active'
AND (e.expires_at IS NULL OR e.expires_at>CURRENT_TIMESTAMP)
AND NOT EXISTS(SELECT 1 FROM plugin_idc_core_refund_entitlement_lock l WHERE l.tenant_id=e.tenant_id AND l.entitlement_no=e.entitlement_no AND l.released_at IS NULL)`
	if lock {
		query += " FOR UPDATE OF c,e"
	}
	var row *nativeCommercialTarget
	if err := s.database.GetScan(ctx, &row, query, target.EntitlementNo, target.CustomerNo); err != nil {
		return nil, PublicError(err)
	}
	if row == nil || row.EntitlementVersion != target.EntitlementVersion || row.CustomerVersion < 1 {
		return nil, bizerr.NewCode(CodeAccessoryQuoteUnavailable)
	}
	row.ChannelNo, row.PoolNo = target.ChannelNo, target.PoolNo
	row.LocalResourceNo = target.LocalResourceNo
	return row, nil
}

func validNativeTargetSnapshot(target servercommercialcap.TargetSnapshot, now time.Time) bool {
	if !validReference(target.EntitlementNo, 40) || !validReference(target.CustomerNo, 40) || !validReference(target.LocalResourceNo, 128) ||
		!validReference(target.ChannelNo, 40) || !validReference(target.PoolNo, 40) || target.EntitlementVersion < 1 || target.SnapshotNo == "" || len(target.SnapshotNo) > 128 ||
		target.ObservedAt.IsZero() || target.ObservedAt.After(now.Add(time.Minute)) || !target.ExpiresAt.After(now) || !target.ExpiresAt.After(target.ObservedAt) ||
		len(target.Items) > 500 || len(target.Actions) > 100 {
		return false
	}
	data, err := json.Marshal(target)
	return err == nil && len(data) <= 65536
}

func nativeSnapshotAllows(target servercommercialcap.TargetSnapshot, action serveroperationresourcecap.Action) bool {
	for _, allowed := range target.Actions {
		if allowed == action {
			if action == serveroperationresourcecap.ActionResize {
				return target.RuntimeStatus == "running" || target.RuntimeStatus == "stopped"
			}
			return allowedAction(&serviceRow{EntitlementStatusCode: "active", RuntimeStatusCode: target.RuntimeStatus, TrafficSuspended: target.TrafficSuspended, ProviderTaskStatus: "none"}, action)
		}
	}
	return false
}

// nativeCommercialBusy is shared by all addon/IP/resize admissions under the
// parent entitlement lock, so individual quote tables cannot bypass one another.
func (s *nativeCommercialService) nativeCommercialBusy(ctx context.Context, entitlement, except string) (bool, error) {
	value, err := s.database.GetValue(ctx, `SELECT EXISTS(SELECT 1 FROM plugin_idc_core_business_operation
WHERE tenant_id=0 AND subject_type_code='entitlement' AND subject_no=? AND operation_no<>?
AND status_code IN ('awaiting_financial_settlement','financially_settled','ready','executing','manual_review')
UNION ALL SELECT 1 FROM plugin_idc_core_native_server_write_fence WHERE tenant_id=0 AND entitlement_no=?
AND status_code IN ('executing','result_unknown'))`, entitlement, except, entitlement)
	if err != nil {
		return false, err
	}
	return value.Bool(), nil
}
