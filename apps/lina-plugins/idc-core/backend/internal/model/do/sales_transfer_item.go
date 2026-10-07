// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesTransferItem is the golang structure of table plugin_idc_core_sales_transfer_item for DAO operations like Where/Data.
type SalesTransferItem struct {
	g.Meta            `orm:"table:plugin_idc_core_sales_transfer_item, do:true"`
	Id                any        //
	TenantId          any        //
	ItemNo            any        //
	BatchNo           any        //
	CustomerNo        any        //
	OldAssignmentNo   any        //
	NewAssignmentNo   any        //
	StatusCode        any        //
	FailureReasonCode *string    //
	ProcessedAt       *time.Time //
	CreatedAt         *time.Time //
}
