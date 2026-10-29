// Native quotes reuse the accessory merchant prices and commerce snapshots for a real owned entitlement.

package serveroperation

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

type nativeAccessoryPolicy struct {
	PolicyNo, PeriodUnitCode, NameZhCN, NameEnUS string
	PolicyVersion, UnitAmountMinor               int64
	FreeCountPerServer, PeriodCount, TrafficGB   int
}

// Quote authorizes current ownership and freezes only published merchant prices and fresh resource facts.
func (s *nativeCommercialService) Quote(ctx context.Context, input servercommercialcap.QuoteRequest) (*servercommercialcap.Quote, error) {
	if input.Action == serveroperationresourcecap.ActionResize || input.Action == serveroperationresourcecap.ActionChangeIP {
		return s.nativeQuotePriced(ctx, input)
	}
	if len(input.IdempotencyKey) < 8 || len(input.IdempotencyKey) > 256 || strings.TrimSpace(input.IdempotencyKey) != input.IdempotencyKey {
		return nil, ErrInvalidInput
	}
	if err := s.authorizeNativeTarget(ctx, input.Target.CustomerNo, input.Action); err != nil {
		return nil, err
	}
	var result *servercommercialcap.Quote
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		target, err := s.readNativeCommercialTarget(ctx, input.Target, true)
		if err != nil {
			return err
		}
		parameters, kind, quantity, err := normalizeAccessoryQuoteInput(AccessoryQuoteInput{CustomerNo: target.CustomerNo, ServiceNo: target.EntitlementNo, Action: input.Action, Parameters: input.Parameters, IdempotencyKey: input.IdempotencyKey})
		if input.Action == serveroperationresourcecap.ActionTrafficAdd {
			parameters = json.RawMessage(`{}`)
			kind = serveroperationresourcecap.AccessoryKindTraffic
			quantity = 1
			err = nil
			if strings.TrimSpace(string(input.Parameters)) != "" && strings.TrimSpace(string(input.Parameters)) != "{}" {
				return ErrInvalidInput
			}
		}
		if err != nil || !accessorySettlementAction(string(input.Action), string(kind)) || !nativeSnapshotAllows(input.Target, input.Action) {
			return ErrInvalidInput
		}
		requestDigest, err := digestValue(struct {
			EntitlementNo, PackageNo string
			Action                   serveroperationresourcecap.Action
			Parameters               json.RawMessage
		}{target.EntitlementNo, input.PackageNo, input.Action, parameters})
		if err != nil {
			return err
		}
		idempotencyDigest, err := digestValue(struct{ CustomerNo, Key string }{target.CustomerNo, "native-quote:" + input.IdempotencyKey})
		if err != nil {
			return err
		}
		if stored, storedRequest, loadErr := s.loadNativeAccessoryQuote(ctx, target.CustomerNo, idempotencyDigest); loadErr != nil {
			return loadErr
		} else if stored != nil {
			if storedRequest != requestDigest {
				return ErrOperationConflict
			}
			result = projectNativeAccessoryQuote(stored, true)
			return nil
		}
		busy, err := s.nativeCommercialBusy(ctx, target.EntitlementNo, "")
		if err != nil {
			return err
		}
		if busy {
			return ErrOperationConflict
		}
		policy, err := s.nativeAccessoryPolicy(ctx, target, kind, input.PackageNo)
		if err != nil {
			return err
		}
		existing, reserved, err := nativeAccessoryOccupancy(ctx, s.database, input.Target, kind)
		if err != nil {
			return err
		}
		charged := quantity
		if kind != serveroperationresourcecap.AccessoryKindTraffic && existing+reserved < policy.FreeCountPerServer {
			charged = 0
		}
		if kind == serveroperationresourcecap.AccessoryKindTraffic {
			parameters, err = freezeTrafficParameters(policy.TrafficGB)
			if err != nil {
				return err
			}
		}
		if err = s.pricing.parameters.ValidateParameters(ctx, serveroperationresourcecap.CapabilityRoute{ServiceNo: input.Target.LocalResourceNo, ChannelNo: input.Target.ChannelNo, PoolNo: input.Target.PoolNo}, input.Action, parameters); err != nil {
			return err
		}
		total := policy.UnitAmountMinor * int64(charged)
		commerceContext, err := s.nativeAccessoryCommerceContext(ctx, target, input.Target, policy, total)
		if err != nil {
			return err
		}
		quoteNo, err := deterministicReference("nqt_", struct{ CustomerNo, Digest string }{target.CustomerNo, idempotencyDigest})
		if err != nil {
			return err
		}
		frozen, err := json.Marshal(input.Target)
		if err != nil {
			return err
		}
		projection := &serveroperationresourcecap.AccessoryProjection{Items: input.Target.Items, SnapshotNo: input.Target.SnapshotNo, ObservedAt: input.Target.ObservedAt}
		baseline, snapshot, observed := accessoryQuoteBaseline(projection, kind)
		quote := &AccessoryQuote{QuoteNo: quoteNo, CustomerNo: target.CustomerNo, SubjectTypeCode: nativeAccessorySubject, SubjectNo: target.EntitlementNo, ExpectedSubjectVersion: target.EntitlementVersion, ExpectedServiceVersion: target.EntitlementVersion, NativeTargetSnapshot: string(frozen), Action: input.Action, Kind: kind, CurrencyCode: target.CurrencyCode, RequestedQuantity: quantity, ExistingObjectCount: existing, ReservedCreateCount: reserved, FreeCountPerServer: policy.FreeCountPerServer, ChargedQuantity: charged, UnitAmountMinor: policy.UnitAmountMinor, SubtotalMinor: commerceContext.SubtotalMinor, NetAmountMinor: commerceContext.NetAmountMinor, TaxAmountMinor: commerceContext.TaxAmountMinor, TotalAmountMinor: total, PeriodUnitCode: policy.PeriodUnitCode, PeriodCount: policy.PeriodCount, PolicyNo: policy.PolicyNo, PolicyVersion: policy.PolicyVersion, CustomerSubjectVersion: target.CustomerVersion, BaselineObjectNos: baseline, BaselineSnapshotNo: snapshot, BaselineObservedAt: observed, TrafficGB: policy.TrafficGB, PackageNameZhCN: policy.NameZhCN, PackageNameEnUS: policy.NameEnUS}
		content, err := accessoryQuoteContentDigest(quote)
		if err != nil {
			return err
		}
		stored, err := s.pricing.persistAccessoryPreview(ctx, quote, parameters, idempotencyDigest, requestDigest, content, input.Target.ExpiresAt, commerceContext)
		if err != nil {
			return err
		}
		result = projectNativeAccessoryQuote(stored, stored.Reused)
		return nil
	})
	return result, err
}

