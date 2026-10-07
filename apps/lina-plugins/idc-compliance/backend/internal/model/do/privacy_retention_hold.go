// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// PrivacyRetentionHold is the golang structure of table plugin_idc_compliance_privacy_retention_hold for DAO operations like Where/Data.
type PrivacyRetentionHold struct {
	g.Meta          `orm:"table:plugin_idc_compliance_privacy_retention_hold, do:true"`
	Id              any        //
	TenantId        any        //
	HoldNo          any        //
	TargetTypeCode  any        //
	TargetNo        any        //
	ReasonCode      any        //
	StatusCode      any        //
	CreatedByOwner  any        //
	ReleasedByOwner any        //
	ReleasedAt      *time.Time //
	CreatedAt       *time.Time //
	UpdatedAt       *time.Time //
}
