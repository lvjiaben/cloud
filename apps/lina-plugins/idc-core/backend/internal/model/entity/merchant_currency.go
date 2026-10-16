// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// MerchantCurrency is the golang structure for table merchant_currency.
type MerchantCurrency struct {
	TenantId     int        `json:"tenantId"     orm:"tenant_id"     description:""`
	CurrencyCode string     `json:"currencyCode" orm:"currency_code" description:"ISO 4217 currency code"`
	Enabled      bool       `json:"enabled"      orm:"enabled"       description:"Whether new quotes may use this currency"`
	Version      int64      `json:"version"      orm:"version"       description:"Optimistic concurrency version"`
	CreatedBy    int        `json:"createdBy"    orm:"created_by"    description:""`
	UpdatedBy    int        `json:"updatedBy"    orm:"updated_by"    description:""`
	CreatedAt    *time.Time `json:"createdAt"    orm:"created_at"    description:""`
	UpdatedAt    *time.Time `json:"updatedAt"    orm:"updated_at"    description:""`
	Deleted      bool       `json:"deleted"      orm:"deleted"       description:"Whether the configuration is removed; retain version and audit continuity"`
}
