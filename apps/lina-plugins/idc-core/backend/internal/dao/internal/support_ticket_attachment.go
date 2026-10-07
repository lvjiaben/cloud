// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SupportTicketAttachmentDao is the data access object for the table plugin_idc_core_support_ticket_attachment.
type SupportTicketAttachmentDao struct {
	table    string                         // table is the underlying table name of the DAO.
	group    string                         // group is the database configuration group name of the current DAO.
	columns  SupportTicketAttachmentColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler             // handlers for customized model modification.
}

// SupportTicketAttachmentColumns defines and stores column names for the table plugin_idc_core_support_ticket_attachment.
type SupportTicketAttachmentColumns struct {
	Id             string //
	TenantId       string //
	AttachmentNo   string //
	TicketNo       string //
	MessageNo      string //
	VisibilityCode string //
	FileName       string //
	MediaType      string //
	SizeBytes      string //
	Sha256Hex      string //
	ObjectRef      string //
	InspectionCode string //
	HiddenAt       string //
	HiddenBy       string //
	HiddenReason   string //
	CreatedBy      string //
	CreatedAt      string //
}

// supportTicketAttachmentColumns holds the columns for the table plugin_idc_core_support_ticket_attachment.
var supportTicketAttachmentColumns = SupportTicketAttachmentColumns{
	Id:             "id",
	TenantId:       "tenant_id",
	AttachmentNo:   "attachment_no",
	TicketNo:       "ticket_no",
	MessageNo:      "message_no",
	VisibilityCode: "visibility_code",
	FileName:       "file_name",
	MediaType:      "media_type",
	SizeBytes:      "size_bytes",
	Sha256Hex:      "sha256_hex",
	ObjectRef:      "object_ref",
	InspectionCode: "inspection_code",
	HiddenAt:       "hidden_at",
	HiddenBy:       "hidden_by",
	HiddenReason:   "hidden_reason",
	CreatedBy:      "created_by",
	CreatedAt:      "created_at",
}

// NewSupportTicketAttachmentDao creates and returns a new DAO object for table data access.
func NewSupportTicketAttachmentDao(handlers ...gdb.ModelHandler) *SupportTicketAttachmentDao {
	return &SupportTicketAttachmentDao{
		group:    "default",
		table:    "plugin_idc_core_support_ticket_attachment",
		columns:  supportTicketAttachmentColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SupportTicketAttachmentDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SupportTicketAttachmentDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SupportTicketAttachmentDao) Columns() SupportTicketAttachmentColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SupportTicketAttachmentDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SupportTicketAttachmentDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SupportTicketAttachmentDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
