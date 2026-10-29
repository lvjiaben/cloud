// Native resale price edits replace the complete recurring amount. Imported
// option prices remain source-owned, and ordinary product editing is unchanged.
package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
)

func saveNativeWholeItemPrices(ctx context.Context, version string, inputs []normalizedPriceScheduleCreate) error {
	var contract *struct{ ReadEvidence string }
	if err := dao.CatalogImportContract.Ctx(ctx).Where(do.CatalogImportContract{TenantId: 0, VersionNo: version}).Fields("read_evidence").Scan(&contract); err != nil {
		return err
	}
	if contract == nil {
		return nil
	}
	var read resourceinterfaceprojectioncap.CatalogReadEvidence
	if json.Unmarshal([]byte(contract.ReadEvidence), &read) != nil || read.Validate() != nil {
		return bizerr.NewCode(CodeManagementConflict)
	}
	// One set update preserves the batch transaction and never changes history.
	values := []map[string]any{}
	for _, input := range inputs {
		if input.configurationPriceDimensionNo != "" || len(input.tiers) != 1 || input.tiers[0].MinimumQuantity != 0 || input.tiers[0].MaximumQuantity != nil {
			return bizerr.NewCode(CodeManagementInvalid)
		}
		amount := input.tiers[0].AmountMinor
		raw, err := json.Marshal(channelcommercecap.ImportWholeItemRule{Kind: "fixed", FixedAmountMinor: &amount})
		if err != nil {
			return err
		}
		values = append(values, map[string]any{"billing_rule_no": input.billingRuleNo, "currency_code": input.currencyCode, "whole_item_rule": json.RawMessage(raw)})
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return err
	}
	result, err := dao.CatalogImportTerm.DB().Exec(ctx, `WITH desired AS (SELECT item.billing_rule_no,item.currency_code,item.whole_item_rule FROM jsonb_to_recordset(CAST(? AS jsonb)) AS item(billing_rule_no TEXT,currency_code TEXT,whole_item_rule JSONB)) UPDATE plugin_idc_core_catalog_import_term term SET whole_item_rule=desired.whole_item_rule FROM desired WHERE term.tenant_id=0 AND term.version_no=? AND term.billing_rule_no=desired.billing_rule_no AND term.currency_code=desired.currency_code`, string(raw), version)
	return requireNativeWholeItemUpdates(result, err, len(values))
}
func requireNativeWholeItemUpdates(result sql.Result, err error, count int) error {
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != int64(count) {
		return bizerr.NewCode(CodeManagementConflict)
	}
	return nil
}
