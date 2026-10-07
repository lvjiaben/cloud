// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CustomerContactRoleDao is the data access object for the table plugin_idc_core_customer_contact_role.
type CustomerContactRoleDao struct {
	table    string                     // table is the underlying table name of the DAO.
	group    string                     // group is the database configuration group name of the current DAO.
	columns  CustomerContactRoleColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler         // handlers for customized model modification.
}

// CustomerContactRoleColumns defines and stores column names for the table plugin_idc_core_customer_contact_role.
type CustomerContactRoleColumns struct {
	Id         string //
	TenantId   string //
	CustomerNo string //
	ContactNo  string //
	RoleCode   string //
	StatusCode string //
	AssignedBy string //
	AssignedAt string //
	RevokedBy  string //
	RevokedAt  string //
	CreatedAt  string //
	UpdatedAt  string //
}

// customerContactRoleColumns holds the columns for the table plugin_idc_core_customer_contact_role.
var customerContactRoleColumns = CustomerContactRoleColumns{
	Id:         "id",
	TenantId:   "tenant_id",
	CustomerNo: "customer_no",
	ContactNo:  "contact_no",
	RoleCode:   "role_code",
	StatusCode: "status_code",
	AssignedBy: "assigned_by",
	AssignedAt: "assigned_at",
	RevokedBy:  "revoked_by",
	RevokedAt:  "revoked_at",
	CreatedAt:  "created_at",
	UpdatedAt:  "updated_at",
}

// NewCustomerContactRoleDao creates and returns a new DAO object for table data access.
func NewCustomerContactRoleDao(handlers ...gdb.ModelHandler) *CustomerContactRoleDao {
	return &CustomerContactRoleDao{
		group:    "default",
		table:    "plugin_idc_core_customer_contact_role",
		columns:  customerContactRoleColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CustomerContactRoleDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CustomerContactRoleDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CustomerContactRoleDao) Columns() CustomerContactRoleColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CustomerContactRoleDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CustomerContactRoleDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CustomerContactRoleDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
