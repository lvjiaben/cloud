// This file publishes accessory price contracts and freezes verified quotes for
// the original server. Fixed traffic packages never accept client-authored price
// or capacity; storage replay and capability freshness remain separate guards.

package serveroperation

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/principal"
	"lina-plugin-idc-core/backend/internal/service/reauth"
)

const (
	PermissionAccessoryPriceRead  = "idc:admin:resource:accessory-price:read"
	PermissionAccessoryPriceWrite = "idc:admin:resource:accessory-price:write"
	accessoryQuoteTTL             = 15 * time.Minute
)

var (
	CodeAccessoryPriceNotAccessible = bizerr.MustDefine("IDC_ACCESSORY_PRICE_NOT_ACCESSIBLE",
		"The server accessory price is not accessible", gcode.CodeNotAuthorized)
	CodeAccessoryPriceInvalid = bizerr.MustDefine("IDC_ACCESSORY_PRICE_INVALID",
		"The server accessory price request is invalid", gcode.CodeInvalidParameter)
	CodeAccessoryPriceConflict = bizerr.MustDefine("IDC_ACCESSORY_PRICE_CONFLICT",
		"The server accessory price changed; reload it and try again", gcode.CodeInvalidOperation)
	CodeAccessoryPriceUnavailable = bizerr.MustDefine("IDC_ACCESSORY_PRICE_UNAVAILABLE",
		"The server accessory price is temporarily unavailable", gcode.CodeInternalError)
	CodeAccessoryQuoteUnavailable = bizerr.MustDefine("IDC_ACCESSORY_QUOTE_UNAVAILABLE",
		"The server accessory quote is unavailable", gcode.CodeInvalidOperation)
)

type AccessoryPricePolicyInput struct {
	Kind               serveroperationresourcecap.AccessoryKind
	CurrencyCode       string
	FreeCountPerServer int
	UnitAmountMinor    int64
	PeriodUnitCode     string
	PeriodCount        int
	ExpectedVersion    int64
}

type AccessoryPricePolicy struct {
	PolicyNo           string
	Kind               serveroperationresourcecap.AccessoryKind
	CurrencyCode       string
	FreeCountPerServer int
	UnitAmountMinor    int64
	PeriodUnitCode     string
	PeriodCount        int
	Version            int64
	PublishedAt        time.Time
}

type AccessoryQuoteInput struct {
	PackageNo      string
	CustomerNo     string
	ServiceNo      string
	Action         serveroperationresourcecap.Action
	Parameters     json.RawMessage
	IdempotencyKey string
}

type AccessoryQuote struct {
	TrafficGB              int
	PackageNameZhCN        string
	PackageNameEnUS        string
	QuoteNo                string
	CustomerNo             string
	ServiceNo              string
	Action                 serveroperationresourcecap.Action
	Kind                   serveroperationresourcecap.AccessoryKind
	CurrencyCode           string
	RequestedQuantity      int
	ExistingObjectCount    int
	ReservedCreateCount    int
	FreeCountPerServer     int
	ChargedQuantity        int
	UnitAmountMinor        int64
	SubtotalMinor          int64
	NetAmountMinor         int64
	TaxAmountMinor         int64
	TotalAmountMinor       int64
	PeriodUnitCode         string
	PeriodCount            int
	PolicyNo               string
	PolicyVersion          int64
	CustomerSubjectVersion int64
	ExpectedServiceVersion int64
	BaselineObjectNos      []string
	BaselineSnapshotNo     string
	BaselineObservedAt     time.Time
	IssuedAt               time.Time
	ExpiresAt              time.Time
	Reused                 bool
}

