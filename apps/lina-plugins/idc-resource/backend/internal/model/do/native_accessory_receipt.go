// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// NativeAccessoryReceipt is the golang structure of table plugin_idc_resource_native_accessory_receipt for DAO operations like Where/Data.
type NativeAccessoryReceipt struct {
	g.Meta        `orm:"table:plugin_idc_resource_native_accessory_receipt, do:true"`
	TaskNo        any        //
	TenantId      any        //
	OperationNo   any        //
	EntitlementNo any        //
	CustomerNo    any        //
	ObjectNo      any        //
	AccessoryKind any        //
	NextReportAt  *time.Time //
	CompletedAt   *time.Time //
	CreatedAt     *time.Time //
	UpdatedAt     *time.Time //
}
