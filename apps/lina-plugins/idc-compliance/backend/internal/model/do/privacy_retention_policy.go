// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// PrivacyRetentionPolicy is the golang structure of table plugin_idc_compliance_privacy_retention_policy for DAO operations like Where/Data.
type PrivacyRetentionPolicy struct {
	g.Meta          `orm:"table:plugin_idc_compliance_privacy_retention_policy, do:true"`
	Id              any        //
	TenantId        any        //
	PolicyNo        any        //
	DataTypeCode    any        //
	Version         any        //
	RetentionDays   any        //
	StatusCode      any        //
	CreatedByUserId any        //
	SupersededAt    *time.Time //
	CreatedAt       *time.Time //
}
