// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// PrivacyDeletionTask is the golang structure of table plugin_idc_compliance_privacy_deletion_task for DAO operations like Where/Data.
type PrivacyDeletionTask struct {
	g.Meta            `orm:"table:plugin_idc_compliance_privacy_deletion_task, do:true"`
	Id                any        //
	TenantId          any        //
	TaskNo            any        //
	DataTypeCode      any        //
	TargetTypeCode    any        //
	TargetNo          any        //
	StatusCode        any        //
	DueAt             *time.Time //
	HoldReasonCode    any        //
	ConfirmedByUserId any        //
	ConfirmedAt       *time.Time //
	CompletedAt       *time.Time //
	Version           any        //
	CreatedAt         *time.Time //
	UpdatedAt         *time.Time //
}
