// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ContentUploadSession is the golang structure of table plugin_idc_core_content_upload_session for DAO operations like Where/Data.
type ContentUploadSession struct {
	g.Meta                `orm:"table:plugin_idc_core_content_upload_session, do:true"`
	Id                    any        //
	TenantId              any        //
	UploadNo              any        //
	ContentNo             any        //
	UploaderUserId        any        //
	FileName              any        //
	DeclaredSizeBytes     any        //
	PersistedSizeBytes    any        //
	TemporaryObjectRef    any        //
	TransferMode          any        //
	ProviderUploadId      any        //
	ChunkSizeBytes        any        //
	StatusCode            any        //
	LastDataAt            *time.Time //
	CompletedAt           *time.Time //
	ExpiresAt             *time.Time //
	CleanupLeaseExpiresAt *time.Time //
	CleanupCompletedAt    *time.Time //
	Version               any        //
	CreatedAt             *time.Time //
	UpdatedAt             *time.Time //
}
