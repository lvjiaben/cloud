// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CustomerFollowUpDao is the data access object for the table plugin_idc_core_customer_follow_up.
type CustomerFollowUpDao struct {
	table    string                  // table is the underlying table name of the DAO.
	group    string                  // group is the database configuration group name of the current DAO.
	columns  CustomerFollowUpColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler      // handlers for customized model modification.
}

// CustomerFollowUpColumns defines and stores column names for the table plugin_idc_core_customer_follow_up.
type CustomerFollowUpColumns struct {
	Id             string //
	TenantId       string //
	FollowUpNo     string //
	CustomerNo     string //
	KindCode       string //
	ChannelCode    string //
	StatusCode     string //
	Subject        string //
	Detail         string //
	OutcomeCode    string //
	AssignedUserId string //
	PlannedAt      string //
	CompletedAt    string //
	CancelledAt    string //
	CreatedBy      string //
	UpdatedBy      string //
	CreatedAt      string //
	UpdatedAt      string //
}

// customerFollowUpColumns holds the columns for the table plugin_idc_core_customer_follow_up.
var customerFollowUpColumns = CustomerFollowUpColumns{
	Id:             "id",
	TenantId:       "tenant_id",
	FollowUpNo:     "follow_up_no",
	CustomerNo:     "customer_no",
	KindCode:       "kind_code",
	ChannelCode:    "channel_code",
	StatusCode:     "status_code",
	Subject:        "subject",
	Detail:         "detail",
	OutcomeCode:    "outcome_code",
	AssignedUserId: "assigned_user_id",
	PlannedAt:      "planned_at",
	CompletedAt:    "completed_at",
	CancelledAt:    "cancelled_at",
	CreatedBy:      "created_by",
	UpdatedBy:      "updated_by",
	CreatedAt:      "created_at",
	UpdatedAt:      "updated_at",
}

// NewCustomerFollowUpDao creates and returns a new DAO object for table data access.
func NewCustomerFollowUpDao(handlers ...gdb.ModelHandler) *CustomerFollowUpDao {
	return &CustomerFollowUpDao{
		group:    "default",
		table:    "plugin_idc_core_customer_follow_up",
		columns:  customerFollowUpColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CustomerFollowUpDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CustomerFollowUpDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CustomerFollowUpDao) Columns() CustomerFollowUpColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CustomerFollowUpDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CustomerFollowUpDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CustomerFollowUpDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