// nativeAccessoryPolicy binds traffic packages to the actual product and requires an enabled published version.
func (s *nativeCommercialService) nativeAccessoryPolicy(ctx context.Context, target *nativeCommercialTarget, kind serveroperationresourcecap.AccessoryKind, packageNo string) (*nativeAccessoryPolicy, error) {
	if kind == serveroperationresourcecap.AccessoryKindTraffic && !validReference(packageNo, 40) || kind != serveroperationresourcecap.AccessoryKindTraffic && packageNo != "" {
		return nil, ErrInvalidInput
	}
	var rows []nativeAccessoryPolicy
	err := s.database.GetScan(ctx, &rows, `SELECT policy.policy_no,version.policy_version,version.free_count_per_server,version.unit_amount_minor,version.period_unit_code,version.period_count,version.traffic_gb,version.name_zh_cn,version.name_en_us
 FROM plugin_idc_core_server_accessory_price_policy policy JOIN plugin_idc_core_server_accessory_price_policy_version version
 ON version.policy_no=policy.policy_no AND version.policy_version=policy.current_version
 WHERE policy.tenant_id=0 AND policy.enabled AND policy.accessory_kind=? AND policy.currency_code=?
 AND (?='' OR policy.policy_no=?) AND (?<>'traffic' OR EXISTS(SELECT 1 FROM plugin_idc_core_traffic_package_product scope
 WHERE scope.tenant_id=0 AND scope.policy_no=policy.policy_no AND scope.policy_version=policy.current_version AND scope.product_no=?)) LIMIT 2`, kind, target.CurrencyCode, packageNo, packageNo, kind, target.ProductNo)
	if err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, bizerr.NewCode(CodeAccessoryQuoteUnavailable)
	}
	return &rows[0], nil
}