type AccessoryPricing interface {
	// ListTrafficPackages returns one authorized page with exact total and stock occupancy.
	ListTrafficPackages(context.Context, TrafficPackageSearch) (*TrafficPackagePage, error)
	// SaveTrafficPackage appends immutable price/scope and changes sale settings under a row lock.
	SaveTrafficPackage(context.Context, TrafficPackageInput) (*TrafficPackage, error)
	// TrafficPackageProducts returns bounded catalog-owner names after settings authorization.
	TrafficPackageProducts(context.Context, string, []string) ([]catalog.TrafficPackageProduct, error)
	// CustomerTrafficPackages resolves the current customer and original server before listing eligible packages.
	CustomerTrafficPackages(context.Context, string, TrafficPackageSearch) (*TrafficPackagePage, error)
	// ForServerTrafficPackages requires an authenticated owner caller and checks the
	// exact customer/server association and fresh provider capability; no match rejects.
	ForServerTrafficPackages(context.Context, string, string, TrafficPackageSearch) (*TrafficPackagePage, error)
	ListPolicies(context.Context) ([]AccessoryPricePolicy, error)
	PublishPolicy(context.Context, AccessoryPricePolicyInput) (*AccessoryPricePolicy, error)
	Preview(context.Context, AccessoryQuoteInput) (*AccessoryQuote, error)
	PreviewCurrentCustomer(context.Context, AccessoryQuoteInput) (*AccessoryQuote, error)
	CheckoutCurrentCustomer(context.Context, AccessoryCheckoutInput) (*AccessoryCheckout, error)
	CheckoutForStaff(context.Context, string, AccessoryCheckoutInput, int, string) (*AccessoryCheckout, error)
	ListIPChangePolicies(context.Context) ([]IPChangePolicy, error)
	PublishIPChangePolicy(context.Context, IPChangePolicyInput) (*IPChangePolicy, error)
	PreviewIPChangeCurrentCustomer(context.Context, IPChangeQuoteInput) (*IPChangeQuote, error)
	CheckoutIPChangeCurrentCustomer(context.Context, IPChangeCheckoutInput) (*IPChangeCheckout, error)
	// ResizeOptionsForStaff checks exact edit permission and merchant scope, then reads current choices.
	ResizeOptionsForStaff(context.Context, string) (*ResizeOptions, error)
	// PreviewResizeForStaff freezes a quote after staff permission and server ownership checks.
	PreviewResizeForStaff(context.Context, ResizeQuoteInput) (*ResizeQuote, error)
	// CheckoutResizeForStaff confirms the quoted server after fresh staff checks; it never edits paid orders.
	CheckoutResizeForStaff(context.Context, string, ResizeCheckoutInput) (*ResizeCheckout, error)
	ResizeOptionsCurrentCustomer(context.Context, string) (*ResizeOptions, error)
	PreviewResizeCurrentCustomer(context.Context, ResizeQuoteInput) (*ResizeQuote, error)
	CheckoutResizeCurrentCustomer(context.Context, ResizeCheckoutInput) (*ResizeCheckout, error)
}

type accessoryPricingImpl struct {
	database      gdb.DB
	bizCtx        bizctxcap.Service
	principal     principal.Reader
	authorization authz.Service
	audit         idccap.BusinessAuditWriter
	directory     serveroperationresourcecap.AccessoryDirectory
	ipCapacity    serveroperationresourcecap.IPCapacity
	parameters    serveroperationresourcecap.ParameterValidator
	customers     identitycap.CurrentCustomerReader
	recentAuth    reauth.SessionConfirmationService
	catalog       catalog.CatalogReader
}

func NewAccessoryPricing(database gdb.DB, bizCtx bizctxcap.Service, principalReader principal.Reader,
	authorization authz.Service, audit idccap.BusinessAuditWriter,
	directory serveroperationresourcecap.AccessoryDirectory,
	ipCapacity serveroperationresourcecap.IPCapacity,
	parameters serveroperationresourcecap.ParameterValidator,
	customers identitycap.CurrentCustomerReader,
	recentAuth reauth.SessionConfirmationService, catalogReader catalog.CatalogReader) (AccessoryPricing, error) {
	dependencies := []any{database, bizCtx, principalReader, authorization, audit, directory, ipCapacity, parameters, customers, recentAuth, catalogReader}
	for _, dependency := range dependencies {
		if nilServerOperationDependency(dependency) {
			return nil, gerror.New("server accessory pricing requires all runtime dependencies")
		}
	}
	return &accessoryPricingImpl{database: database, bizCtx: bizCtx, principal: principalReader,
		authorization: authorization, audit: audit, directory: directory, ipCapacity: ipCapacity,
		parameters: parameters, customers: customers, recentAuth: recentAuth, catalog: catalogReader}, nil
}

func (s *accessoryPricingImpl) PreviewCurrentCustomer(ctx context.Context,
	input AccessoryQuoteInput) (*AccessoryQuote, error) {
	subject, err := s.customers.CurrentCustomer(ctx)
	if err != nil || subject == nil || !validReference(subject.CustomerNo, 40) ||
		(input.CustomerNo != "" && input.CustomerNo != subject.CustomerNo) {
		return nil, bizerr.NewCode(CodeAccessoryQuoteUnavailable)
	}
	input.CustomerNo = subject.CustomerNo
	return s.Preview(ctx, input)
}

