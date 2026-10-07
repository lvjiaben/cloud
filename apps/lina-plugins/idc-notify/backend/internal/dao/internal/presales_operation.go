// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// PresalesOperationDao is the data access object for the table plugin_idc_notify_presales_operation.
type PresalesOperationDao struct {
	table    string                   // table is the underlying table name of the DAO.
	group    string                   // group is the database configuration group name of the current DAO.
	columns  PresalesOperationColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler       // handlers for customized model modification.
}

// PresalesOperationColumns defines and stores column names for the table plugin_idc_notify_presales_operation.
type PresalesOperationColumns struct {
	TenantId                string //
	OperationDigest         string //
	OperationCode           string //
	VerificationNo          string //
	DestinationKind         string //
	DestinationDigest       string //
	BrowserDigest           string //
	TicketDigest            string //
	RequestDigest           string //
	StatusCode              string //
	TicketNo                string //
	AccessExpiresAt         string //
	LinkDeliveryState       string //
	ProviderBoundaryCrossed string //
	CompletedAt             string //
	CreatedAt               string //
	UpdatedAt               string //
}

// presalesOperationColumns holds the columns for the table plugin_idc_notify_presales_operation.
var presalesOperationColumns = PresalesOperationColumns{
	TenantId:                "tenant_id",
	OperationDigest:         "operation_digest",
	OperationCode:           "operation_code",
	VerificationNo:          "verification_no",
	DestinationKind:         "destination_kind",
	DestinationDigest:       "destination_digest",
	BrowserDigest:           "browser_digest",
	TicketDigest:            "ticket_digest",
	RequestDigest:           "request_digest",
	StatusCode:              "status_code",
	TicketNo:                "ticket_no",
	AccessExpiresAt:         "access_expires_at",
	LinkDeliveryState:       "link_delivery_state",
	ProviderBoundaryCrossed: "provider_boundary_crossed",
	CompletedAt:             "completed_at",
	CreatedAt:               "created_at",
	UpdatedAt:               "updated_at",
}

// NewPresalesOperationDao creates and returns a new DAO object for table data access.
func NewPresalesOperationDao(handlers ...gdb.ModelHandler) *PresalesOperationDao {
	return &PresalesOperationDao{
		group:    "default",
		table:    "plugin_idc_notify_presales_operation",
		columns:  presalesOperationColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *PresalesOperationDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *PresalesOperationDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *PresalesOperationDao) Columns() PresalesOperationColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *PresalesOperationDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *PresalesOperationDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *PresalesOperationDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
