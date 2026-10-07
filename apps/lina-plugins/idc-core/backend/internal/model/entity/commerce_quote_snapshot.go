// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CommerceQuoteSnapshot is the golang structure for table commerce_quote_snapshot.
type CommerceQuoteSnapshot struct {
	Id                     int64      `json:"id"                     orm:"id"                       description:""`
	TenantId               int        `json:"tenantId"               orm:"tenant_id"                description:""`
	QuoteNo                string     `json:"quoteNo"                orm:"quote_no"                 description:""`
	CartNo                 *string    `json:"cartNo"                 orm:"cart_no"                  description:""`
	CustomerNo             *string    `json:"customerNo"             orm:"customer_no"              description:""`
	CustomerSubjectVersion *int64     `json:"customerSubjectVersion" orm:"customer_subject_version" description:""`
	PurposeCode            string     `json:"purposeCode"            orm:"purpose_code"             description:""`
	SourceCode             string     `json:"sourceCode"             orm:"source_code"              description:""`
	CurrencyCode           string     `json:"currencyCode"           orm:"currency_code"            description:""`
	ItemCount              int        `json:"itemCount"              orm:"item_count"               description:""`
	SubtotalMinor          int64      `json:"subtotalMinor"          orm:"subtotal_minor"           description:""`
	DiscountMinor          int64      `json:"discountMinor"          orm:"discount_minor"           description:""`
	NetAmountMinor         int64      `json:"netAmountMinor"         orm:"net_amount_minor"         description:""`
	TaxAmountMinor         int64      `json:"taxAmountMinor"         orm:"tax_amount_minor"         description:""`
	TotalAmountMinor       int64      `json:"totalAmountMinor"       orm:"total_amount_minor"       description:""`
	InputDigest            string     `json:"inputDigest"            orm:"input_digest"             description:""`
	PricingRuleDigest      string     `json:"pricingRuleDigest"      orm:"pricing_rule_digest"      description:""`
	EligibilityDigest      string     `json:"eligibilityDigest"      orm:"eligibility_digest"       description:""`
	InventoryDigest        string     `json:"inventoryDigest"        orm:"inventory_digest"         description:""`
	ContentDigest          string     `json:"contentDigest"          orm:"content_digest"           description:""`
	IssuedAt               *time.Time `json:"issuedAt"               orm:"issued_at"                description:""`
	ExpiresAt              *time.Time `json:"expiresAt"              orm:"expires_at"               description:""`
	CreatedBy              int        `json:"createdBy"              orm:"created_by"               description:""`
}
