// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SupportSlaHolidayDao is the data access object for the table plugin_idc_core_support_sla_holiday.
type SupportSlaHolidayDao struct {
	table    string                   // table is the underlying table name of the DAO.
	group    string                   // group is the database configuration group name of the current DAO.
	columns  SupportSlaHolidayColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler       // handlers for customized model modification.
}

// SupportSlaHolidayColumns defines and stores column names for the table plugin_idc_core_support_sla_holiday.
type SupportSlaHolidayColumns struct {
	Id          string //
	TenantId    string //
	CalendarNo  string //
	HolidayDate string //
	Name        string //
	CreatedAt   string //
}

// supportSlaHolidayColumns holds the columns for the table plugin_idc_core_support_sla_holiday.
var supportSlaHolidayColumns = SupportSlaHolidayColumns{
	Id:          "id",
	TenantId:    "tenant_id",
	CalendarNo:  "calendar_no",
	HolidayDate: "holiday_date",
	Name:        "name",
	CreatedAt:   "created_at",
}

// NewSupportSlaHolidayDao creates and returns a new DAO object for table data access.
func NewSupportSlaHolidayDao(handlers ...gdb.ModelHandler) *SupportSlaHolidayDao {
	return &SupportSlaHolidayDao{
		group:    "default",
		table:    "plugin_idc_core_support_sla_holiday",
		columns:  supportSlaHolidayColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SupportSlaHolidayDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SupportSlaHolidayDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SupportSlaHolidayDao) Columns() SupportSlaHolidayColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SupportSlaHolidayDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SupportSlaHolidayDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SupportSlaHolidayDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
