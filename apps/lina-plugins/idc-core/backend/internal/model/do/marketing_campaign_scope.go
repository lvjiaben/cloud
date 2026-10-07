// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// MarketingCampaignScope is the golang structure of table plugin_idc_core_marketing_campaign_scope for DAO operations like Where/Data.
type MarketingCampaignScope struct {
	g.Meta        `orm:"table:plugin_idc_core_marketing_campaign_scope, do:true"`
	Id            any        //
	TenantId      any        //
	CampaignNo    any        //
	ScopeKindCode any        //
	MatchModeCode any        //
	ReferenceCode any        //
	CreatedAt     *time.Time //
}
