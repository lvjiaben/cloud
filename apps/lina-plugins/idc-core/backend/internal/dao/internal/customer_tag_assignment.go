// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CustomerTagAssignmentDao is the data access object for the table plugin_idc_core_customer_tag_assignment.
type CustomerTagAssignmentDao struct {
	table    string                       // table is the underlying table name of the DAO.
	group    string                       // group is the database configuration group name of the current DAO.
	columns  CustomerTagAssignmentColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler           // handlers for customized model modification.
}

// CustomerTagAssignmentColumns defines and stores column names for the table plugin_idc_core_customer_tag_assignment.
type CustomerTagAssignmentColumns struct {
	Id         string //
	TenantId   string //
	CustomerNo string //
	TagNo      string //
	SourceCode string //
	StatusCode string //
	AssignedBy string //
	AssignedAt string //
	RevokedBy  string //
	RevokedAt  string //
	CreatedAt  string //
	UpdatedAt  string //
}

// customerTagAssignmentColumns holds the columns for the table plugin_idc_core_customer_tag_assignment.
var customerTagAssignmentColumns = CustomerTagAssignmentColumns{
	Id:         "id",
	TenantId:   "tenant_id",
	CustomerNo: "customer_no",
	TagNo:      "tag_no",
	SourceCode: "source_code",
	StatusCode: "status_code",
	AssignedBy: "assigned_by",
	AssignedAt: "assigned_at",
	RevokedBy:  "revoked_by",
	RevokedAt:  "revoked_at",
	CreatedAt:  "created_at",
	UpdatedAt:  "updated_at",
}

// NewCustomerTagAssignmentDao creates and returns a new DAO object for table data access.
func NewCustomerTagAssignmentDao(handlers ...gdb.ModelHandler) *CustomerTagAssignmentDao {
	return &CustomerTagAssignmentDao{
		group:    "default",
		table:    "plugin_idc_core_customer_tag_assignment",
		columns:  customerTagAssignmentColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CustomerTagAssignmentDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CustomerTagAssignmentDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CustomerTagAssignmentDao) Columns() CustomerTagAssignmentColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CustomerTagAssignmentDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CustomerTagAssignmentDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CustomerTagAssignmentDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
