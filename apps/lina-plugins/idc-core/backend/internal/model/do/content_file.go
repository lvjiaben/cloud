// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ContentFile is the golang structure of table plugin_idc_core_content_file for DAO operations like Where/Data.
type ContentFile struct {
	g.Meta              `orm:"table:plugin_idc_core_content_file, do:true"`
	Id                  any        //
	TenantId            any        //
	FileNo              any        //
	ContentNo           any        //
	UploadNo            any        //
	CandidateGeneration any        //
	RoleCode            any        //
	StatusCode          any        //
	FileName            any        //
	MediaType           any        //
	ContentKind         any        //
	SizeBytes           any        //
	Sha256Hex           any        //
	ObjectRef           any        //
	ScanRequired        any        //
	ScanStatus          any        //
	GateCompletedAt     *time.Time //
	RetentionExpiresAt  *time.Time //
	CancelledAt         *time.Time //
	EnabledAt           *time.Time //
	ObjectDeletedAt     *time.Time //
	Version             any        //
	CreatedBy           any        //
	CreatedAt           *time.Time //
	UpdatedAt           *time.Time //
}
