// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SupportTicketAttachmentUploadDao is the data access object for the table plugin_idc_core_support_ticket_attachment_upload.
type SupportTicketAttachmentUploadDao struct {
	table    string                               // table is the underlying table name of the DAO.
	group    string                               // group is the database configuration group name of the current DAO.
	columns  SupportTicketAttachmentUploadColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                   // handlers for customized model modification.
}

// SupportTicketAttachmentUploadColumns defines and stores column names for the table plugin_idc_core_support_ticket_attachment_upload.
type SupportTicketAttachmentUploadColumns struct {
	Id                 string //
	TenantId           string //
	UploadNo           string //
	TicketNo           string //
	UploaderUserId     string //
	UploaderType       string //
	ContextCode        string //
	FileName           string //
	DeclaredSizeBytes  string //
	MediaType          string //
	Sha256Hex          string //
	TemporaryObjectRef string //
	StatusCode         string //
	ExpiresAt          string //
	CompletedAt        string //
	ConsumedAt         string //
	Version            string //
	CreatedAt          string //
	UpdatedAt          string //
}

// supportTicketAttachmentUploadColumns holds the columns for the table plugin_idc_core_support_ticket_attachment_upload.
var supportTicketAttachmentUploadColumns = SupportTicketAttachmentUploadColumns{
	Id:                 "id",
	TenantId:           "tenant_id",
	UploadNo:           "upload_no",
	TicketNo:           "ticket_no",
	UploaderUserId:     "uploader_user_id",
	UploaderType:       "uploader_type",
	ContextCode:        "context_code",
	FileName:           "file_name",
	DeclaredSizeBytes:  "declared_size_bytes",
	MediaType:          "media_type",
	Sha256Hex:          "sha256_hex",
	TemporaryObjectRef: "temporary_object_ref",
	StatusCode:         "status_code",
	ExpiresAt:          "expires_at",
	CompletedAt:        "completed_at",
	ConsumedAt:         "consumed_at",
	Version:            "version",
	CreatedAt:          "created_at",
	UpdatedAt:          "updated_at",
}

// NewSupportTicketAttachmentUploadDao creates and returns a new DAO object for table data access.
func NewSupportTicketAttachmentUploadDao(handlers ...gdb.ModelHandler) *SupportTicketAttachmentUploadDao {
	return &SupportTicketAttachmentUploadDao{
		group:    "default",
		table:    "plugin_idc_core_support_ticket_attachment_upload",
		columns:  supportTicketAttachmentUploadColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SupportTicketAttachmentUploadDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SupportTicketAttachmentUploadDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SupportTicketAttachmentUploadDao) Columns() SupportTicketAttachmentUploadColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SupportTicketAttachmentUploadDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SupportTicketAttachmentUploadDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SupportTicketAttachmentUploadDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
