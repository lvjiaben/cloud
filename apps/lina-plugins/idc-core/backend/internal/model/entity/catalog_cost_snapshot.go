// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CatalogCostSnapshot is the golang structure for table catalog_cost_snapshot.
type CatalogCostSnapshot struct {
	Id               int64      `json:"id"               orm:"id"                 description:""`
	TenantId         int        `json:"tenantId"         orm:"tenant_id"          description:""`
	CostSnapshotNo   string     `json:"costSnapshotNo"   orm:"cost_snapshot_no"   description:""`
	BindingNo        string     `json:"bindingNo"        orm:"binding_no"         description:""`
	SourceCode       string     `json:"sourceCode"       orm:"source_code"        description:""`
	SourceSnapshotNo string     `json:"sourceSnapshotNo" orm:"source_snapshot_no" description:""`
	CurrencyCode     string     `json:"currencyCode"     orm:"currency_code"      description:""`
	AmountMinor      int64      `json:"amountMinor"      orm:"amount_minor"       description:""`
	PeriodUnitCode   string     `json:"periodUnitCode"   orm:"period_unit_code"   description:""`
	PeriodCount      int        `json:"periodCount"      orm:"period_count"       description:""`
	ObservedAt       *time.Time `json:"observedAt"       orm:"observed_at"        description:""`
	ValidUntil       *time.Time `json:"validUntil"       orm:"valid_until"        description:""`
	CreatedBy        int        `json:"createdBy"        orm:"created_by"         description:""`
	CreatedAt        *time.Time `json:"createdAt"        orm:"created_at"         description:""`
}
