// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// PrivacySupportExport is the golang structure of table plugin_idc_compliance_privacy_support_export for DAO operations like Where/Data.
type PrivacySupportExport struct {
	g.Meta          `orm:"table:plugin_idc_compliance_privacy_support_export, do:true"`
	Id              any        //
	TenantId        any        //
	ExportNo        any        //
	PurposeCode     any        //
	AssessmentNo    any        //
	FieldCount      any        //
	RecordCount     any        //
	ContentSha256   any        //
	ContentBytes    []byte     //
	StatusCode      any        //
	RetentionUntil  *time.Time //
	CreatedByUserId any        //
	CreatedAt       *time.Time //
}
