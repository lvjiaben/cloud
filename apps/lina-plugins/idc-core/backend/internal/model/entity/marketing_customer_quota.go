// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// MarketingCustomerQuota is the golang structure for table marketing_customer_quota.
type MarketingCustomerQuota struct {
	Id            int64      `json:"id"            orm:"id"             description:""`
	TenantId      int        `json:"tenantId"      orm:"tenant_id"      description:""`
	CampaignNo    string     `json:"campaignNo"    orm:"campaign_no"    description:""`
	CustomerNo    string     `json:"customerNo"    orm:"customer_no"    description:""`
	QuotaLimit    int        `json:"quotaLimit"    orm:"quota_limit"    description:""`
	ReservedCount int        `json:"reservedCount" orm:"reserved_count" description:""`
	ConsumedCount int        `json:"consumedCount" orm:"consumed_count" description:""`
	Version       int64      `json:"version"       orm:"version"        description:""`
	CreatedAt     *time.Time `json:"createdAt"     orm:"created_at"     description:""`
	UpdatedAt     *time.Time `json:"updatedAt"     orm:"updated_at"     description:""`
}
