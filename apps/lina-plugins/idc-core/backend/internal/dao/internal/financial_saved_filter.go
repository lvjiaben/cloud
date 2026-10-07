// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// FinancialSavedFilterDao is the data access object for the table plugin_idc_core_financial_saved_filter.
type FinancialSavedFilterDao struct {
	table    string                      // table is the underlying table name of the DAO.
	group    string                      // group is the database configuration group name of the current DAO.
	columns  FinancialSavedFilterColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler          // handlers for customized model modification.
}

// FinancialSavedFilterColumns defines and stores column names for the table plugin_idc_core_financial_saved_filter.
type FinancialSavedFilterColumns struct {
	Id           string //
	TenantId     string //
	FilterNo     string //
	OwnerUserId  string //
	ViewCode     string //
	DisplayName  string //
	FilterJson   string //
	FilterDigest string //
	Version      string //
	CreatedAt    string //
	UpdatedAt    string //
}

// financialSavedFilterColumns holds the columns for the table plugin_idc_core_financial_saved_filter.
var financialSavedFilterColumns = FinancialSavedFilterColumns{
	Id:           "id",
	TenantId:     "tenant_id",
	FilterNo:     "filter_no",
	OwnerUserId:  "owner_user_id",
	ViewCode:     "view_code",
	DisplayName:  "display_name",
	FilterJson:   "filter_json",
	FilterDigest: "filter_digest",
	Version:      "version",
	CreatedAt:    "created_at",
	UpdatedAt:    "updated_at",
}

// NewFinancialSavedFilterDao creates and returns a new DAO object for table data access.
func NewFinancialSavedFilterDao(handlers ...gdb.ModelHandler) *FinancialSavedFilterDao {
	return &FinancialSavedFilterDao{
		group:    "default",
		table:    "plugin_idc_core_financial_saved_filter",
		columns:  financialSavedFilterColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *FinancialSavedFilterDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *FinancialSavedFilterDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *FinancialSavedFilterDao) Columns() FinancialSavedFilterColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *FinancialSavedFilterDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *FinancialSavedFilterDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *FinancialSavedFilterDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
