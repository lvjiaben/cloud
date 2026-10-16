// This file freezes a paid upgrade against a customer-owned compiler-validated sales quote.
// HTTP callers never supply amounts, Provider parameters, or customer identities.

package entitlementupgrade

import (
	"context"

	"encoding/json"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/commerce"
	"strings"
	"time"
)

type targetEvidence struct {
	QuoteNo, QuoteItemNo, ProductNo, VersionNo, BillingRuleNo, ConfigurationSnapshotNo, StructuredNo, CurrencyCode string
	TypedValues                                                                                                    string
	Line                                                                                                           catalog.PurchaseQuoteLine
	PeriodNet                                                                                                      int64
}

// periodic reconstructs the immutable paid line using the same discount allocation as checkout.
func (s *serviceImpl) periodic(ctx context.Context, item string) (catalog.PurchaseQuoteLine, int64, error) {
	var line *catalog.PurchaseQuoteLine
	err := s.database.GetScan(ctx, &line, `SELECT item.product_no,item.version_no,item.billing_rule_no,quantity.quantity,purchase_cycles,period_unit_code,period_count,
 tax_policy_no,price_includes_tax,tax_treatment_code,tax_rate_basis_points,tax_category_code,rounding_scope_code,rounding_mode_code,rounding_quantum_minor
 FROM plugin_idc_core_commerce_quote_item_snapshot item JOIN plugin_idc_core_commerce_quote_item_quantity_snapshot quantity ON quantity.tenant_id=item.tenant_id AND quantity.quote_item_no=item.quote_item_no WHERE item.tenant_id=0 AND item.quote_item_no=?`, item)
	if err != nil {
		return catalog.PurchaseQuoteLine{}, 0, err
	}
	if line == nil {
		return catalog.PurchaseQuoteLine{}, 0, bizerr.NewCode(CodeInvalid)
	}
	err = s.database.GetScan(ctx, &line.Charges, `SELECT charge_kind_code AS kind_code,COALESCE(option_no,'') AS option_no,
 COALESCE(configuration_price_dimension_no,'') AS configuration_price_dimension_no,quantity,charge_amount_minor,setup_fee_minor,priced_amount_minor,price_source_digest
 FROM plugin_idc_core_commerce_quote_charge_snapshot WHERE tenant_id=0 AND quote_item_no=? ORDER BY line_number LIMIT 5000`, item)
	if err != nil {
		return *line, 0, err
	}
	var adjustments []commerce.QuoteAdjustment
	err = s.database.GetScan(ctx, &adjustments, `SELECT adjustment_type_code AS type_code,source_reference,source_version_digest,amount_minor,original_period_amount_minor,target_period_amount_minor
 FROM plugin_idc_core_commerce_quote_adjustment_snapshot WHERE tenant_id=0 AND quote_item_no=? ORDER BY application_order LIMIT 4096`, item)
	if err != nil {
		return *line, 0, err
	}
	amount, err := commerce.CalculatePeriodicNetAmountWithAdjustments(*line, adjustments)
	return *line, amount, err
}

