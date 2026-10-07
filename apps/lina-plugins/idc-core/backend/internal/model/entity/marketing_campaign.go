// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// MarketingCampaign is the golang structure for table marketing_campaign.
type MarketingCampaign struct {
	Id                    int64      `json:"id"                    orm:"id"                       description:""`
	TenantId              int        `json:"tenantId"              orm:"tenant_id"                description:""`
	CampaignNo            string     `json:"campaignNo"            orm:"campaign_no"              description:""`
	CampaignCode          string     `json:"campaignCode"          orm:"campaign_code"            description:""`
	NameZhCn              string     `json:"nameZhCn"              orm:"name_zh_cn"               description:""`
	NameEnUs              string     `json:"nameEnUs"              orm:"name_en_us"               description:""`
	DescriptionZhCn       string     `json:"descriptionZhCn"       orm:"description_zh_cn"        description:""`
	DescriptionEnUs       string     `json:"descriptionEnUs"       orm:"description_en_us"        description:""`
	CampaignTypeCode      string     `json:"campaignTypeCode"      orm:"campaign_type_code"       description:""`
	TargetSegmentNo       *string    `json:"targetSegmentNo"       orm:"target_segment_no"        description:""`
	TargetSnapshotNo      *string    `json:"targetSnapshotNo"      orm:"target_snapshot_no"       description:""`
	StartsAt              *time.Time `json:"startsAt"              orm:"starts_at"                description:""`
	EndsAt                *time.Time `json:"endsAt"                orm:"ends_at"                  description:""`
	Priority              int        `json:"priority"              orm:"priority"                 description:""`
	StackPolicyCode       string     `json:"stackPolicyCode"       orm:"stack_policy_code"        description:""`
	StackGroupCode        string     `json:"stackGroupCode"        orm:"stack_group_code"         description:""`
	GlobalQuotaLimit      *int64     `json:"globalQuotaLimit"      orm:"global_quota_limit"       description:""`
	GlobalReservedCount   int64      `json:"globalReservedCount"   orm:"global_reserved_count"    description:""`
	GlobalConsumedCount   int64      `json:"globalConsumedCount"   orm:"global_consumed_count"    description:""`
	PerCustomerQuotaLimit *int       `json:"perCustomerQuotaLimit" orm:"per_customer_quota_limit" description:""`
	StatusCode            string     `json:"statusCode"            orm:"status_code"              description:""`
	Version               int64      `json:"version"               orm:"version"                  description:""`
	CreatedBy             int        `json:"createdBy"             orm:"created_by"               description:""`
	UpdatedBy             int        `json:"updatedBy"             orm:"updated_by"               description:""`
	CreatedAt             *time.Time `json:"createdAt"             orm:"created_at"               description:""`
	UpdatedAt             *time.Time `json:"updatedAt"             orm:"updated_at"               description:""`
	IsNewcomerExclusive   bool       `json:"isNewcomerExclusive"   orm:"is_newcomer_exclusive"    description:""`
}
