// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ContentDownloadTicketDao is the data access object for the table plugin_idc_core_content_download_ticket.
type ContentDownloadTicketDao struct {
	table    string                       // table is the underlying table name of the DAO.
	group    string                       // group is the database configuration group name of the current DAO.
	columns  ContentDownloadTicketColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler           // handlers for customized model modification.
}

// ContentDownloadTicketColumns defines and stores column names for the table plugin_idc_core_content_download_ticket.
type ContentDownloadTicketColumns struct {
	Id                     string //
	TenantId               string //
	TicketNo               string //
	TokenDigest            string //
	ContentNo              string //
	FileNo                 string //
	ContentValidityVersion string //
	VisibilityCode         string //
	UserId                 string //
	SessionReferenceDigest string //
	AuthorizationRevision  string //
	IssuedAt               string //
	ExpiresAt              string //
	RevokedAt              string //
}

// contentDownloadTicketColumns holds the columns for the table plugin_idc_core_content_download_ticket.
var contentDownloadTicketColumns = ContentDownloadTicketColumns{
	Id:                     "id",
	TenantId:               "tenant_id",
	TicketNo:               "ticket_no",
	TokenDigest:            "token_digest",
	ContentNo:              "content_no",
	FileNo:                 "file_no",
	ContentValidityVersion: "content_validity_version",
	VisibilityCode:         "visibility_code",
	UserId:                 "user_id",
	SessionReferenceDigest: "session_reference_digest",
	AuthorizationRevision:  "authorization_revision",
	IssuedAt:               "issued_at",
	ExpiresAt:              "expires_at",
	RevokedAt:              "revoked_at",
}

// NewContentDownloadTicketDao creates and returns a new DAO object for table data access.
func NewContentDownloadTicketDao(handlers ...gdb.ModelHandler) *ContentDownloadTicketDao {
	return &ContentDownloadTicketDao{
		group:    "default",
		table:    "plugin_idc_core_content_download_ticket",
		columns:  contentDownloadTicketColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ContentDownloadTicketDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ContentDownloadTicketDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ContentDownloadTicketDao) Columns() ContentDownloadTicketColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ContentDownloadTicketDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ContentDownloadTicketDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ContentDownloadTicketDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
