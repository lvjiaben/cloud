// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ChannelTestOperation is the golang structure of table plugin_idc_notify_channel_test_operation for DAO operations like Where/Data.
type ChannelTestOperation struct {
	g.Meta                  `orm:"table:plugin_idc_notify_channel_test_operation, do:true"`
	TenantId                any        //
	OperationNo             any        //
	ChannelCode             any        //
	ChannelVersion          any        //
	TemplateNo              any        //
	TemplateVersion         any        //
	TargetMask              any        //
	RequestDigest           any        //
	StatusCode              any        //
	ResultClass             any        //
	CrossedExternalBoundary any        //
	ProviderReference       any        //
	ActorUserId             any        //
	CompletedAt             any        //
	CreatedAt               *time.Time //
	UpdatedAt               *time.Time //
}
