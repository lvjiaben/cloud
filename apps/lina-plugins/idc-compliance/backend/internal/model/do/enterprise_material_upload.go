// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// EnterpriseMaterialUpload is the golang structure of table plugin_idc_compliance_enterprise_material_upload for DAO operations like Where/Data.
type EnterpriseMaterialUpload struct {
	g.Meta               `orm:"table:plugin_idc_compliance_enterprise_material_upload, do:true"`
	Id                   any        //
	TenantId             any        //
	MaterialNo           any        //
	CaseNo               any        //
	CaseSubjectTypeCode  any        //
	MaterialTypeCode     any        //
	IdempotencyKeyDigest any        //
	RequestFingerprint   any        //
	StatusCode           any        //
	Version              any        //
	CreatedAt            *time.Time //
	UpdatedAt            *time.Time //
}
