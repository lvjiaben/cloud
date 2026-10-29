// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// NativeCommercialTask is the golang structure of table plugin_idc_resource_native_commercial_task for DAO operations like Where/Data.
type NativeCommercialTask struct {
	g.Meta              `orm:"table:plugin_idc_resource_native_commercial_task, do:true"`
	OperationNo         any        //
	TenantId            any        //
	EntitlementNo       any        //
	CustomerNo          any        //
	TaskNo              any        //
	LocalResourceNo     any        //
	ChannelNo           any        //
	PoolNo              any        //
	ExternalResourceRef any        //
	ActionCode          any        //
	Parameters          any        //
	RequestDigest       any        //
	IntentDigest        any        //
	EntitlementVersion  any        //
	AdmittedAt          *time.Time //
	AdmittedAttempt     any        //
	ResultStatus        any        //
	NextReportAt        *time.Time //
	CompletedAt         *time.Time //
	CreatedAt           *time.Time //
	UpdatedAt           *time.Time //
}
