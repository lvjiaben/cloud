// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ServiceRenewalQuoteRequest is the golang structure for table service_renewal_quote_request.
type ServiceRenewalQuoteRequest struct {
	Id                         int64      `json:"id"                         orm:"id"                           description:""`
	TenantId                   int        `json:"tenantId"                   orm:"tenant_id"                    description:""`
	RequestNo                  string     `json:"requestNo"                  orm:"request_no"                   description:""`
	CustomerNo                 string     `json:"customerNo"                 orm:"customer_no"                  description:""`
	EntitlementNo              string     `json:"entitlementNo"              orm:"entitlement_no"               description:""`
	CurrencyCode               string     `json:"currencyCode"               orm:"currency_code"                description:""`
	RequestSourceCode          string     `json:"requestSourceCode"          orm:"request_source_code"          description:""`
	RenewalCycles              int        `json:"renewalCycles"              orm:"renewal_cycles"               description:""`
	IdempotencyDigest          string     `json:"idempotencyDigest"          orm:"idempotency_digest"           description:""`
	RequestDigest              string     `json:"requestDigest"              orm:"request_digest"               description:""`
	QuoteNo                    string     `json:"quoteNo"                    orm:"quote_no"                     description:""`
	PriceSnapshotNo            string     `json:"priceSnapshotNo"            orm:"price_snapshot_no"            description:""`
	ExpectedEntitlementVersion int64      `json:"expectedEntitlementVersion" orm:"expected_entitlement_version" description:""`
	ContentDigest              string     `json:"contentDigest"              orm:"content_digest"               description:""`
	CreatedAt                  *time.Time `json:"createdAt"                  orm:"created_at"                   description:""`
}
