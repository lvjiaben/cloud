// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SupportSlaRuleDao is the data access object for the table plugin_idc_core_support_sla_rule.
type SupportSlaRuleDao struct {
	table    string                // table is the underlying table name of the DAO.
	group    string                // group is the database configuration group name of the current DAO.
	columns  SupportSlaRuleColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler    // handlers for customized model modification.
}

// SupportSlaRuleColumns defines and stores column names for the table plugin_idc_core_support_sla_rule.
type SupportSlaRuleColumns struct {
	Id                   string //
	TenantId             string //
	RuleNo               string //
	DepartmentNo         string //
	CategoryNo           string //
	PriorityCode         string //
	CalendarNo           string //
	FirstResponseMinutes string //
	ResolutionMinutes    string //
	NearDueMinutes       string //
	StatusCode           string //
	Version              string //
	CreatedBy            string //
	UpdatedBy            string //
	CreatedAt            string //
	UpdatedAt            string //
}

// supportSlaRuleColumns holds the columns for the table plugin_idc_core_support_sla_rule.
var supportSlaRuleColumns = SupportSlaRuleColumns{
	Id:                   "id",
	TenantId:             "tenant_id",
	RuleNo:               "rule_no",
	DepartmentNo:         "department_no",
	CategoryNo:           "category_no",
	PriorityCode:         "priority_code",
	CalendarNo:           "calendar_no",
	FirstResponseMinutes: "first_response_minutes",
	ResolutionMinutes:    "resolution_minutes",
	NearDueMinutes:       "near_due_minutes",
	StatusCode:           "status_code",
	Version:              "version",
	CreatedBy:            "created_by",
	UpdatedBy:            "updated_by",
	CreatedAt:            "created_at",
	UpdatedAt:            "updated_at",
}

// NewSupportSlaRuleDao creates and returns a new DAO object for table data access.
func NewSupportSlaRuleDao(handlers ...gdb.ModelHandler) *SupportSlaRuleDao {
	return &SupportSlaRuleDao{
		group:    "default",
		table:    "plugin_idc_core_support_sla_rule",
		columns:  supportSlaRuleColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SupportSlaRuleDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SupportSlaRuleDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SupportSlaRuleDao) Columns() SupportSlaRuleColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SupportSlaRuleDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SupportSlaRuleDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SupportSlaRuleDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
