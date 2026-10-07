// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CustomerContact is the golang structure for table customer_contact.
type CustomerContact struct {
	Id                int64      `json:"id"                orm:"id"                  description:""`
	TenantId          int        `json:"tenantId"          orm:"tenant_id"           description:""`
	ContactNo         string     `json:"contactNo"         orm:"contact_no"          description:""`
	CustomerNo        string     `json:"customerNo"        orm:"customer_no"         description:""`
	DisplayName       string     `json:"displayName"       orm:"display_name"        description:""`
	Email             string     `json:"email"             orm:"email"               description:""`
	EmailLookupDigest string     `json:"emailLookupDigest" orm:"email_lookup_digest" description:"Keyed normalized email lookup digest"`
	EmailVerifiedAt   *time.Time `json:"emailVerifiedAt"   orm:"email_verified_at"   description:""`
	PhoneCountryCode  string     `json:"phoneCountryCode"  orm:"phone_country_code"  description:""`
	Phone             string     `json:"phone"             orm:"phone"               description:""`
	PhoneLookupDigest string     `json:"phoneLookupDigest" orm:"phone_lookup_digest" description:"Keyed E.164-style phone lookup digest"`
	PhoneVerifiedAt   *time.Time `json:"phoneVerifiedAt"   orm:"phone_verified_at"   description:""`
	PreferredLocale   string     `json:"preferredLocale"   orm:"preferred_locale"    description:""`
	StatusCode        string     `json:"statusCode"        orm:"status_code"         description:""`
	Version           int64      `json:"version"           orm:"version"             description:""`
	CreatedBy         int        `json:"createdBy"         orm:"created_by"          description:""`
	UpdatedBy         int        `json:"updatedBy"         orm:"updated_by"          description:""`
	CreatedAt         *time.Time `json:"createdAt"         orm:"created_at"          description:""`
	UpdatedAt         *time.Time `json:"updatedAt"         orm:"updated_at"          description:""`
}
