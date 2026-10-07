// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SalesAgentLevelDao is the data access object for the table plugin_idc_core_sales_agent_level.
type SalesAgentLevelDao struct {
	table    string                 // table is the underlying table name of the DAO.
	group    string                 // group is the database configuration group name of the current DAO.
	columns  SalesAgentLevelColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler     // handlers for customized model modification.
}

// SalesAgentLevelColumns defines and stores column names for the table plugin_idc_core_sales_agent_level.
type SalesAgentLevelColumns struct {
	Id         string //
	TenantId   string //
	LevelNo    string //
	LevelCode  string //
	NameZhCn   string //
	NameEnUs   string //
	SortOrder  string //
	StatusCode string //
	Version    string //
	CreatedAt  string //
	UpdatedAt  string //
}

// salesAgentLevelColumns holds the columns for the table plugin_idc_core_sales_agent_level.
var salesAgentLevelColumns = SalesAgentLevelColumns{
	Id:         "id",
	TenantId:   "tenant_id",
	LevelNo:    "level_no",
	LevelCode:  "level_code",
	NameZhCn:   "name_zh_cn",
	NameEnUs:   "name_en_us",
	SortOrder:  "sort_order",
	StatusCode: "status_code",
	Version:    "version",
	CreatedAt:  "created_at",
	UpdatedAt:  "updated_at",
}

// NewSalesAgentLevelDao creates and returns a new DAO object for table data access.
func NewSalesAgentLevelDao(handlers ...gdb.ModelHandler) *SalesAgentLevelDao {
	return &SalesAgentLevelDao{
		group:    "default",
		table:    "plugin_idc_core_sales_agent_level",
		columns:  salesAgentLevelColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SalesAgentLevelDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SalesAgentLevelDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SalesAgentLevelDao) Columns() SalesAgentLevelColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SalesAgentLevelDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SalesAgentLevelDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SalesAgentLevelDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
