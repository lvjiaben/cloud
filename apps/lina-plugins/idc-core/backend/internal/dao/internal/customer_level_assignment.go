// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CustomerLevelAssignmentDao is the data access object for the table plugin_idc_core_customer_level_assignment.
type CustomerLevelAssignmentDao struct {
	table    string                         // table is the underlying table name of the DAO.
	group    string                         // group is the database configuration group name of the current DAO.
	columns  CustomerLevelAssignmentColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler             // handlers for customized model modification.
}

// CustomerLevelAssignmentColumns defines and stores column names for the table plugin_idc_core_customer_level_assignment.
type CustomerLevelAssignmentColumns struct {
	Id         string //
	TenantId   string //
	CustomerNo string //
	LevelNo    string //
	SourceCode string //
	Reason     string //
	StatusCode string //
	AssignedBy string //
	AssignedAt string //
	RevokedBy  string //
	RevokedAt  string //
	CreatedAt  string //
	UpdatedAt  string //
}

// customerLevelAssignmentColumns holds the columns for the table plugin_idc_core_customer_level_assignment.
var customerLevelAssignmentColumns = CustomerLevelAssignmentColumns{
	Id:         "id",
	TenantId:   "tenant_id",
	CustomerNo: "customer_no",
	LevelNo:    "level_no",
	SourceCode: "source_code",
	Reason:     "reason",
	StatusCode: "status_code",
	AssignedBy: "assigned_by",
	AssignedAt: "assigned_at",
	RevokedBy:  "revoked_by",
	RevokedAt:  "revoked_at",
	CreatedAt:  "created_at",
	UpdatedAt:  "updated_at",
}

// NewCustomerLevelAssignmentDao creates and returns a new DAO object for table data access.
func NewCustomerLevelAssignmentDao(handlers ...gdb.ModelHandler) *CustomerLevelAssignmentDao {
	return &CustomerLevelAssignmentDao{
		group:    "default",
		table:    "plugin_idc_core_customer_level_assignment",
		columns:  customerLevelAssignmentColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CustomerLevelAssignmentDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CustomerLevelAssignmentDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CustomerLevelAssignmentDao) Columns() CustomerLevelAssignmentColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CustomerLevelAssignmentDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CustomerLevelAssignmentDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CustomerLevelAssignmentDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
