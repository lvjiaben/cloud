// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// NativeServerWriteFence is the golang structure of table plugin_idc_core_native_server_write_fence for DAO operations like Where/Data.
type NativeServerWriteFence struct {
	g.Meta                     `orm:"table:plugin_idc_core_native_server_write_fence, do:true"`
	TenantId                   any        //
	TaskNo                     any        //
	OperationNo                any        //
	EntitlementNo              any        //
	CustomerNo                 any        //
	ExpectedEntitlementVersion any        //
	ActionCode                 any        //
	RequestDigest              any        //
	StatusCode                 any        //
	EvidenceDigest             any        //
	CreatedAt                  *time.Time //
	UpdatedAt                  *time.Time //
	CompletedAt                *time.Time //
}
