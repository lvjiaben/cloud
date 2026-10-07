// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CommerceAssistedQuoteRequestItem is the golang structure for table commerce_assisted_quote_request_item.
type CommerceAssistedQuoteRequestItem struct {
	Id                      int64      `json:"id"                      orm:"id"                        description:""`
	TenantId                int        `json:"tenantId"                orm:"tenant_id"                 description:""`
	RequestItemNo           string     `json:"requestItemNo"           orm:"request_item_no"           description:""`
	RequestNo               string     `json:"requestNo"               orm:"request_no"                description:""`
	CurrencyCode            string     `json:"currencyCode"            orm:"currency_code"             description:""`
	LineNumber              int        `json:"lineNumber"              orm:"line_number"               description:""`
	ProductNo               string     `json:"productNo"               orm:"product_no"                description:""`
	VersionNo               string     `json:"versionNo"               orm:"version_no"                description:""`
	BillingRuleNo           string     `json:"billingRuleNo"           orm:"billing_rule_no"           description:""`
	ConfigurationSnapshotNo string     `json:"configurationSnapshotNo" orm:"configuration_snapshot_no" description:""`
	ProductNameZhCn         string     `json:"productNameZhCn"         orm:"product_name_zh_cn"        description:""`
	ProductNameEnUs         string     `json:"productNameEnUs"         orm:"product_name_en_us"        description:""`
	PurchaseCycles          int        `json:"purchaseCycles"          orm:"purchase_cycles"           description:""`
	ContentDigest           string     `json:"contentDigest"           orm:"content_digest"            description:""`
	CreatedAt               *time.Time `json:"createdAt"               orm:"created_at"                description:""`
}
