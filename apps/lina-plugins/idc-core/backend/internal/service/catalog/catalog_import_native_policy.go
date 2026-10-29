// Native pricing policies are local merchant decisions. Supplier schema,
// identity, revision and opaque mappings remain immutable when prices change.
package catalog

import (
	"context"
	"encoding/json"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"math/big"
	"reflect"
)

type ImportedNativePricing struct {
	Mode, MarkupPercent string
	ManualPrices        []resourceinterfaceprojectioncap.CatalogPrice
}

// ReadImportedNativePricing requires a caller-authorized product version.
func ReadImportedNativePricing(ctx context.Context, db gdb.DB, version string) (*ImportedNativePricing, error) {
	var contract *struct{ ReadEvidence, Snapshot string }
	if err := db.Model(do.CatalogImportContract{}).Ctx(ctx).Where(do.CatalogImportContract{TenantId: 0, VersionNo: version}).Fields("read_evidence,snapshot").Scan(&contract); err != nil {
		return nil, err
	}
	if contract == nil {
		return nil, nil
	}
	var read resourceinterfaceprojectioncap.CatalogReadEvidence
	if json.Unmarshal([]byte(contract.ReadEvidence), &read) != nil || read.Validate() != nil {
		return nil, bizerr.NewCode(CodeManagementConflict)
	}
	var snapshot channelcommercecap.ImportSnapshot
	if json.Unmarshal([]byte(contract.Snapshot), &snapshot) != nil || snapshot.Validate() != nil {
		return nil, bizerr.NewCode(CodeManagementConflict)
	}
	var rows []nativePricingPolicyRow
	if err := db.GetScan(ctx, &rows, nativePricingPolicySQL, version, maximumPublicationPrices+1); err != nil {
		return nil, err
	}
	if len(rows) == 0 || len(rows) > maximumPublicationPrices {
		return nil, bizerr.NewCode(CodeManagementConflict)
	}
	expected, err := calculatePortableImportPrices(&snapshot, rows[0].MarkupPercent)
	if err != nil {
		return nil, err
	}
	expectedRules := map[string]*channelcommercecap.ImportWholeItemRule{}
	for _, price := range expected {
		expectedRules[price.portable.CurrencyCode+"\x00"+price.portable.TermReference] = price.portable.WholeItemRule
	}
	result := &ImportedNativePricing{Mode: "markup", MarkupPercent: rows[0].MarkupPercent, ManualPrices: []resourceinterfaceprojectioncap.CatalogPrice{}}
	fixed := 0
	for _, row := range rows {
		if row.MarkupPercent != result.MarkupPercent {
			result.Mode = "mixed"
			result.MarkupPercent = ""
		}
		if row.WholeItemRule == "" || row.WholeItemRule == "null" {
			continue
		}
		var rule channelcommercecap.ImportWholeItemRule
		if json.Unmarshal([]byte(row.WholeItemRule), &rule) != nil || rule.Validate() != nil {
			return nil, bizerr.NewCode(CodeManagementConflict)
		}
		if reflect.DeepEqual(&rule, expectedRules[row.CurrencyCode+"\x00"+row.UpstreamTermReference]) {
			continue
		}
		if rule.Kind != "fixed" || row.SetupFeeMinor == nil {
			return nil, bizerr.NewCode(CodeManagementConflict)
		}
		amount, err := nativePolicyMajor(*rule.FixedAmountMinor, row.CurrencyCode)
		if err != nil {
			return nil, err
		}
		setup, err := nativePolicyMajor(*row.SetupFeeMinor, row.CurrencyCode)
		if err != nil {
			return nil, err
		}
		fixed++
		result.ManualPrices = append(result.ManualPrices, resourceinterfaceprojectioncap.CatalogPrice{CurrencyCode: row.CurrencyCode, Period: importedPolicyPeriod(row.ChargeTypeCode, row.PeriodUnitCode), PeriodCount: row.PeriodCount, Amount: amount, SetupFee: setup})
	}
	if fixed == len(rows) {
		result.Mode = "manual"
		result.MarkupPercent = ""
	} else if fixed > 0 {
		result.Mode = "mixed"
		result.MarkupPercent = ""
	}
	return result, nil
}

type nativePricingPolicyRow struct {
	BillingRuleNo, CurrencyCode, ChargeTypeCode, PeriodUnitCode, WholeItemRule, MarkupPercent, UpstreamTermReference string
	PeriodCount                                                                                                      int
	SetupFeeMinor                                                                                                    *int64
}

const nativePricingPolicySQL = `SELECT term.billing_rule_no,term.currency_code,term.whole_item_rule,term.markup_percent,term.upstream_term_reference,rule.charge_type_code,rule.period_unit_code,rule.period_count,price.setup_fee_minor
FROM plugin_idc_core_catalog_import_term term
JOIN plugin_idc_core_catalog_billing_rule rule ON rule.tenant_id=term.tenant_id AND rule.version_no=term.version_no AND rule.billing_rule_no=term.billing_rule_no
LEFT JOIN plugin_idc_core_catalog_price price ON price.tenant_id=term.tenant_id AND price.version_no=term.version_no AND price.billing_rule_no=term.billing_rule_no AND price.currency_code=term.currency_code AND price.option_no IS NULL AND price.configuration_price_dimension_no IS NULL AND price.minimum_quantity=0 AND price.maximum_quantity IS NULL
WHERE term.tenant_id=0 AND term.version_no=? ORDER BY term.currency_code,term.billing_rule_no LIMIT ?`

func nativePolicyMajor(amount int64, currency string) (string, error) {
	precision, err := importCurrencyMinorUnit(currency)
	if err != nil || amount < 0 {
		return "", bizerr.NewCode(CodeManagementConflict)
	}
	denominator := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(precision)), nil)
	return new(big.Rat).SetFrac(big.NewInt(amount), denominator).FloatString(precision), nil
}

func importedPolicyPeriod(charge, period string) resourceinterfaceprojectioncap.CatalogPeriod {
	if charge == string(ChargeTypeOneTime) {
		return resourceinterfaceprojectioncap.CatalogPeriodOnce
	}
	return resourceinterfaceprojectioncap.CatalogPeriod(period)
}
