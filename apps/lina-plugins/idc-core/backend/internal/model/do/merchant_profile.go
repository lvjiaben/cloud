// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// MerchantProfile is the golang structure of table plugin_idc_core_merchant_profile for DAO operations like Where/Data.
type MerchantProfile struct {
	g.Meta          `orm:"table:plugin_idc_core_merchant_profile, do:true"`
	Id              any        //
	TenantId        any        // Host tenant context; the single-merchant edition requires tenant 0
	MerchantNo      any        // Opaque public merchant business number
	DisplayName     any        // Merchant display name
	LegalName       any        // Merchant legal entity name
	CountryCode     any        // ISO 3166-1 alpha-2 country code
	DefaultLocale   any        // Default IETF locale tag
	DefaultCurrency any        // Default ISO 4217 currency code
	Timezone        any        // Default IANA time zone
	Enabled         any        // Whether the merchant operating context accepts normal business traffic
	Version         any        // Optimistic concurrency version
	CreatedBy       any        //
	UpdatedBy       any        //
	CreatedAt       *time.Time //
	UpdatedAt       *time.Time //
}
