// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SupportSlaCalendarDao is the data access object for the table plugin_idc_core_support_sla_calendar.
type SupportSlaCalendarDao struct {
	table    string                    // table is the underlying table name of the DAO.
	group    string                    // group is the database configuration group name of the current DAO.
	columns  SupportSlaCalendarColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler        // handlers for customized model modification.
}

// SupportSlaCalendarColumns defines and stores column names for the table plugin_idc_core_support_sla_calendar.
type SupportSlaCalendarColumns struct {
	Id              string //
	TenantId        string //
	CalendarNo      string //
	Name            string //
	ModeCode        string //
	TimezoneName    string //
	Workdays        string //
	WorkStartMinute string //
	WorkEndMinute   string //
	StatusCode      string //
	Version         string //
	CreatedBy       string //
	UpdatedBy       string //
	CreatedAt       string //
	UpdatedAt       string //
}

// supportSlaCalendarColumns holds the columns for the table plugin_idc_core_support_sla_calendar.
var supportSlaCalendarColumns = SupportSlaCalendarColumns{
	Id:              "id",
	TenantId:        "tenant_id",
	CalendarNo:      "calendar_no",
	Name:            "name",
	ModeCode:        "mode_code",
	TimezoneName:    "timezone_name",
	Workdays:        "workdays",
	WorkStartMinute: "work_start_minute",
	WorkEndMinute:   "work_end_minute",
	StatusCode:      "status_code",
	Version:         "version",
	CreatedBy:       "created_by",
	UpdatedBy:       "updated_by",
	CreatedAt:       "created_at",
	UpdatedAt:       "updated_at",
}

// NewSupportSlaCalendarDao creates and returns a new DAO object for table data access.
func NewSupportSlaCalendarDao(handlers ...gdb.ModelHandler) *SupportSlaCalendarDao {
	return &SupportSlaCalendarDao{
		group:    "default",
		table:    "plugin_idc_core_support_sla_calendar",
		columns:  supportSlaCalendarColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SupportSlaCalendarDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SupportSlaCalendarDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SupportSlaCalendarDao) Columns() SupportSlaCalendarColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SupportSlaCalendarDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SupportSlaCalendarDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SupportSlaCalendarDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
