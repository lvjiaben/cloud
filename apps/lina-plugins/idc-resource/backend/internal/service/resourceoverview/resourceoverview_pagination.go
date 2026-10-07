// This file keeps overview counts and page queries on identical local fact sets.

package resourceoverview

import "lina-core/pkg/bizerr"

const poolDirectorySQL = `FROM plugin_idc_resource_pool pool
JOIN plugin_idc_resource_channel channel ON channel.tenant_id=pool.tenant_id AND channel.channel_no=pool.channel_no
WHERE pool.tenant_id=? AND pool.deleted_at IS NULL`

const financeDirectorySQL = `FROM plugin_idc_resource_financial_snapshot snapshot
JOIN plugin_idc_resource_channel channel ON channel.tenant_id=snapshot.tenant_id AND channel.channel_no=snapshot.channel_no
WHERE snapshot.tenant_id=? AND snapshot.id IN (
 SELECT max(latest.id) FROM plugin_idc_resource_financial_snapshot latest
 WHERE latest.tenant_id=? GROUP BY latest.channel_no,latest.snapshot_kind,latest.external_resource_ref
)`

// normalizeList validates independent numeric pages without truncating either table's total.
func normalizeList(input ListInput) (ListInput, error) {
	for _, page := range []*int{&input.PoolPage, &input.FinancePage} {
		if *page == 0 {
			*page = 1
		}
		if *page < 1 || *page > 1_000_000 {
			return ListInput{}, bizerr.NewCode(CodeInvalid)
		}
	}
	for _, size := range []*int{&input.PoolPageSize, &input.FinancePageSize} {
		if *size == 0 {
			*size = 10
		}
		switch *size {
		case 10, 20, 50, 100:
		default:
			return ListInput{}, bizerr.NewCode(CodeInvalid)
		}
	}
	return input, nil
}
