// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ServerAccessoryPricePolicy is the golang structure for table server_accessory_price_policy.
type ServerAccessoryPricePolicy struct {
	Id             int64      `json:"id"             orm:"id"              description:""`
	TenantId       int        `json:"tenantId"       orm:"tenant_id"       description:""`
	PolicyNo       string     `json:"policyNo"       orm:"policy_no"       description:""`
	AccessoryKind  string     `json:"accessoryKind"  orm:"accessory_kind"  description:""`
	CurrencyCode   string     `json:"currencyCode"   orm:"currency_code"   description:""`
	CurrentVersion int64      `json:"currentVersion" orm:"current_version" description:""`
	CreatedAt      *time.Time `json:"createdAt"      orm:"created_at"      description:""`
	UpdatedAt      *time.Time `json:"updatedAt"      orm:"updated_at"      description:""`
	Enabled        bool       `json:"enabled"        orm:"enabled"         description:""`
	StockQuantity  *int64     `json:"stockQuantity"  orm:"stock_quantity"  description:""`
}
