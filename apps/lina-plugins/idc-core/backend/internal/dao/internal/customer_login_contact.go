// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CustomerLoginContactDao is the data access object for the table plugin_idc_core_customer_login_contact.
type CustomerLoginContactDao struct {
	table    string                      // table is the underlying table name of the DAO.
	group    string                      // group is the database configuration group name of the current DAO.
	columns  CustomerLoginContactColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler          // handlers for customized model modification.
}

// CustomerLoginContactColumns defines and stores column names for the table plugin_idc_core_customer_login_contact.
type CustomerLoginContactColumns struct {
	TenantId           string //
	UserId             string //
	EmailDigest        string //
	PhoneDigest        string //
	IdentityCiphertext string //
	Revision           string //
	CreatedAt          string //
	UpdatedAt          string //
}

// customerLoginContactColumns holds the columns for the table plugin_idc_core_customer_login_contact.
var customerLoginContactColumns = CustomerLoginContactColumns{
	TenantId:           "tenant_id",
	UserId:             "user_id",
	EmailDigest:        "email_digest",
	PhoneDigest:        "phone_digest",
	IdentityCiphertext: "identity_ciphertext",
	Revision:           "revision",
	CreatedAt:          "created_at",
	UpdatedAt:          "updated_at",
}

// NewCustomerLoginContactDao creates and returns a new DAO object for table data access.
func NewCustomerLoginContactDao(handlers ...gdb.ModelHandler) *CustomerLoginContactDao {
	return &CustomerLoginContactDao{
		group:    "default",
		table:    "plugin_idc_core_customer_login_contact",
		columns:  customerLoginContactColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CustomerLoginContactDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CustomerLoginContactDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CustomerLoginContactDao) Columns() CustomerLoginContactColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CustomerLoginContactDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CustomerLoginContactDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CustomerLoginContactDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
