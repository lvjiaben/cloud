// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// MarketingConditionValueDao is the data access object for the table plugin_idc_core_marketing_condition_value.
type MarketingConditionValueDao struct {
	table    string                         // table is the underlying table name of the DAO.
	group    string                         // group is the database configuration group name of the current DAO.
	columns  MarketingConditionValueColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler             // handlers for customized model modification.
}

// MarketingConditionValueColumns defines and stores column names for the table plugin_idc_core_marketing_condition_value.
type MarketingConditionValueColumns struct {
	Id           string //
	TenantId     string //
	ConditionNo  string //
	ValueText    string //
	ValueInteger string //
	SortOrder    string //
	CreatedAt    string //
}

// marketingConditionValueColumns holds the columns for the table plugin_idc_core_marketing_condition_value.
var marketingConditionValueColumns = MarketingConditionValueColumns{
	Id:           "id",
	TenantId:     "tenant_id",
	ConditionNo:  "condition_no",
	ValueText:    "value_text",
	ValueInteger: "value_integer",
	SortOrder:    "sort_order",
	CreatedAt:    "created_at",
}

// NewMarketingConditionValueDao creates and returns a new DAO object for table data access.
func NewMarketingConditionValueDao(handlers ...gdb.ModelHandler) *MarketingConditionValueDao {
	return &MarketingConditionValueDao{
		group:    "default",
		table:    "plugin_idc_core_marketing_condition_value",
		columns:  marketingConditionValueColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *MarketingConditionValueDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *MarketingConditionValueDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *MarketingConditionValueDao) Columns() MarketingConditionValueColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *MarketingConditionValueDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *MarketingConditionValueDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *MarketingConditionValueDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