func (s *accessoryPricingImpl) ListPolicies(ctx context.Context) ([]AccessoryPricePolicy, error) {
	if _, err := s.authorize(ctx, PermissionAccessoryPriceRead); err != nil {
		return nil, err
	}
	var rows []struct {
		PolicyNo           string    `orm:"policy_no"`
		Kind               string    `orm:"accessory_kind"`
		CurrencyCode       string    `orm:"currency_code"`
		FreeCountPerServer int       `orm:"free_count_per_server"`
		UnitAmountMinor    int64     `orm:"unit_amount_minor"`
		PeriodUnitCode     string    `orm:"period_unit_code"`
		PeriodCount        int       `orm:"period_count"`
		Version            int64     `orm:"policy_version"`
		PublishedAt        time.Time `orm:"published_at"`
	}
	if err := s.database.GetScan(ctx, &rows, `SELECT policy.policy_no,policy.accessory_kind,
policy.currency_code,version.free_count_per_server,version.unit_amount_minor,
version.period_unit_code,version.period_count,version.policy_version,version.published_at
FROM plugin_idc_core_server_accessory_price_policy policy
JOIN plugin_idc_core_server_accessory_price_policy_version version
 ON version.policy_no=policy.policy_no AND version.policy_version=policy.current_version
WHERE policy.tenant_id=0 AND policy.accessory_kind<>'traffic' ORDER BY policy.accessory_kind,policy.currency_code LIMIT 300`); err != nil {
		return nil, bizerr.WrapCode(err, CodeAccessoryPriceUnavailable)
	}
	result := make([]AccessoryPricePolicy, len(rows))
	for index := range rows {
		result[index] = AccessoryPricePolicy{PolicyNo: rows[index].PolicyNo,
			Kind: serveroperationresourcecap.AccessoryKind(rows[index].Kind), CurrencyCode: rows[index].CurrencyCode,
			FreeCountPerServer: rows[index].FreeCountPerServer, UnitAmountMinor: rows[index].UnitAmountMinor,
			PeriodUnitCode: rows[index].PeriodUnitCode, PeriodCount: rows[index].PeriodCount,
			Version: rows[index].Version, PublishedAt: rows[index].PublishedAt.UTC()}
	}
	return result, nil
}

