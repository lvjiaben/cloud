// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// MarketingPointsAccount is the golang structure for table marketing_points_account.
type MarketingPointsAccount struct {
	Id              int64      `json:"id"              orm:"id"               description:""`
	TenantId        int        `json:"tenantId"        orm:"tenant_id"        description:""`
	CustomerNo      string     `json:"customerNo"      orm:"customer_no"      description:""`
	AvailablePoints int64      `json:"availablePoints" orm:"available_points" description:""`
	FrozenPoints    int64      `json:"frozenPoints"    orm:"frozen_points"    description:""`
	DebtPoints      int64      `json:"debtPoints"      orm:"debt_points"      description:""`
	Version         int64      `json:"version"         orm:"version"          description:""`
	CreatedAt       *time.Time `json:"createdAt"       orm:"created_at"       description:""`
	UpdatedAt       *time.Time `json:"updatedAt"       orm:"updated_at"       description:""`
}
