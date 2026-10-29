// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// NativeServerTargetQuote is the golang structure for table native_server_target_quote.
type NativeServerTargetQuote struct {
	TenantId                   int        `json:"tenantId"                   orm:"tenant_id"                    description:""`
	Id                         int64      `json:"id"                         orm:"id"                           description:""`
	TargetQuoteNo              string     `json:"targetQuoteNo"              orm:"target_quote_no"              description:""`
	QuoteItemNo                string     `json:"quoteItemNo"                orm:"quote_item_no"                description:""`
	StructuredConfigurationNo  string     `json:"structuredConfigurationNo"  orm:"structured_configuration_no"  description:""`
	EntitlementNo              string     `json:"entitlementNo"              orm:"entitlement_no"               description:""`
	CustomerNo                 string     `json:"customerNo"                 orm:"customer_no"                  description:""`
	ExpectedEntitlementVersion int64      `json:"expectedEntitlementVersion" orm:"expected_entitlement_version" description:""`
	CustomerSubjectVersion     int64      `json:"customerSubjectVersion"     orm:"customer_subject_version"     description:""`
	CurrencyCode               string     `json:"currencyCode"               orm:"currency_code"                description:""`
	NativeTargetSnapshot       string     `json:"nativeTargetSnapshot"       orm:"native_target_snapshot"       description:""`
	CreatedAt                  *time.Time `json:"createdAt"                  orm:"created_at"                   description:""`
}
