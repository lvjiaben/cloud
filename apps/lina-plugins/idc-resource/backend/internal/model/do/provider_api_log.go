// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ProviderApiLog is the golang structure of table plugin_idc_resource_provider_api_log for DAO operations like Where/Data.
type ProviderApiLog struct {
	g.Meta           `orm:"table:plugin_idc_resource_provider_api_log, do:true"`
	Id               any        //
	TenantId         any        //
	ChannelNo        any        //
	ProviderPluginId any        //
	MethodCode       any        //
	Succeeded        any        //
	ErrorCode        any        //
	DurationMs       any        //
	StartedAt        *time.Time //
	CreatedAt        *time.Time //
}
