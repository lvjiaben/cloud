// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// FinancialSnapshotDao is the data access object for the table plugin_idc_resource_financial_snapshot.
type FinancialSnapshotDao struct {
	table    string                   // table is the underlying table name of the DAO.
	group    string                   // group is the database configuration group name of the current DAO.
	columns  FinancialSnapshotColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler       // handlers for customized model modification.
}

// FinancialSnapshotColumns defines and stores column names for the table plugin_idc_resource_financial_snapshot.
type FinancialSnapshotColumns struct {
	Id                  string //
	TenantId            string //
	SnapshotNo          string //
	ChannelNo           string //
	SnapshotKind        string //
	ExternalResourceRef string //
	CurrencyCode        string //
	Amount              string // Non-negative Provider amount in the exact ISO currency; never converted implicitly
	ExpiresAt           string //
	Projection          string //
	ObservedAt          string //
	CreatedAt           string //
}

// financialSnapshotColumns holds the columns for the table plugin_idc_resource_financial_snapshot.
var financialSnapshotColumns = FinancialSnapshotColumns{
	Id:                  "id",
	TenantId:            "tenant_id",
	SnapshotNo:          "snapshot_no",
	ChannelNo:           "channel_no",
	SnapshotKind:        "snapshot_kind",
	ExternalResourceRef: "external_resource_ref",
	CurrencyCode:        "currency_code",
	Amount:              "amount",
	ExpiresAt:           "expires_at",
	Projection:          "projection",
	ObservedAt:          "observed_at",
	CreatedAt:           "created_at",
}

// NewFinancialSnapshotDao creates and returns a new DAO object for table data access.
func NewFinancialSnapshotDao(handlers ...gdb.ModelHandler) *FinancialSnapshotDao {
	return &FinancialSnapshotDao{
		group:    "default",
		table:    "plugin_idc_resource_financial_snapshot",
		columns:  financialSnapshotColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *FinancialSnapshotDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *FinancialSnapshotDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *FinancialSnapshotDao) Columns() FinancialSnapshotColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *FinancialSnapshotDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *FinancialSnapshotDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *FinancialSnapshotDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
