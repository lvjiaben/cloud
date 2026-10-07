// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SupportTicketAccessLinkDao is the data access object for the table plugin_idc_core_support_ticket_access_link.
type SupportTicketAccessLinkDao struct {
	table    string                         // table is the underlying table name of the DAO.
	group    string                         // group is the database configuration group name of the current DAO.
	columns  SupportTicketAccessLinkColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler             // handlers for customized model modification.
}

// SupportTicketAccessLinkColumns defines and stores column names for the table plugin_idc_core_support_ticket_access_link.
type SupportTicketAccessLinkColumns struct {
	Id                string //
	TenantId          string //
	LinkNo            string //
	TicketNo          string //
	TokenDigest       string //
	DestinationDigest string //
	DestinationKind   string //
	OperationDigest   string //
	RequestDigest     string //
	TokenSecretRef    string //
	ExpiresAt         string //
	RevokedAt         string //
	CreatedAt         string //
}

// supportTicketAccessLinkColumns holds the columns for the table plugin_idc_core_support_ticket_access_link.
var supportTicketAccessLinkColumns = SupportTicketAccessLinkColumns{
	Id:                "id",
	TenantId:          "tenant_id",
	LinkNo:            "link_no",
	TicketNo:          "ticket_no",
	TokenDigest:       "token_digest",
	DestinationDigest: "destination_digest",
	DestinationKind:   "destination_kind",
	OperationDigest:   "operation_digest",
	RequestDigest:     "request_digest",
	TokenSecretRef:    "token_secret_ref",
	ExpiresAt:         "expires_at",
	RevokedAt:         "revoked_at",
	CreatedAt:         "created_at",
}

// NewSupportTicketAccessLinkDao creates and returns a new DAO object for table data access.
func NewSupportTicketAccessLinkDao(handlers ...gdb.ModelHandler) *SupportTicketAccessLinkDao {
	return &SupportTicketAccessLinkDao{
		group:    "default",
		table:    "plugin_idc_core_support_ticket_access_link",
		columns:  supportTicketAccessLinkColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SupportTicketAccessLinkDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SupportTicketAccessLinkDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SupportTicketAccessLinkDao) Columns() SupportTicketAccessLinkColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SupportTicketAccessLinkDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SupportTicketAccessLinkDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SupportTicketAccessLinkDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