func (s *accessoryPricingImpl) PublishPolicy(ctx context.Context,
	input AccessoryPricePolicyInput) (*AccessoryPricePolicy, error) {
	input.CurrencyCode = strings.ToUpper(input.CurrencyCode)
	if input.Kind == serveroperationresourcecap.AccessoryKindTraffic || !validAccessoryPriceInput(input) {
		return nil, bizerr.NewCode(CodeAccessoryPriceInvalid)
	}
	actor, err := s.authorize(ctx, PermissionAccessoryPriceWrite)
	if err != nil {
		return nil, err
	}
	policyNo, err := deterministicReference("apc_", struct{ Kind, Currency string }{string(input.Kind), input.CurrencyCode})
	if err != nil {
		return nil, bizerr.NewCode(CodeAccessoryPriceInvalid)
	}
	var result *AccessoryPricePolicy
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var current *struct {
			PolicyNo string `orm:"policy_no"`
			Version  int64  `orm:"current_version"`
		}
		if loadErr := s.database.GetScan(ctx, &current, `SELECT policy_no,current_version
FROM plugin_idc_core_server_accessory_price_policy
WHERE tenant_id=0 AND accessory_kind=? AND currency_code=? FOR UPDATE`, input.Kind, input.CurrencyCode); loadErr != nil {
			return loadErr
		}
		currentVersion := int64(0)
		if current != nil {
			policyNo = current.PolicyNo
			currentVersion = current.Version
		}
		if currentVersion != input.ExpectedVersion {
			return bizerr.NewCode(CodeAccessoryPriceConflict)
		}
		nextVersion := currentVersion + 1
		digest, digestErr := digestValue(struct {
			Contract, PolicyNo, Kind, Currency, PeriodUnit string
			Version, UnitAmount                            int64
			FreeCount, PeriodCount                         int
		}{"server-accessory-price/v1", policyNo, string(input.Kind), input.CurrencyCode,
			input.PeriodUnitCode, nextVersion, input.UnitAmountMinor, input.FreeCountPerServer, input.PeriodCount})
		if digestErr != nil {
			return digestErr
		}
		now, nowErr := databaseNow(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		if current == nil {
			if _, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_server_accessory_price_policy
(tenant_id,policy_no,accessory_kind,currency_code,current_version,created_at,updated_at)
VALUES (0,?,?,?,?,?,?)`, policyNo, input.Kind, input.CurrencyCode, nextVersion, now, now); insertErr != nil {
				return insertErr
			}
		}
		if _, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_server_accessory_price_policy_version
(tenant_id,policy_no,policy_version,accessory_kind,currency_code,free_count_per_server,
unit_amount_minor,period_unit_code,period_count,content_digest,published_by,published_at)
VALUES (0,?,?,?,?,?,?,?,?,?,?,?)`, policyNo, nextVersion, input.Kind, input.CurrencyCode,
			input.FreeCountPerServer, input.UnitAmountMinor, input.PeriodUnitCode, input.PeriodCount,
			digest, actor, now); insertErr != nil {
			return insertErr
		}
		if current != nil {
			if updateErr := execOne(ctx, s.database, `UPDATE plugin_idc_core_server_accessory_price_policy
SET current_version=?,updated_at=? WHERE tenant_id=0 AND policy_no=? AND current_version=?`,
				nextVersion, now, policyNo, currentVersion); updateErr != nil {
				return bizerr.NewCode(CodeAccessoryPriceConflict)
			}
		}
		if auditErr := s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "server.accessory.price.publish", IdempotencyKey: policyNo + ":" + strconv.FormatInt(nextVersion, 10),
			OperationCode: "server.accessory.price.publish", TargetType: "server_accessory_price", TargetNo: policyNo,
			ResultCode: idccap.BusinessAuditResultSucceeded, ReasonCode: "staff_price_publication", RiskCode: "high",
			RequiredPermission: PermissionAccessoryPriceWrite,
			BeforeSnapshot:     map[string]any{"version": currentVersion}, AfterSnapshot: map[string]any{
				"version": nextVersion, "kind": input.Kind, "currencyCode": input.CurrencyCode,
				"freeCountPerServer": input.FreeCountPerServer, "unitAmountMinor": input.UnitAmountMinor,
				"periodUnitCode": input.PeriodUnitCode, "periodCount": input.PeriodCount,
			},
		}); auditErr != nil {
			return auditErr
		}
		result = &AccessoryPricePolicy{PolicyNo: policyNo, Kind: input.Kind, CurrencyCode: input.CurrencyCode,
			FreeCountPerServer: input.FreeCountPerServer, UnitAmountMinor: input.UnitAmountMinor,
			PeriodUnitCode: input.PeriodUnitCode, PeriodCount: input.PeriodCount,
			Version: nextVersion, PublishedAt: now.UTC()}
		return nil
	})
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeAccessoryPriceUnavailable)
	}
	return result, nil
}

func validAccessoryPriceInput(input AccessoryPricePolicyInput) bool {
	if len(input.CurrencyCode) != 3 || input.CurrencyCode != strings.ToUpper(input.CurrencyCode) ||
		input.FreeCountPerServer < 0 || input.FreeCountPerServer > 10000 ||
		input.UnitAmountMinor < 0 || input.UnitAmountMinor > 1000000000 || input.ExpectedVersion < 0 {
		return false
	}
	for _, character := range input.CurrencyCode {
		if character < 'A' || character > 'Z' {
			return false
		}
	}
	switch input.Kind {
	case serveroperationresourcecap.AccessoryKindSnapshot, serveroperationresourcecap.AccessoryKindBackup:
		switch input.PeriodUnitCode {
		case "day":
			return input.PeriodCount >= 1 && input.PeriodCount <= 3650
		case "month":
			return input.PeriodCount >= 1 && input.PeriodCount <= 120
		case "year":
			return input.PeriodCount >= 1 && input.PeriodCount <= 10
		default:
			return false
		}
	case serveroperationresourcecap.AccessoryKindTraffic:
		return input.FreeCountPerServer == 0 && input.PeriodUnitCode == "once" && input.PeriodCount == 1
	default:
		return false
	}
}

