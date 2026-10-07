// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CustomerContact is the golang structure of table plugin_idc_core_customer_contact for DAO operations like Where/Data.
type CustomerContact struct {
	g.Meta            `orm:"table:plugin_idc_core_customer_contact, do:true"`
	Id                any        //
	TenantId          any        //
	ContactNo         any        //
	CustomerNo        any        //
	DisplayName       any        //
	Email             any        //
	EmailLookupDigest any        // Keyed normalized email lookup digest
	EmailVerifiedAt   *time.Time //
	PhoneCountryCode  any        //
	Phone             any        //
	PhoneLookupDigest any        // Keyed E.164-style phone lookup digest
	PhoneVerifiedAt   *time.Time //
	PreferredLocale   any        //
	StatusCode        any        //
	Version           any        //
	CreatedBy         any        //
	UpdatedBy         any        //
	CreatedAt         *time.Time //
	UpdatedAt         *time.Time //
}