func (s *serviceImpl) target(ctx context.Context, quote, customer string, state *paidState) (*targetEvidence, error) {
	var t *struct {
		QuoteItemNo, ConfigurationSnapshotNo, StructuredNo, TypedValues string
		ExpiresAt                                                       time.Time
	}
	err := s.database.GetScan(ctx, &t, `SELECT i.quote_item_no,i.configuration_snapshot_no,sc.snapshot_no AS structured_no,sc.typed_value_snapshot::text AS typed_values,q.expires_at
 FROM plugin_idc_core_commerce_quote_snapshot q
 JOIN plugin_idc_core_commerce_quote_item_snapshot i ON i.tenant_id=q.tenant_id AND i.quote_no=q.quote_no
 JOIN plugin_idc_core_commerce_structured_configuration_snapshot sc ON sc.tenant_id=q.tenant_id AND sc.quote_item_no=i.quote_item_no AND sc.customer_no=q.customer_no
 WHERE q.tenant_id=0 AND q.quote_no=? AND q.customer_no=? AND q.item_count=1
 AND i.product_no=? AND i.version_no=? AND i.currency_code=? AND i.charge_type_code='recurring'
 AND sc.order_no IS NULL AND NOT EXISTS(SELECT 1 FROM plugin_idc_core_commerce_structured_configuration_file_snapshot f WHERE f.snapshot_no=sc.snapshot_no)
 AND NOT EXISTS(SELECT 1 FROM plugin_idc_core_commerce_order o WHERE o.quote_no=q.quote_no)
 AND NOT EXISTS(SELECT 1 FROM plugin_idc_core_commerce_addon_target a WHERE a.quote_no=q.quote_no)`, quote, customer, state.ProductNo, state.VersionNo, state.CurrencyCode)
	if err != nil {
		return nil, err
	}
	if t == nil || !t.ExpiresAt.After(time.Now()) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	line, amount, err := s.periodic(ctx, t.QuoteItemNo)
	if err != nil {
		return nil, err
	}
	if line.Quantity != 1 || line.PeriodUnitCode != state.PeriodUnitCode || line.PeriodCount != state.PeriodCount || line.PurchaseCycles != state.PurchaseCycles {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	return &targetEvidence{QuoteNo: quote, QuoteItemNo: t.QuoteItemNo, ProductNo: state.ProductNo, VersionNo: state.VersionNo, BillingRuleNo: line.BillingRuleNo, ConfigurationSnapshotNo: t.ConfigurationSnapshotNo, StructuredNo: t.StructuredNo, CurrencyCode: state.CurrencyCode, TypedValues: t.TypedValues, Line: line, PeriodNet: amount}, nil
}

// Quote binds one unexpired normal quote and its frozen configuration to the exact paid service revision.
func (s *serviceImpl) Quote(ctx context.Context, no, quote, key string) (*View, error) {
	customer, err := s.customer(ctx)
	if err != nil {
		return nil, err
	}
	if len(no) > 40 || no == "" || len(quote) > 40 || quote == "" || len(key) < 8 || len(key) > 256 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	idem, err := digest([]string{customer, key})
	if err != nil {
		return nil, err
	}
	request, err := digest([]string{no, quote})
	if err != nil {
		return nil, err
	}
	var out *View
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, e := s.database.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended(?,0))`, "upgrade:"+customer+":"+idem); e != nil {
			return e
		}
		var replay *entity.EntitlementUpgrade
		if e := s.database.Model(do.EntitlementUpgrade{}).Ctx(ctx).Where(do.EntitlementUpgrade{TenantId: 0, CustomerNo: customer, IdempotencyDigest: idem}).Scan(&replay); e != nil {
			return e
		}
		if replay != nil {
			if replay.RequestDigest != request {
				return bizerr.NewCode(CodeConflict)
			}
			out = view(replay)
			return nil
		}
		state, e := s.state(ctx, no, customer)
		if e != nil {
			return e
		}
		target, e := s.target(ctx, quote, customer, state)
		if e != nil {
			return e
		}
		compiled, e := s.compiler.CompileUpgrade(ctx, serveroperationresourcecap.UpgradeConfigurationRequest{ServiceNo: no, CustomerNo: customer, ProductConfiguration: json.RawMessage(state.ProductConfiguration), CurrentConfiguration: json.RawMessage(state.CurrentConfiguration), TargetConfiguration: json.RawMessage(target.TypedValues)})
		if e != nil || compiled == nil {
			return bizerr.NewCode(CodeUnavailable)
		}
		_, actual, e := s.periodic(ctx, state.QuoteItemNo)
		if e != nil {
			return e
		}
		// A prior successful resize establishes the full-period equivalent paid basis.
		var prior *struct{ TargetPeriodNetMinor int64 }
		if e = s.database.GetScan(ctx, &prior, `SELECT target_period_net_minor FROM plugin_idc_core_entitlement_upgrade WHERE tenant_id=0 AND entitlement_no=? AND customer_no=? AND status_code='succeeded' AND target_price_snapshot_no=? ORDER BY id DESC LIMIT 1`, no, customer, state.PriceNo); e != nil {
			return e
		}
		if prior != nil {
			actual = prior.TargetPeriodNetMinor
		}
		now := time.Now().UTC()
		net, e := ProrateDifference(PeriodPrice{actual, target.PeriodNet, state.StartedAt, state.ExpiresAt, now})
		if e != nil {
			return e
		}
		taxLine := target.Line
		taxLine.PriceIncludesTax = false
		taxLine.Charges = []catalog.PurchaseQuoteCharge{{KindCode: "base", Quantity: 1, ChargeAmountMinor: net, PricedAmountMinor: net}}
		amounts, e := commerce.CalculatePricingAmounts(taxLine, 0)
		if e != nil {
			return e
		}
		frozen, e := json.Marshal(target)
		if e != nil {
			return e
		}
		content, e := digest(target)
		if e != nil {
			return e
		}
		upgradeNo := "UPG_" + idem[:32]
		expires := now.Add(15 * time.Minute)
		r := do.EntitlementUpgrade{TenantId: 0, UpgradeNo: upgradeNo, CustomerNo: customer, EntitlementNo: no, ProductNo: state.ProductNo, SourceVersion: state.Version, SourceConfigurationNo: state.ConfigurationSnapshotNo, CurrencyCode: state.CurrencyCode, SourcePeriodNetMinor: actual, TargetPeriodNetMinor: target.PeriodNet, ChargeNetMinor: net, ChargeTaxMinor: amounts.TaxAmountMinor, ChargeTotalMinor: amounts.TotalAmountMinor, PeriodStartedAt: &state.StartedAt, PeriodExpiresAt: &state.ExpiresAt, QuoteExpiresAt: &expires, FrozenTarget: string(frozen), FrozenPrice: string(frozen), ProviderParameters: string(compiled.Parameters), ChannelNo: compiled.Route.ChannelNo, PoolNo: compiled.Route.PoolNo, StatusCode: statusQuoted, IdempotencyDigest: idem, RequestDigest: request, ContentDigest: content}
		if _, e = s.database.Model(do.EntitlementUpgrade{}).Ctx(ctx).Data(r).Insert(); e != nil {
			return e
		}
		stored, e := s.load(ctx, upgradeNo, customer)
		if e != nil {
			return e
		}
		out = view(stored)
		return nil
	})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeConflict)
	}
	return out, nil
}

// decodeTarget only consumes previously persisted compiler evidence.
func decodeTarget(row *entity.EntitlementUpgrade) (*targetEvidence, error) {
	var t targetEvidence
	if json.Unmarshal([]byte(row.FrozenTarget), &t) != nil || strings.TrimSpace(t.QuoteItemNo) == "" {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	return &t, nil
}
