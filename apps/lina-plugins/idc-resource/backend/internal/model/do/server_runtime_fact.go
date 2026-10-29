// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ServerRuntimeFact is the golang structure of table plugin_idc_resource_server_runtime_fact for DAO operations like Where/Data.
type ServerRuntimeFact struct {
	g.Meta         `orm:"table:plugin_idc_resource_server_runtime_fact, do:true"`
	MappingNo      any        //
	TenantId       any        //
	Facts          any        //
	ObservedAt     *time.Time //
	NextRefreshAt  *time.Time //
	LeaseUntil     *time.Time //
	LeaseVersion   any        //
	LastErrorCode  any        //
	MappingVersion any        //
}