func (s *accessoryPricingImpl) authorize(ctx context.Context, permission string) (int, error) {
	current := s.bizCtx.Current(ctx)
	if current.UserID <= 0 || current.TenantID != idccap.DefaultTenantID || current.ActingUserID != 0 ||
		current.IsImpersonation || current.ActingAsTenant || current.DataScopeUnsupported ||
		(current.DataScope != 1 && current.DataScope != 2) {
		return 0, bizerr.NewCode(CodeAccessoryPriceNotAccessible)
	}
	audience, err := s.principal.Current(ctx)
	if err != nil || audience == nil || audience.UserID != current.UserID || audience.Code != principal.AudienceStaff ||
		audience.Status != principal.AudienceStatusActive || audience.CustomerNo != "" {
		return 0, bizerr.NewCode(CodeAccessoryPriceNotAccessible)
	}
	keys := []authz.PermissionKey{authz.PermissionKey(permission), authz.PermissionKey(idccap.HostPermissionWildcard)}
	grants, err := s.authorization.BatchHasPermissions(ctx, keys)
	if err != nil {
		return 0, bizerr.WrapCode(err, CodeAccessoryPriceNotAccessible)
	}
	platformAdmin, err := s.authorization.IsPlatformAdmin(ctx, authz.UserID(strconv.Itoa(current.UserID)))
	if err != nil || (!grants[keys[0]] && !grants[keys[1]] && !platformAdmin) {
		return 0, bizerr.NewCode(CodeAccessoryPriceNotAccessible)
	}
	return current.UserID, nil
}

