// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ContentUploadSession is the golang structure for table content_upload_session.
type ContentUploadSession struct {
	Id                    int64      `json:"id"                    orm:"id"                       description:""`
	TenantId              int64      `json:"tenantId"              orm:"tenant_id"                description:""`
	UploadNo              string     `json:"uploadNo"              orm:"upload_no"                description:""`
	ContentNo             string     `json:"contentNo"             orm:"content_no"               description:""`
	UploaderUserId        int64      `json:"uploaderUserId"        orm:"uploader_user_id"         description:""`
	FileName              string     `json:"fileName"              orm:"file_name"                description:""`
	DeclaredSizeBytes     int64      `json:"declaredSizeBytes"     orm:"declared_size_bytes"      description:""`
	PersistedSizeBytes    int64      `json:"persistedSizeBytes"    orm:"persisted_size_bytes"     description:""`
	TemporaryObjectRef    string     `json:"temporaryObjectRef"    orm:"temporary_object_ref"     description:""`
	TransferMode          string     `json:"transferMode"          orm:"transfer_mode"            description:""`
	ProviderUploadId      string     `json:"providerUploadId"      orm:"provider_upload_id"       description:""`
	ChunkSizeBytes        int        `json:"chunkSizeBytes"        orm:"chunk_size_bytes"         description:""`
	StatusCode            string     `json:"statusCode"            orm:"status_code"              description:""`
	LastDataAt            *time.Time `json:"lastDataAt"            orm:"last_data_at"             description:""`
	CompletedAt           *time.Time `json:"completedAt"           orm:"completed_at"             description:""`
	ExpiresAt             *time.Time `json:"expiresAt"             orm:"expires_at"               description:""`
	CleanupLeaseExpiresAt *time.Time `json:"cleanupLeaseExpiresAt" orm:"cleanup_lease_expires_at" description:""`
	CleanupCompletedAt    *time.Time `json:"cleanupCompletedAt"    orm:"cleanup_completed_at"     description:""`
	Version               int64      `json:"version"               orm:"version"                  description:""`
	CreatedAt             *time.Time `json:"createdAt"             orm:"created_at"               description:""`
	UpdatedAt             *time.Time `json:"updatedAt"             orm:"updated_at"               description:""`
}
