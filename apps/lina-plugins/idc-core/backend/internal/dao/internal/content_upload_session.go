// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ContentUploadSessionDao is the data access object for the table plugin_idc_core_content_upload_session.
type ContentUploadSessionDao struct {
	table    string                      // table is the underlying table name of the DAO.
	group    string                      // group is the database configuration group name of the current DAO.
	columns  ContentUploadSessionColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler          // handlers for customized model modification.
}

// ContentUploadSessionColumns defines and stores column names for the table plugin_idc_core_content_upload_session.
type ContentUploadSessionColumns struct {
	Id                    string //
	TenantId              string //
	UploadNo              string //
	ContentNo             string //
	UploaderUserId        string //
	FileName              string //
	DeclaredSizeBytes     string //
	PersistedSizeBytes    string //
	TemporaryObjectRef    string //
	TransferMode          string //
	ProviderUploadId      string //
	ChunkSizeBytes        string //
	StatusCode            string //
	LastDataAt            string //
	CompletedAt           string //
	ExpiresAt             string //
	CleanupLeaseExpiresAt string //
	CleanupCompletedAt    string //
	Version               string //
	CreatedAt             string //
	UpdatedAt             string //
}

// contentUploadSessionColumns holds the columns for the table plugin_idc_core_content_upload_session.
var contentUploadSessionColumns = ContentUploadSessionColumns{
	Id:                    "id",
	TenantId:              "tenant_id",
	UploadNo:              "upload_no",
	ContentNo:             "content_no",
	UploaderUserId:        "uploader_user_id",
	FileName:              "file_name",
	DeclaredSizeBytes:     "declared_size_bytes",
	PersistedSizeBytes:    "persisted_size_bytes",
	TemporaryObjectRef:    "temporary_object_ref",
	TransferMode:          "transfer_mode",
	ProviderUploadId:      "provider_upload_id",
	ChunkSizeBytes:        "chunk_size_bytes",
	StatusCode:            "status_code",
	LastDataAt:            "last_data_at",
	CompletedAt:           "completed_at",
	ExpiresAt:             "expires_at",
	CleanupLeaseExpiresAt: "cleanup_lease_expires_at",
	CleanupCompletedAt:    "cleanup_completed_at",
	Version:               "version",
	CreatedAt:             "created_at",
	UpdatedAt:             "updated_at",
}

// NewContentUploadSessionDao creates and returns a new DAO object for table data access.
func NewContentUploadSessionDao(handlers ...gdb.ModelHandler) *ContentUploadSessionDao {
	return &ContentUploadSessionDao{
		group:    "default",
		table:    "plugin_idc_core_content_upload_session",
		columns:  contentUploadSessionColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ContentUploadSessionDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ContentUploadSessionDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ContentUploadSessionDao) Columns() ContentUploadSessionColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ContentUploadSessionDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ContentUploadSessionDao) Ctx(ctx context.Context) *gdb.Model {
	model := dao.DB().Model(dao.table)
	for _, handler := range dao.handlers {
		model = handler(model)
	}
	return model.Safe().Ctx(ctx)
}

// Transaction wraps the transaction logic using function f.
// It rolls back the transaction and returns the error if function f returns a non-nil error.
// It commits the transaction and returns nil if function f returns nil.
//
// Note: Do not commit or roll back the transaction in function f,
// as it is automatically handled by this function.
func (dao *ContentUploadSessionDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