func (s *accessoryPricingImpl) Preview(ctx context.Context, input AccessoryQuoteInput) (*AccessoryQuote, error) {
	if ctx == nil || ctx.Err() != nil {
		return nil, bizerr.NewCode(CodeAccessoryPriceInvalid)
	}
	var selectedPackage *TrafficPackage
	if input.Action == serveroperationresourcecap.ActionTrafficAdd {
		var err error
		selectedPackage, err = s.resolveTrafficPackage(ctx, input)
		if err != nil {
			return nil, err
		}
		input.Parameters, err = freezeTrafficParameters(selectedPackage.TrafficGB)
		if err != nil {
			return nil, bizerr.WrapCode(err, CodeAccessoryPriceInvalid)
		}
	} else if input.PackageNo != "" {
		return nil, bizerr.NewCode(CodeAccessoryPriceInvalid)
	}
	normalized, kind, quantity, err := normalizeAccessoryQuoteInput(input)
	if err != nil {
		return nil, bizerr.NewCode(CodeAccessoryPriceInvalid)
	}
	var service *serviceRow
	if err = s.database.GetScan(ctx, &service, `SELECT service.service_no,service.customer_no,service.currency_code,
service.pool_no,COALESCE(service.channel_no,'') AS channel_no,service.entitlement_status_code,
service.runtime_status_code,service.provider_task_status_code,service.version,
COALESCE(service.current_price_snapshot_no,'') AS current_price_snapshot_no,
customer.version AS customer_subject_version,
EXISTS (SELECT 1 FROM plugin_idc_core_service_traffic_suspension suspension
 WHERE suspension.tenant_id=service.tenant_id AND suspension.service_no=service.service_no AND suspension.active) AS traffic_suspended
FROM plugin_idc_core_service_instance service
JOIN plugin_idc_core_customer_subject customer
 ON customer.tenant_id=service.tenant_id AND customer.customer_no=service.customer_no
WHERE service.tenant_id=0 AND service.service_no=? AND service.customer_no=?`, input.ServiceNo, input.CustomerNo); err != nil {
		return nil, bizerr.WrapCode(err, CodeAccessoryQuoteUnavailable)
	}
	if service == nil || service.ChannelNo == "" || !allowedAction(service, input.Action) {
		return nil, bizerr.NewCode(CodeAccessoryQuoteUnavailable)
	}
	route := serveroperationresourcecap.CapabilityRoute{
		ServiceNo: service.ServiceNo, ChannelNo: service.ChannelNo, PoolNo: service.PoolNo,
	}
	if err = s.parameters.ValidateParameters(ctx, route, input.Action, normalized); err != nil {
		return nil, bizerr.NewCode(CodeAccessoryQuoteUnavailable)
	}
	directory, err := s.directory.GetAccessories(ctx, route)
	if err != nil || directory == nil || !directory.ExpiresAt.After(time.Now().UTC()) ||
		!containsAccessoryAction(directory.Actions, input.Action) {
		return nil, bizerr.NewCode(CodeAccessoryQuoteUnavailable)
	}
	existingCount := countAccessoryItems(directory.Items, kind)
	baselineObjectNos, baselineSnapshotNo, baselineObservedAt := accessoryQuoteBaseline(directory, kind)
	reservedValue, err := s.database.GetValue(ctx, `SELECT
 (SELECT count(*) FROM plugin_idc_core_server_operation_execution execution
  WHERE execution.tenant_id=0 AND execution.service_no=? AND execution.action_code=?
    AND execution.status_code IN ('ready','pending','running','result_unknown'))
 +
 (SELECT count(*) FROM plugin_idc_core_business_operation operation
  JOIN plugin_idc_core_server_accessory_quote quote
    ON quote.tenant_id=operation.tenant_id AND quote.quote_no=operation.quote_no
  WHERE operation.tenant_id=0 AND operation.service_no=? AND quote.action_code=?
    AND operation.operation_type_code='addon'
    AND operation.status_code='awaiting_financial_settlement')`,
		service.ServiceNo, input.Action, service.ServiceNo, input.Action)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeAccessoryQuoteUnavailable)
	}
	reservedCount := reservedValue.Int()
	var policy *struct {
		PolicyNo           string `orm:"policy_no"`
		Version            int64  `orm:"policy_version"`
		FreeCountPerServer int    `orm:"free_count_per_server"`
		UnitAmountMinor    int64  `orm:"unit_amount_minor"`
		PeriodUnitCode     string `orm:"period_unit_code"`
		PeriodCount        int    `orm:"period_count"`
	}
	if err = s.database.GetScan(ctx, &policy, `SELECT policy.policy_no,version.policy_version,
version.free_count_per_server,version.unit_amount_minor,version.period_unit_code,version.period_count
FROM plugin_idc_core_server_accessory_price_policy policy
JOIN plugin_idc_core_server_accessory_price_policy_version version
 ON version.policy_no=policy.policy_no AND version.policy_version=policy.current_version
WHERE policy.tenant_id=0 AND policy.accessory_kind=? AND policy.currency_code=?
 AND (?='' OR policy.policy_no=?)`, kind, service.CurrencyCode, input.PackageNo, input.PackageNo); err != nil {
		return nil, bizerr.WrapCode(err, CodeAccessoryQuoteUnavailable)
	}
	if policy == nil {
		return nil, bizerr.NewCode(CodeAccessoryQuoteUnavailable)
	}
	if selectedPackage != nil && policy.Version != selectedPackage.Version {
		return nil, bizerr.NewCode(CodeAccessoryPriceConflict)
	}
	chargedQuantity := quantity
	if kind != serveroperationresourcecap.AccessoryKindTraffic && existingCount+reservedCount < policy.FreeCountPerServer {
		chargedQuantity = 0
	}
	total := policy.UnitAmountMinor * int64(chargedQuantity)
	commerceContext, err := s.loadAccessoryCommerceContext(ctx, service, directory, kind, total)
	if err != nil {
		return nil, bizerr.NewCode(CodeAccessoryQuoteUnavailable)
	}
	if selectedPackage != nil {
		commerceContext.NameZhCN, commerceContext.NameEnUS = selectedPackage.NameZhCN, selectedPackage.NameEnUS
	}
	subtotal, net, tax := commerceContext.SubtotalMinor, commerceContext.NetAmountMinor, commerceContext.TaxAmountMinor
	idempotencyDigest, err := digestValue(struct{ CustomerNo, Key string }{input.CustomerNo, input.IdempotencyKey})
	if err != nil {
		return nil, bizerr.NewCode(CodeAccessoryPriceInvalid)
	}
	requestDigest, err := digestValue(struct {
		CustomerNo, ServiceNo string
		Action                serveroperationresourcecap.Action
		Parameters            json.RawMessage
	}{input.CustomerNo, input.ServiceNo, input.Action, normalized})
	if err != nil {
		return nil, bizerr.NewCode(CodeAccessoryPriceInvalid)
	}
	quoteNo, err := deterministicReference("aqt_", struct{ CustomerNo, Digest string }{input.CustomerNo, idempotencyDigest})
	if err != nil {
		return nil, bizerr.NewCode(CodeAccessoryPriceInvalid)
	}
	quote := &AccessoryQuote{QuoteNo: quoteNo, CustomerNo: input.CustomerNo, ServiceNo: input.ServiceNo,
		Action: input.Action, Kind: kind, CurrencyCode: service.CurrencyCode, RequestedQuantity: quantity,
		ExistingObjectCount: existingCount, ReservedCreateCount: reservedCount,
		FreeCountPerServer: policy.FreeCountPerServer, ChargedQuantity: chargedQuantity,
		UnitAmountMinor: policy.UnitAmountMinor, SubtotalMinor: subtotal, NetAmountMinor: net,
		TaxAmountMinor: tax, TotalAmountMinor: total,
		PeriodUnitCode: policy.PeriodUnitCode, PeriodCount: policy.PeriodCount,
		PolicyNo: policy.PolicyNo, PolicyVersion: policy.Version,
		CustomerSubjectVersion: service.CustomerSubjectVersion, ExpectedServiceVersion: service.Version,
		BaselineObjectNos: baselineObjectNos, BaselineSnapshotNo: baselineSnapshotNo,
		BaselineObservedAt: baselineObservedAt}
	if selectedPackage != nil {
		quote.TrafficGB, quote.PackageNameZhCN, quote.PackageNameEnUS = selectedPackage.TrafficGB, selectedPackage.NameZhCN, selectedPackage.NameEnUS
	}
	contentDigest, err := accessoryQuoteContentDigest(quote)
	if err != nil {
		return nil, bizerr.NewCode(CodeAccessoryPriceInvalid)
	}
	return s.persistAccessoryPreview(ctx, quote, normalized, idempotencyDigest, requestDigest, contentDigest, directory.ExpiresAt, commerceContext)
}

