// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CommerceCart is the golang structure for table commerce_cart.
type CommerceCart struct {
	Id               int64      `json:"id"               orm:"id"                  description:""`
	TenantId         int        `json:"tenantId"         orm:"tenant_id"           description:""`
	CartNo           string     `json:"cartNo"           orm:"cart_no"             description:""`
	CustomerNo       *string    `json:"customerNo"       orm:"customer_no"         description:""`
	GuestOwnerDigest *string    `json:"guestOwnerDigest" orm:"guest_owner_digest"  description:""`
	CurrencyCode     string     `json:"currencyCode"     orm:"currency_code"       description:""`
	StatusCode       string     `json:"statusCode"       orm:"status_code"         description:""`
	LatestQuoteNo    *string    `json:"latestQuoteNo"    orm:"latest_quote_no"     description:""`
	MergedIntoCartNo *string    `json:"mergedIntoCartNo" orm:"merged_into_cart_no" description:""`
	ExpiresAt        *time.Time `json:"expiresAt"        orm:"expires_at"          description:""`
	ClosedAt         *time.Time `json:"closedAt"         orm:"closed_at"           description:""`
	Version          int64      `json:"version"          orm:"version"             description:""`
	CreatedAt        *time.Time `json:"createdAt"        orm:"created_at"          description:""`
	UpdatedAt        *time.Time `json:"updatedAt"        orm:"updated_at"          description:""`
}
