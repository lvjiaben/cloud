// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SupportReplyPresetDao is the data access object for the table plugin_idc_core_support_reply_preset.
type SupportReplyPresetDao struct {
	table    string                    // table is the underlying table name of the DAO.
	group    string                    // group is the database configuration group name of the current DAO.
	columns  SupportReplyPresetColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler        // handlers for customized model modification.
}

// SupportReplyPresetColumns defines and stores column names for the table plugin_idc_core_support_reply_preset.
type SupportReplyPresetColumns struct {
	Id           string //
	TenantId     string //
	PresetNo     string //
	DepartmentNo string //
	Locale       string //
	Title        string //
	Body         string //
	SortOrder    string //
	StatusCode   string //
	Version      string //
	CreatedBy    string //
	UpdatedBy    string //
	CreatedAt    string //
	UpdatedAt    string //
}

// supportReplyPresetColumns holds the columns for the table plugin_idc_core_support_reply_preset.
var supportReplyPresetColumns = SupportReplyPresetColumns{
	Id:           "id",
	TenantId:     "tenant_id",
	PresetNo:     "preset_no",
	DepartmentNo: "department_no",
	Locale:       "locale",
	Title:        "title",
	Body:         "body",
	SortOrder:    "sort_order",
	StatusCode:   "status_code",
	Version:      "version",
	CreatedBy:    "created_by",
	UpdatedBy:    "updated_by",
	CreatedAt:    "created_at",
	UpdatedAt:    "updated_at",
}

// NewSupportReplyPresetDao creates and returns a new DAO object for table data access.
func NewSupportReplyPresetDao(handlers ...gdb.ModelHandler) *SupportReplyPresetDao {
	return &SupportReplyPresetDao{
		group:    "default",
		table:    "plugin_idc_core_support_reply_preset",
		columns:  supportReplyPresetColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SupportReplyPresetDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SupportReplyPresetDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SupportReplyPresetDao) Columns() SupportReplyPresetColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SupportReplyPresetDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SupportReplyPresetDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SupportReplyPresetDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