// persistAccessoryPreview owns append-only quote storage and exact idempotent
// replay, after the original service and capability have both been verified.
func (s *accessoryPricingImpl) persistAccessoryPreview(ctx context.Context, quote *AccessoryQuote,
	normalized json.RawMessage, idempotencyDigest, requestDigest, contentDigest string,
	capabilityExpiresAt time.Time, commerceContext *accessoryCommerceContext) (*AccessoryQuote, error) {
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var replay *struct {
			RequestDigest string    `orm:"request_digest"`
			ContentDigest string    `orm:"content_digest"`
			IssuedAt      time.Time `orm:"issued_at"`
			ExpiresAt     time.Time `orm:"expires_at"`
		}
		if loadErr := s.database.GetScan(ctx, &replay, `SELECT request_digest,content_digest,issued_at,expires_at
FROM plugin_idc_core_server_accessory_quote
WHERE tenant_id=0 AND customer_no=? AND idempotency_digest=? FOR UPDATE`, quote.CustomerNo, idempotencyDigest); loadErr != nil {
			return loadErr
		}
		if replay != nil {
			if replay.RequestDigest != requestDigest || replay.ContentDigest != contentDigest {
				return bizerr.NewCode(CodeAccessoryPriceConflict)
			}
			quote.IssuedAt, quote.ExpiresAt, quote.Reused = replay.IssuedAt.UTC(), replay.ExpiresAt.UTC(), true
			return nil
		}
		now, nowErr := databaseNow(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		quote.IssuedAt, quote.ExpiresAt = now.UTC(), now.Add(accessoryQuoteTTL).UTC()
		if capabilityExpiresAt.Before(quote.ExpiresAt) {
			quote.ExpiresAt = capabilityExpiresAt.UTC()
		}
		if !quote.ExpiresAt.After(quote.IssuedAt) {
			return bizerr.NewCode(CodeAccessoryQuoteUnavailable)
		}
		baselineJSON, marshalErr := json.Marshal(quote.BaselineObjectNos)
		if marshalErr != nil {
			return marshalErr
		}
		_, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_server_accessory_quote
(tenant_id,quote_no,customer_no,service_no,action_code,accessory_kind,currency_code,
requested_quantity,existing_object_count,reserved_create_count,free_count_per_server,charged_quantity,
unit_amount_minor,subtotal_minor,net_amount_minor,tax_amount_minor,total_amount_minor,
period_unit_code,period_count,policy_no,policy_version,parameters,customer_subject_version,
expected_service_version,tax_policy_no,tax_treatment_code,tax_rate_basis_points,tax_category_code,
rounding_scope_code,rounding_mode_code,rounding_quantum_minor,
baseline_object_nos,baseline_snapshot_no,baseline_observed_at,
idempotency_digest,request_digest,content_digest,issued_at,expires_at)
VALUES (0,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, quote.QuoteNo, quote.CustomerNo, quote.ServiceNo,
			quote.Action, quote.Kind, quote.CurrencyCode, quote.RequestedQuantity, quote.ExistingObjectCount,
			quote.ReservedCreateCount, quote.FreeCountPerServer, quote.ChargedQuantity, quote.UnitAmountMinor,
			quote.SubtotalMinor, quote.NetAmountMinor, quote.TaxAmountMinor, quote.TotalAmountMinor,
			quote.PeriodUnitCode, quote.PeriodCount, quote.PolicyNo, quote.PolicyVersion, string(normalized),
			quote.CustomerSubjectVersion, quote.ExpectedServiceVersion, accessoryQuoteTaxValue(quote, commerceContext, "policy"),
			accessoryQuoteTaxValue(quote, commerceContext, "treatment"), accessoryQuoteTaxRate(quote, commerceContext),
			accessoryQuoteTaxValue(quote, commerceContext, "category"), accessoryQuoteTaxValue(quote, commerceContext, "scope"),
			accessoryQuoteTaxValue(quote, commerceContext, "mode"), accessoryQuoteTaxQuantum(quote, commerceContext),
			string(baselineJSON), quote.BaselineSnapshotNo, nullableAccessoryBaselineObservedAt(quote),
			idempotencyDigest, requestDigest, contentDigest, quote.IssuedAt, quote.ExpiresAt)
		if insertErr != nil {
			return insertErr
		}
		return s.persistAccessoryQuoteCommerce(ctx, quote, normalized, requestDigest, contentDigest, commerceContext)
	})
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeAccessoryQuoteUnavailable)
	}
	return quote, nil
}

