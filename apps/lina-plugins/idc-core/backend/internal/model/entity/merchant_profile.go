// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// MerchantProfile is the golang structure for table merchant_profile.
type MerchantProfile struct {
	Id              int64      `json:"id"              orm:"id"               description:""`
	TenantId        int        `json:"tenantId"        orm:"tenant_id"        description:"Host tenant context; the single-merchant edition requires tenant 0"`
	MerchantNo      string     `json:"merchantNo"      orm:"merchant_no"      description:"Opaque public merchant business number"`
	DisplayName     string     `json:"displayName"     orm:"display_name"     description:"Merchant display name"`
	LegalName       string     `json:"legalName"       orm:"legal_name"       description:"Merchant legal entity name"`
	CountryCode     string     `json:"countryCode"     orm:"country_code"     description:"ISO 3166-1 alpha-2 country code"`
	DefaultLocale   string     `json:"defaultLocale"   orm:"default_locale"   description:"Default IETF locale tag"`
	DefaultCurrency string     `json:"defaultCurrency" orm:"default_currency" description:"Default ISO 4217 currency code"`
	Timezone        string     `json:"timezone"        orm:"timezone"         description:"Default IANA time zone"`
	Enabled         bool       `json:"enabled"         orm:"enabled"          description:"Whether the merchant operating context accepts normal business traffic"`
	Version         int64      `json:"version"         orm:"version"          description:"Optimistic concurrency version"`
	CreatedBy       int        `json:"createdBy"       orm:"created_by"       description:""`
	UpdatedBy       int        `json:"updatedBy"       orm:"updated_by"       description:""`
	CreatedAt       *time.Time `json:"createdAt"       orm:"created_at"       description:""`
	UpdatedAt       *time.Time `json:"updatedAt"       orm:"updated_at"       description:""`
}
