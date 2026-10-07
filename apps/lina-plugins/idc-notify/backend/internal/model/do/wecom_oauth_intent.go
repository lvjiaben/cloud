// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// WecomOauthIntent is the golang structure of table plugin_idc_notify_wecom_oauth_intent for DAO operations like Where/Data.
type WecomOauthIntent struct {
	g.Meta            `orm:"table:plugin_idc_notify_wecom_oauth_intent, do:true"`
	TenantId          any        //
	IntentNo          any        //
	StateDigest       any        //
	HostUserId        any        //
	StaffVersion      any        //
	SessionDigest     any        //
	ChannelGeneration any        //
	ChannelVersion    any        //
	CorpId            any        //
	AgentId           any        //
	StatusCode        any        //
	FailureCode       any        //
	LeaseToken        any        //
	LeaseExpiresAt    any        //
	ExpiresAt         any        //
	CompletedAt       any        //
	Version           any        //
	CreatedAt         *time.Time //
	UpdatedAt         *time.Time //
}
