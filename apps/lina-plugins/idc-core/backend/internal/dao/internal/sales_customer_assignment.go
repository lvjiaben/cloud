// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SalesCustomerAssignmentDao is the data access object for the table plugin_idc_core_sales_customer_assignment.
type SalesCustomerAssignmentDao struct {
	table    string                         // table is the underlying table name of the DAO.
	group    string                         // group is the database configuration group name of the current DAO.
	columns  SalesCustomerAssignmentColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler             // handlers for customized model modification.
}

// SalesCustomerAssignmentColumns defines and stores column names for the table plugin_idc_core_sales_customer_assignment.
type SalesCustomerAssignmentColumns struct {
	Id                  string //
	TenantId            string //
	AssignmentNo        string //
	CustomerNo          string //
	MembershipNo        string //
	ProtectionExpiresAt string //
	EffectiveAt         string //
	EndedAt             string //
	StatusCode          string //
	SourceCode          string //
	CreatedAt           string //
}

// salesCustomerAssignmentColumns holds the columns for the table plugin_idc_core_sales_customer_assignment.
var salesCustomerAssignmentColumns = SalesCustomerAssignmentColumns{
	Id:                  "id",
	TenantId:            "tenant_id",
	AssignmentNo:        "assignment_no",
	CustomerNo:          "customer_no",
	MembershipNo:        "membership_no",
	ProtectionExpiresAt: "protection_expires_at",
	EffectiveAt:         "effective_at",
	EndedAt:             "ended_at",
	StatusCode:          "status_code",
	SourceCode:          "source_code",
	CreatedAt:           "created_at",
}

// NewSalesCustomerAssignmentDao creates and returns a new DAO object for table data access.
func NewSalesCustomerAssignmentDao(handlers ...gdb.ModelHandler) *SalesCustomerAssignmentDao {
	return &SalesCustomerAssignmentDao{
		group:    "default",
		table:    "plugin_idc_core_sales_customer_assignment",
		columns:  salesCustomerAssignmentColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SalesCustomerAssignmentDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SalesCustomerAssignmentDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SalesCustomerAssignmentDao) Columns() SalesCustomerAssignmentColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SalesCustomerAssignmentDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SalesCustomerAssignmentDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SalesCustomerAssignmentDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