// nativeAccessoryCommerceContext applies the shared inclusive tax calculation without changing the merchant's final price.
func (s *nativeCommercialService) nativeAccessoryCommerceContext(ctx context.Context, target *nativeCommercialTarget, snapshot servercommercialcap.TargetSnapshot, policy *nativeAccessoryPolicy, total int64) (*accessoryCommerceContext, error) {
	tax, err := s.prices.LoadTax(ctx, target.VersionNo, target.CurrencyCode)
	if err != nil {
		return nil, err
	}
	amounts, err := commerce.CalculatePricingAmounts(catalog.PurchaseQuoteLine{PeriodUnitCode: policy.PeriodUnitCode, PriceIncludesTax: true, TaxTreatmentCode: tax.Treatment, TaxRateBasisPoints: tax.Rate, TaxCategoryCode: tax.Category, RoundingScopeCode: tax.Scope, RoundingModeCode: tax.Mode, RoundingQuantumMinor: tax.Quantum, Charges: []catalog.PurchaseQuoteCharge{{KindCode: "base", Quantity: 1, ChargeAmountMinor: total, PricedAmountMinor: total}}}, 0)
	if err != nil || amounts.TotalAmountMinor != total {
		return nil, ErrOperationConflict
	}
	capabilityDigest, err := digestValue(snapshot)
	if err != nil {
		return nil, err
	}
	nameZh, nameEn := policy.NameZhCN, policy.NameEnUS
	if nameZh == "" {
		nameZh, nameEn = target.ProductNameZhCN, target.ProductNameEnUS
	}
	return &accessoryCommerceContext{ProductNo: target.ProductNo, VersionNo: target.VersionNo, BillingRuleNo: target.BillingRuleNo, ConfigurationSnapshotNo: target.PriceConfigurationNo, TaxPolicyNo: tax.PolicyNo, TaxTreatmentCode: tax.Treatment, TaxRateBasisPoints: tax.Rate, TaxCategoryCode: tax.Category, RoundingScopeCode: tax.Scope, RoundingModeCode: tax.Mode, RoundingQuantumMinor: tax.Quantum, CapabilityEvidenceNo: snapshot.SnapshotNo, CapabilityEvidenceDigest: capabilityDigest, NameZhCN: nameZh, NameEnUS: nameEn, SubtotalMinor: amounts.SubtotalMinor, NetAmountMinor: amounts.NetAmountMinor, TaxAmountMinor: amounts.TaxAmountMinor}, nil
}

func projectNativeAccessoryQuote(quote *AccessoryQuote, reused bool) *servercommercialcap.Quote {
	return &servercommercialcap.Quote{QuoteNo: quote.QuoteNo, EntitlementNo: quote.SubjectNo, CurrencyCode: quote.CurrencyCode, PolicyNo: quote.PolicyNo, Action: quote.Action, Kind: quote.Kind, TotalAmountMinor: quote.TotalAmountMinor, TaxAmountMinor: quote.TaxAmountMinor, PolicyVersion: quote.PolicyVersion, FreeCountPerServer: quote.FreeCountPerServer, ExistingObjectCount: quote.ExistingObjectCount, ReservedCreateCount: quote.ReservedCreateCount, ChargedQuantity: quote.ChargedQuantity, TrafficGB: quote.TrafficGB, PeriodUnit: quote.PeriodUnitCode, PeriodCount: quote.PeriodCount, ExpiresAt: quote.ExpiresAt, Reused: reused}
}
