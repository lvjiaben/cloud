// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesAgentProductRule is the golang structure for table sales_agent_product_rule.
type SalesAgentProductRule struct {
	Id            int64      `json:"id"            orm:"id"              description:""`
	TenantId      int        `json:"tenantId"      orm:"tenant_id"       description:""`
	ProductRuleNo string     `json:"productRuleNo" orm:"product_rule_no" description:""`
	LevelNo       string     `json:"levelNo"       orm:"level_no"        description:""`
	ProductNo     string     `json:"productNo"     orm:"product_no"      description:""`
	StatusCode    string     `json:"statusCode"    orm:"status_code"     description:""`
	EffectiveAt   *time.Time `json:"effectiveAt"   orm:"effective_at"    description:""`
	ExpiresAt     *time.Time `json:"expiresAt"     orm:"expires_at"      description:""`
	Version       int64      `json:"version"       orm:"version"         description:""`
	RuleDigest    string     `json:"ruleDigest"    orm:"rule_digest"     description:""`
	CreatedAt     *time.Time `json:"createdAt"     orm:"created_at"      description:""`
}
