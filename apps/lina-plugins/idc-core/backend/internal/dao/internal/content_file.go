// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ContentFileDao is the data access object for the table plugin_idc_core_content_file.
type ContentFileDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  ContentFileColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// ContentFileColumns defines and stores column names for the table plugin_idc_core_content_file.
type ContentFileColumns struct {
	Id                  string //
	TenantId            string //
	FileNo              string //
	ContentNo           string //
	UploadNo            string //
	CandidateGeneration string //
	RoleCode            string //
	StatusCode          string //
	FileName            string //
	MediaType           string //
	ContentKind         string //
	SizeBytes           string //
	Sha256Hex           string //
	ObjectRef           string //
	ScanRequired        string //
	ScanStatus          string //
	GateCompletedAt     string //
	RetentionExpiresAt  string //
	CancelledAt         string //
	EnabledAt           string //
	ObjectDeletedAt     string //
	Version             string //
	CreatedBy           string //
	CreatedAt           string //
	UpdatedAt           string //
}

// contentFileColumns holds the columns for the table plugin_idc_core_content_file.
var contentFileColumns = ContentFileColumns{
	Id:                  "id",
	TenantId:            "tenant_id",
	FileNo:              "file_no",
	ContentNo:           "content_no",
	UploadNo:            "upload_no",
	CandidateGeneration: "candidate_generation",
	RoleCode:            "role_code",
	StatusCode:          "status_code",
	FileName:            "file_name",
	MediaType:           "media_type",
	ContentKind:         "content_kind",
	SizeBytes:           "size_bytes",
	Sha256Hex:           "sha256_hex",
	ObjectRef:           "object_ref",
	ScanRequired:        "scan_required",
	ScanStatus:          "scan_status",
	GateCompletedAt:     "gate_completed_at",
	RetentionExpiresAt:  "retention_expires_at",
	CancelledAt:         "cancelled_at",
	EnabledAt:           "enabled_at",
	ObjectDeletedAt:     "object_deleted_at",
	Version:             "version",
	CreatedBy:           "created_by",
	CreatedAt:           "created_at",
	UpdatedAt:           "updated_at",
}

// NewContentFileDao creates and returns a new DAO object for table data access.
func NewContentFileDao(handlers ...gdb.ModelHandler) *ContentFileDao {
	return &ContentFileDao{
		group:    "default",
		table:    "plugin_idc_core_content_file",
		columns:  contentFileColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ContentFileDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ContentFileDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ContentFileDao) Columns() ContentFileColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ContentFileDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ContentFileDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ContentFileDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
