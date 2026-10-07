// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// DeliveryQuery is the golang structure of table plugin_idc_notify_delivery_query for DAO operations like Where/Data.
type DeliveryQuery struct {
	g.Meta         `orm:"table:plugin_idc_notify_delivery_query, do:true"`
	TenantId       any        //
	QueryNo        any        //
	TaskNo         any        //
	SourceCode     any        //
	ResultClass    any        //
	EvidenceDigest any        //
	ObservedAt     *time.Time //
}
