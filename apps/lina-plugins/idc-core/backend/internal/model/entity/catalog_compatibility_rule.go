// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CatalogCompatibilityRule is the golang structure for table catalog_compatibility_rule.
type CatalogCompatibilityRule struct {
	Id                    int64      `json:"id"                    orm:"id"                      description:""`
	TenantId              int        `json:"tenantId"              orm:"tenant_id"               description:""`
	RuleNo                string     `json:"ruleNo"                orm:"rule_no"                 description:""`
	VersionNo             string     `json:"versionNo"             orm:"version_no"              description:""`
	RuleTypeCode          string     `json:"ruleTypeCode"          orm:"rule_type_code"          description:""`
	SourceOptionNo        string     `json:"sourceOptionNo"        orm:"source_option_no"        description:""`
	TargetOptionNo        string     `json:"targetOptionNo"        orm:"target_option_no"        description:""`
	MinimumTargetQuantity int        `json:"minimumTargetQuantity" orm:"minimum_target_quantity" description:""`
	MaximumTargetQuantity *int       `json:"maximumTargetQuantity" orm:"maximum_target_quantity" description:""`
	StatusCode            string     `json:"statusCode"            orm:"status_code"             description:""`
	CreatedAt             *time.Time `json:"createdAt"             orm:"created_at"              description:""`
}
