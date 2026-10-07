// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// TrialEntitlement is the golang structure for table trial_entitlement.
type TrialEntitlement struct {
	Id                     int64      `json:"id"                     orm:"id"                       description:""`
	TenantId               int        `json:"tenantId"               orm:"tenant_id"                description:""`
	EntitlementNo          string     `json:"entitlementNo"          orm:"entitlement_no"           description:""`
	ScopeNo                string     `json:"scopeNo"                orm:"scope_no"                 description:""`
	CustomerNo             string     `json:"customerNo"             orm:"customer_no"              description:""`
	ProductNo              string     `json:"productNo"              orm:"product_no"               description:""`
	VersionNo              string     `json:"versionNo"              orm:"version_no"               description:""`
	BillingRuleNo          string     `json:"billingRuleNo"          orm:"billing_rule_no"          description:""`
	OrderNo                string     `json:"orderNo"                orm:"order_no"                 description:""`
	InvoiceNo              string     `json:"invoiceNo"              orm:"invoice_no"               description:""`
	OrderItemNo            string     `json:"orderItemNo"            orm:"order_item_no"            description:""`
	CurrencyCode           string     `json:"currencyCode"           orm:"currency_code"            description:""`
	TrialLimitQuantity     int        `json:"trialLimitQuantity"     orm:"trial_limit_quantity"     description:""`
	StatusCode             string     `json:"statusCode"             orm:"status_code"              description:""`
	HoldExpiresAt          *time.Time `json:"holdExpiresAt"          orm:"hold_expires_at"          description:""`
	ConsumedAt             *time.Time `json:"consumedAt"             orm:"consumed_at"              description:""`
	ReleasedAt             *time.Time `json:"releasedAt"             orm:"released_at"              description:""`
	IdentityEvidenceDigest *string    `json:"identityEvidenceDigest" orm:"identity_evidence_digest" description:""`
	ContentDigest          string     `json:"contentDigest"          orm:"content_digest"           description:""`
	Version                int64      `json:"version"                orm:"version"                  description:""`
	CreatedAt              *time.Time `json:"createdAt"              orm:"created_at"               description:""`
	UpdatedAt              *time.Time `json:"updatedAt"              orm:"updated_at"               description:""`
	UnitSequence           int        `json:"unitSequence"           orm:"unit_sequence"            description:"Generic one-based quantity unit inside the immutable order item; it has no server meaning."`
}
