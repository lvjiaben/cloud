// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CommerceCartItem is the golang structure for table commerce_cart_item.
type CommerceCartItem struct {
	Id                      int64      `json:"id"                      orm:"id"                        description:""`
	TenantId                int        `json:"tenantId"                orm:"tenant_id"                 description:""`
	CartItemNo              string     `json:"cartItemNo"              orm:"cart_item_no"              description:""`
	CartNo                  string     `json:"cartNo"                  orm:"cart_no"                   description:""`
	ProductNo               string     `json:"productNo"               orm:"product_no"                description:""`
	VersionNo               string     `json:"versionNo"               orm:"version_no"                description:""`
	BillingRuleNo           string     `json:"billingRuleNo"           orm:"billing_rule_no"           description:""`
	ConfigurationSnapshotNo string     `json:"configurationSnapshotNo" orm:"configuration_snapshot_no" description:""`
	PurchaseCycles          int        `json:"purchaseCycles"          orm:"purchase_cycles"           description:""`
	StatusCode              string     `json:"statusCode"              orm:"status_code"               description:""`
	IssueCode               string     `json:"issueCode"               orm:"issue_code"                description:""`
	SortOrder               int        `json:"sortOrder"               orm:"sort_order"                description:""`
	Version                 int64      `json:"version"                 orm:"version"                   description:""`
	RemovedAt               *time.Time `json:"removedAt"               orm:"removed_at"                description:""`
	CreatedAt               *time.Time `json:"createdAt"               orm:"created_at"                description:""`
	UpdatedAt               *time.Time `json:"updatedAt"               orm:"updated_at"                description:""`
}
