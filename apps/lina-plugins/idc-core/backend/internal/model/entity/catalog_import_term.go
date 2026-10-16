// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CatalogImportTerm is the golang structure for table catalog_import_term.
type CatalogImportTerm struct {
	Id                    int64      `json:"id"                    orm:"id"                      description:""`
	TenantId              int        `json:"tenantId"              orm:"tenant_id"               description:""`
	VersionNo             string     `json:"versionNo"             orm:"version_no"              description:""`
	BillingRuleNo         string     `json:"billingRuleNo"         orm:"billing_rule_no"         description:""`
	CurrencyCode          string     `json:"currencyCode"          orm:"currency_code"           description:""`
	UpstreamTermReference string     `json:"upstreamTermReference" orm:"upstream_term_reference" description:""`
	WholeItemRule         string     `json:"wholeItemRule"         orm:"whole_item_rule"         description:""`
	MarkupPercent         string     `json:"markupPercent"         orm:"markup_percent"          description:""`
	CreatedAt             *time.Time `json:"createdAt"             orm:"created_at"              description:""`
}
