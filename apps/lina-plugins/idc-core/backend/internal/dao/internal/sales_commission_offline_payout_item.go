// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SalesCommissionOfflinePayoutItemDao is the data access object for the table plugin_idc_core_sales_commission_offline_payout_item.
type SalesCommissionOfflinePayoutItemDao struct {
	table    string                                  // table is the underlying table name of the DAO.
	group    string                                  // group is the database configuration group name of the current DAO.
	columns  SalesCommissionOfflinePayoutItemColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                      // handlers for customized model modification.
}

// SalesCommissionOfflinePayoutItemColumns defines and stores column names for the table plugin_idc_core_sales_commission_offline_payout_item.
type SalesCommissionOfflinePayoutItemColumns struct {
	Id           string //
	TenantId     string //
	PayoutNo     string //
	CommissionNo string //
	AmountMinor  string //
	CreatedAt    string //
}

// salesCommissionOfflinePayoutItemColumns holds the columns for the table plugin_idc_core_sales_commission_offline_payout_item.
var salesCommissionOfflinePayoutItemColumns = SalesCommissionOfflinePayoutItemColumns{
	Id:           "id",
	TenantId:     "tenant_id",
	PayoutNo:     "payout_no",
	CommissionNo: "commission_no",
	AmountMinor:  "amount_minor",
	CreatedAt:    "created_at",
}

// NewSalesCommissionOfflinePayoutItemDao creates and returns a new DAO object for table data access.
func NewSalesCommissionOfflinePayoutItemDao(handlers ...gdb.ModelHandler) *SalesCommissionOfflinePayoutItemDao {
	return &SalesCommissionOfflinePayoutItemDao{
		group:    "default",
		table:    "plugin_idc_core_sales_commission_offline_payout_item",
		columns:  salesCommissionOfflinePayoutItemColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SalesCommissionOfflinePayoutItemDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SalesCommissionOfflinePayoutItemDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SalesCommissionOfflinePayoutItemDao) Columns() SalesCommissionOfflinePayoutItemColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SalesCommissionOfflinePayoutItemDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SalesCommissionOfflinePayoutItemDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SalesCommissionOfflinePayoutItemDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
