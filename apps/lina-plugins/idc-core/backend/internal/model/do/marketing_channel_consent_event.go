// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// MarketingChannelConsentEvent is the golang structure of table plugin_idc_core_marketing_channel_consent_event for DAO operations like Where/Data.
type MarketingChannelConsentEvent struct {
	g.Meta               `orm:"table:plugin_idc_core_marketing_channel_consent_event, do:true"`
	Id                   any        //
	TenantId             any        //
	EventNo              any        //
	CustomerNo           any        //
	ContactNo            any        //
	ChannelCode          any        //
	ActionCode           any        //
	PolicyReference      any        //
	PolicyVersion        any        //
	Locale               any        //
	SourceCode           any        //
	EvidenceDigest       any        //
	IdempotencyKeyDigest any        //
	RequestFingerprint   any        //
	OccurredAt           *time.Time //
	CreatedAt            *time.Time //
}
