// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CatalogProductVersionCost is the golang structure for table catalog_product_version_cost.
type CatalogProductVersionCost struct {
	Id             int64      `json:"id"             orm:"id"               description:""`
	TenantId       int        `json:"tenantId"       orm:"tenant_id"        description:""`
	CostNo         string     `json:"costNo"         orm:"cost_no"          description:""`
	VersionNo      string     `json:"versionNo"      orm:"version_no"       description:""`
	CurrencyCode   string     `json:"currencyCode"   orm:"currency_code"    description:""`
	PeriodUnitCode string     `json:"periodUnitCode" orm:"period_unit_code" description:""`
	PeriodCount    int        `json:"periodCount"    orm:"period_count"     description:""`
	UnitCostMinor  int64      `json:"unitCostMinor"  orm:"unit_cost_minor"  description:""`
	EffectiveFrom  *time.Time `json:"effectiveFrom"  orm:"effective_from"   description:""`
	EffectiveUntil *time.Time `json:"effectiveUntil" orm:"effective_until"  description:""`
	Version        int64      `json:"version"        orm:"version"          description:""`
	CreatedAt      *time.Time `json:"createdAt"      orm:"created_at"       description:""`
	UpdatedAt      *time.Time `json:"updatedAt"      orm:"updated_at"       description:""`
}
