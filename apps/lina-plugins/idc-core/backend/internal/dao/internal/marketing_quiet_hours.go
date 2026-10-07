// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// MarketingQuietHoursDao is the data access object for the table plugin_idc_core_marketing_quiet_hours.
type MarketingQuietHoursDao struct {
	table    string                     // table is the underlying table name of the DAO.
	group    string                     // group is the database configuration group name of the current DAO.
	columns  MarketingQuietHoursColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler         // handlers for customized model modification.
}

// MarketingQuietHoursColumns defines and stores column names for the table plugin_idc_core_marketing_quiet_hours.
type MarketingQuietHoursColumns struct {
	Id           string //
	TenantId     string //
	CustomerNo   string //
	TimezoneName string //
	StartMinute  string //
	EndMinute    string //
	Enabled      string //
	Version      string //
	CreatedAt    string //
	UpdatedAt    string //
}

// marketingQuietHoursColumns holds the columns for the table plugin_idc_core_marketing_quiet_hours.
var marketingQuietHoursColumns = MarketingQuietHoursColumns{
	Id:           "id",
	TenantId:     "tenant_id",
	CustomerNo:   "customer_no",
	TimezoneName: "timezone_name",
	StartMinute:  "start_minute",
	EndMinute:    "end_minute",
	Enabled:      "enabled",
	Version:      "version",
	CreatedAt:    "created_at",
	UpdatedAt:    "updated_at",
}

// NewMarketingQuietHoursDao creates and returns a new DAO object for table data access.
func NewMarketingQuietHoursDao(handlers ...gdb.ModelHandler) *MarketingQuietHoursDao {
	return &MarketingQuietHoursDao{
		group:    "default",
		table:    "plugin_idc_core_marketing_quiet_hours",
		columns:  marketingQuietHoursColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *MarketingQuietHoursDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *MarketingQuietHoursDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *MarketingQuietHoursDao) Columns() MarketingQuietHoursColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *MarketingQuietHoursDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *MarketingQuietHoursDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *MarketingQuietHoursDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
