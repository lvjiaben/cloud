// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CustomerAttachmentDao is the data access object for the table plugin_idc_core_customer_attachment.
type CustomerAttachmentDao struct {
	table    string                    // table is the underlying table name of the DAO.
	group    string                    // group is the database configuration group name of the current DAO.
	columns  CustomerAttachmentColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler        // handlers for customized model modification.
}

// CustomerAttachmentColumns defines and stores column names for the table plugin_idc_core_customer_attachment.
type CustomerAttachmentColumns struct {
	Id               string //
	TenantId         string //
	AttachmentNo     string //
	CustomerNo       string //
	CategoryCode     string //
	VisibilityCode   string //
	ObjectKey        string // Internal private object key, never a public or long-lived download URL
	OriginalFilename string //
	MediaType        string //
	SizeBytes        string //
	ContentSha256    string //
	ScanStatusCode   string //
	StatusCode       string //
	RetentionUntil   string //
	UploadedBy       string //
	AvailableAt      string //
	RemovedAt        string //
	CreatedAt        string //
	UpdatedAt        string //
}

// customerAttachmentColumns holds the columns for the table plugin_idc_core_customer_attachment.
var customerAttachmentColumns = CustomerAttachmentColumns{
	Id:               "id",
	TenantId:         "tenant_id",
	AttachmentNo:     "attachment_no",
	CustomerNo:       "customer_no",
	CategoryCode:     "category_code",
	VisibilityCode:   "visibility_code",
	ObjectKey:        "object_key",
	OriginalFilename: "original_filename",
	MediaType:        "media_type",
	SizeBytes:        "size_bytes",
	ContentSha256:    "content_sha256",
	ScanStatusCode:   "scan_status_code",
	StatusCode:       "status_code",
	RetentionUntil:   "retention_until",
	UploadedBy:       "uploaded_by",
	AvailableAt:      "available_at",
	RemovedAt:        "removed_at",
	CreatedAt:        "created_at",
	UpdatedAt:        "updated_at",
}

// NewCustomerAttachmentDao creates and returns a new DAO object for table data access.
func NewCustomerAttachmentDao(handlers ...gdb.ModelHandler) *CustomerAttachmentDao {
	return &CustomerAttachmentDao{
		group:    "default",
		table:    "plugin_idc_core_customer_attachment",
		columns:  customerAttachmentColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CustomerAttachmentDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CustomerAttachmentDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CustomerAttachmentDao) Columns() CustomerAttachmentColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CustomerAttachmentDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CustomerAttachmentDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CustomerAttachmentDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