func accessoryQuoteBaseline(directory *serveroperationresourcecap.AccessoryProjection,
	kind serveroperationresourcecap.AccessoryKind) ([]string, string, time.Time) {
	if directory == nil || (kind != serveroperationresourcecap.AccessoryKindSnapshot &&
		kind != serveroperationresourcecap.AccessoryKindBackup) {
		return []string{}, "", time.Time{}
	}
	return accessoryObjectNos(directory.Items, kind), directory.SnapshotNo, directory.ObservedAt.UTC()
}

func accessoryQuoteContentDigest(quote *AccessoryQuote) (string, error) {
	if quote == nil {
		return "", ErrInvalidInput
	}
	digestQuote := *quote
	// The owner snapshot number and object set fence checkout. ObservedAt only
	// describes cache freshness and may differ across equivalent replay reads.
	digestQuote.BaselineObservedAt = time.Time{}
	return digestValue(&digestQuote)
}

func nullableAccessoryBaselineObservedAt(quote *AccessoryQuote) any {
	if quote == nil || quote.BaselineObservedAt.IsZero() {
		return nil
	}
	return quote.BaselineObservedAt
}

func accessoryObjectNos(items []serveroperationresourcecap.AccessoryItem,
	kind serveroperationresourcecap.AccessoryKind) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		if item.Kind == kind && validReference(item.ObjectNo, 128) {
			result = append(result, item.ObjectNo)
		}
	}
	sort.Strings(result)
	return result
}

func normalizeAccessoryQuoteInput(input AccessoryQuoteInput) (json.RawMessage,
	serveroperationresourcecap.AccessoryKind, int, error) {
	if !validReference(input.CustomerNo, 40) || !validReference(input.ServiceNo, 40) ||
		len(input.IdempotencyKey) < 8 || len(input.IdempotencyKey) > 256 ||
		input.IdempotencyKey != strings.TrimSpace(input.IdempotencyKey) {
		return nil, "", 0, ErrInvalidInput
	}
	var parameters map[string]any
	if len(input.Parameters) == 0 || len(input.Parameters) > 16*1024 ||
		json.Unmarshal(input.Parameters, &parameters) != nil || parameters == nil {
		return nil, "", 0, ErrInvalidInput
	}
	normalized, err := json.Marshal(parameters)
	if err != nil {
		return nil, "", 0, ErrInvalidInput
	}
	switch input.Action {
	case serveroperationresourcecap.ActionSnapshotCreate:
		return normalized, serveroperationresourcecap.AccessoryKindSnapshot, 1, nil
	case serveroperationresourcecap.ActionBackupCreate:
		return normalized, serveroperationresourcecap.AccessoryKindBackup, 1, nil
	case serveroperationresourcecap.ActionTrafficAdd:
		var value struct {
			TrafficGB int `json:"trafficGb"`
		}
		if json.Unmarshal(normalized, &value) != nil || value.TrafficGB < 1 || value.TrafficGB > 1000000 {
			return nil, "", 0, ErrInvalidInput
		}
		if !validReference(input.PackageNo, 40) {
			return nil, "", 0, ErrInvalidInput
		}
		return normalized, serveroperationresourcecap.AccessoryKindTraffic, 1, nil
	default:
		return nil, "", 0, ErrActionUnavailable
	}
}

func containsAccessoryAction(actions []serveroperationresourcecap.Action,
	wanted serveroperationresourcecap.Action) bool {
	for _, action := range actions {
		if action == wanted {
			return true
		}
	}
	return false
}

func countAccessoryItems(items []serveroperationresourcecap.AccessoryItem,
	kind serveroperationresourcecap.AccessoryKind) int {
	if kind == serveroperationresourcecap.AccessoryKindTraffic {
		return 0
	}
	count := 0
	for index := range items {
		if items[index].Kind == kind {
			count++
		}
	}
	return count
}
