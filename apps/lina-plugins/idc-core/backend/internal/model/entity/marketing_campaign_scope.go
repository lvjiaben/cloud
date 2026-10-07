// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// MarketingCampaignScope is the golang structure for table marketing_campaign_scope.
type MarketingCampaignScope struct {
	Id            int64      `json:"id"            orm:"id"              description:""`
	TenantId      int        `json:"tenantId"      orm:"tenant_id"       description:""`
	CampaignNo    string     `json:"campaignNo"    orm:"campaign_no"     description:""`
	ScopeKindCode string     `json:"scopeKindCode" orm:"scope_kind_code" description:""`
	MatchModeCode string     `json:"matchModeCode" orm:"match_mode_code" description:""`
	ReferenceCode string     `json:"referenceCode" orm:"reference_code"  description:""`
	CreatedAt     *time.Time `json:"createdAt"     orm:"created_at"      description:""`
}
