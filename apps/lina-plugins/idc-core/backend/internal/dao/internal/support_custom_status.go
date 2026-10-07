// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SupportCustomStatusDao is the data access object for the table plugin_idc_core_support_custom_status.
type SupportCustomStatusDao struct {
	table    string                     // table is the underlying table name of the DAO.
	group    string                     // group is the database configuration group name of the current DAO.
	columns  SupportCustomStatusColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler         // handlers for customized model modification.
}

// SupportCustomStatusColumns defines and stores column names for the table plugin_idc_core_support_custom_status.
type SupportCustomStatusColumns struct {
	Id         string //
	TenantId   string //
	StatusNo   string //
	NameZhCn   string //
	NameEnUs   string //
	BaseStage  string //
	SortOrder  string //
	StatusCode string //
	Version    string //
	CreatedBy  string //
	UpdatedBy  string //
	CreatedAt  string //
	UpdatedAt  string //
}

// supportCustomStatusColumns holds the columns for the table plugin_idc_core_support_custom_status.
var supportCustomStatusColumns = SupportCustomStatusColumns{
	Id:         "id",
	TenantId:   "tenant_id",
	StatusNo:   "status_no",
	NameZhCn:   "name_zh_cn",
	NameEnUs:   "name_en_us",
	BaseStage:  "base_stage",
	SortOrder:  "sort_order",
	StatusCode: "status_code",
	Version:    "version",
	CreatedBy:  "created_by",
	UpdatedBy:  "updated_by",
	CreatedAt:  "created_at",
	UpdatedAt:  "updated_at",
}

// NewSupportCustomStatusDao creates and returns a new DAO object for table data access.
func NewSupportCustomStatusDao(handlers ...gdb.ModelHandler) *SupportCustomStatusDao {
	return &SupportCustomStatusDao{
		group:    "default",
		table:    "plugin_idc_core_support_custom_status",
		columns:  supportCustomStatusColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SupportCustomStatusDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SupportCustomStatusDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SupportCustomStatusDao) Columns() SupportCustomStatusColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SupportCustomStatusDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SupportCustomStatusDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SupportCustomStatusDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
