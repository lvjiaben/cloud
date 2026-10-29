package serveroperation

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/internal/service/catalog"
)

// PrepareChannelResize reuses the server-domain shrink guard and the exact
// Provider parameter whitelist used by the customer portal. Channel callers
// never get a second, weaker resize interpretation.
func PrepareChannelResize(current, target []catalog.PurchaseConfigurationItem) (json.RawMessage, error) {
	if resizeDiskShrink(current, target) {
		return nil, ErrActionUnavailable
	}
	return resizeProviderParameters(current, target)
}

// ProrateChannelResize reuses the integer, round-up proration rule.
func ProrateChannelResize(deltaMinor int64, periodStart, periodEnd, now time.Time) (int64, error) {
	return proratedUpgradeAmount(deltaMinor, periodStart, periodEnd, now)
}

// PersistChannelResizeConfiguration freezes the exact target configuration
// selected by the authoritative catalog quote.
func PersistChannelResizeConfiguration(ctx context.Context, database gdb.DB,
	resolved *catalog.ServerResizeConfiguration, snapshotNo string) error {
	if resolved == nil || database == nil || snapshotNo == "" {
		return ErrInvalidInput
	}
	type dimensionRow struct{ DimensionNo, FieldKey, CandidateValue string }
	var dimensions []dimensionRow
	if err := database.GetScan(ctx, &dimensions, `SELECT dimension_no,field_key,candidate_value
FROM plugin_idc_core_catalog_configuration_price_dimension
WHERE tenant_id=0 AND version_no=? AND status_code='active'
ORDER BY field_key,candidate_value,id`, resolved.VersionNo); err != nil {
		return err
	}
	dimensionByValue := make(map[string]string, len(dimensions))
	for _, dimension := range dimensions {
		dimensionByValue[dimension.FieldKey+"\x00"+dimension.CandidateValue] = dimension.DimensionNo
	}
	selections := make([]catalog.ConfigurationPriceSelection, 0, len(resolved.Items))
	seen := make(map[string]struct{}, len(resolved.Items))
	var total int64
	for _, item := range resolved.Items {
		total += int64(item.Quantity)
		dimensionNo := dimensionByValue[item.GroupCode+"\x00"+item.OptionCode]
		if dimensionNo == "" {
			continue
		}
		if _, exists := seen[dimensionNo]; exists {
			return ErrInvalidInput
		}
		seen[dimensionNo] = struct{}{}
		selections = append(selections, catalog.ConfigurationPriceSelection{
			DimensionNo: dimensionNo, Quantity: item.Quantity,
		})
	}
	sort.Slice(selections, func(i, j int) bool { return selections[i].DimensionNo < selections[j].DimensionNo })
	selectionDigest, err := catalog.ConfigurationPriceSelectionDigest(selections)
	if err != nil {
		return err
	}
	contentDigest, err := digestValue(resolved.Items)
	if err != nil {
		return err
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_commerce_configuration_snapshot
(tenant_id,configuration_snapshot_no,product_no,version_no,source_revision_digest,
selection_digest,selected_option_count,total_selected_quantity,content_digest)
VALUES(0,?,?,?,?,?,?,?,?)`, snapshotNo, resolved.ProductNo, resolved.VersionNo,
		resolved.SourceRevisionDigest, selectionDigest, len(resolved.Items), total, contentDigest); err != nil {
		return err
	}
	for index, item := range resolved.Items {
		itemNo, _ := deterministicReference("rsi_", fmt.Sprintf("%s:%d", snapshotNo, index))
		if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_commerce_configuration_snapshot_item
(tenant_id,snapshot_item_no,configuration_snapshot_no,version_no,group_no,option_no,group_code,option_code,
component_type_code,group_name_zh_cn,group_name_en_us,option_name_zh_cn,option_name_en_us,
normalized_value,quantity,sort_order)
VALUES(0,?,?,?,?,?,?,?,?,?,?,?,?,?::JSONB,?,?)`, itemNo, snapshotNo, resolved.VersionNo, item.GroupNo,
			item.OptionNo, item.GroupCode, item.OptionCode, item.ComponentTypeCode,
			item.GroupNameZhCN, item.GroupNameEnUS, item.OptionNameZhCN, item.OptionNameEnUS,
			item.NormalizedValue, item.Quantity, item.SortOrder); err != nil {
			return err
		}
	}
	for _, selection := range selections {
		if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_commerce_configuration_price_selection
(tenant_id,configuration_snapshot_no,version_no,dimension_no,quantity)
VALUES(0,?,?,?,?)`, snapshotNo, resolved.VersionNo, selection.DimensionNo,
			selection.Quantity); err != nil {
			return err
		}
	}
	return nil
}
