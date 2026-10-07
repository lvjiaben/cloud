// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesCommissionTier is the golang structure for table sales_commission_tier.
type SalesCommissionTier struct {
	Id              int64      `json:"id"              orm:"id"                description:""`
	TenantId        int        `json:"tenantId"        orm:"tenant_id"         description:""`
	TierNo          string     `json:"tierNo"          orm:"tier_no"           description:""`
	RuleNo          string     `json:"ruleNo"          orm:"rule_no"           description:""`
	TierOrder       int        `json:"tierOrder"       orm:"tier_order"        description:""`
	LowerBoundMinor int64      `json:"lowerBoundMinor" orm:"lower_bound_minor" description:""`
	UpperBoundMinor *int64     `json:"upperBoundMinor" orm:"upper_bound_minor" description:""`
	RateBasisPoints int        `json:"rateBasisPoints" orm:"rate_basis_points" description:""`
	CreatedAt       *time.Time `json:"createdAt"       orm:"created_at"        description:""`
}
