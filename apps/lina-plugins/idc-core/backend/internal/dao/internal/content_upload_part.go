// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ContentUploadPartDao is the data access object for the table plugin_idc_core_content_upload_part.
type ContentUploadPartDao struct {
	table    string                   // table is the underlying table name of the DAO.
	group    string                   // group is the database configuration group name of the current DAO.
	columns  ContentUploadPartColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler       // handlers for customized model modification.
}

// ContentUploadPartColumns defines and stores column names for the table plugin_idc_core_content_upload_part.
type ContentUploadPartColumns struct {
	Id          string //
	TenantId    string //
	UploadNo    string //
	PartNumber  string //
	OffsetBytes string //
	SizeBytes   string //
	Etag        string //
	ObjectRef   string //
	Sha256Hex   string //
	CreatedAt   string //
}

// contentUploadPartColumns holds the columns for the table plugin_idc_core_content_upload_part.
var contentUploadPartColumns = ContentUploadPartColumns{
	Id:          "id",
	TenantId:    "tenant_id",
	UploadNo:    "upload_no",
	PartNumber:  "part_number",
	OffsetBytes: "offset_bytes",
	SizeBytes:   "size_bytes",
	Etag:        "etag",
	ObjectRef:   "object_ref",
	Sha256Hex:   "sha256_hex",
	CreatedAt:   "created_at",
}

// NewContentUploadPartDao creates and returns a new DAO object for table data access.
func NewContentUploadPartDao(handlers ...gdb.ModelHandler) *ContentUploadPartDao {
	return &ContentUploadPartDao{
		group:    "default",
		table:    "plugin_idc_core_content_upload_part",
		columns:  contentUploadPartColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ContentUploadPartDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ContentUploadPartDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ContentUploadPartDao) Columns() ContentUploadPartColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ContentUploadPartDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ContentUploadPartDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ContentUploadPartDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
