// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesTransferBatch is the golang structure of table plugin_idc_core_sales_transfer_batch for DAO operations like Where/Data.
type SalesTransferBatch struct {
	g.Meta             `orm:"table:plugin_idc_core_sales_transfer_batch, do:true"`
	Id                 any        //
	TenantId           any        //
	BatchNo            any        //
	SourceMembershipNo any        //
	TargetMembershipNo any        //
	RequestedCount     any        //
	SucceededCount     any        //
	FailedCount        any        //
	ReasonCode         any        //
	RequestedBy        any        //
	StatusCode         any        //
	CreatedAt          *time.Time //
	CompletedAt        *time.Time //
}
