// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CustomerContactPreferenceDao is the data access object for the table plugin_idc_core_customer_contact_preference.
type CustomerContactPreferenceDao struct {
	table    string                           // table is the underlying table name of the DAO.
	group    string                           // group is the database configuration group name of the current DAO.
	columns  CustomerContactPreferenceColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler               // handlers for customized model modification.
}

// CustomerContactPreferenceColumns defines and stores column names for the table plugin_idc_core_customer_contact_preference.
type CustomerContactPreferenceColumns struct {
	Id          string //
	TenantId    string //
	CustomerNo  string //
	ContactNo   string //
	TopicCode   string //
	ChannelCode string //
	Enabled     string //
	Version     string //
	CreatedAt   string //
	UpdatedAt   string //
}

// customerContactPreferenceColumns holds the columns for the table plugin_idc_core_customer_contact_preference.
var customerContactPreferenceColumns = CustomerContactPreferenceColumns{
	Id:          "id",
	TenantId:    "tenant_id",
	CustomerNo:  "customer_no",
	ContactNo:   "contact_no",
	TopicCode:   "topic_code",
	ChannelCode: "channel_code",
	Enabled:     "enabled",
	Version:     "version",
	CreatedAt:   "created_at",
	UpdatedAt:   "updated_at",
}

// NewCustomerContactPreferenceDao creates and returns a new DAO object for table data access.
func NewCustomerContactPreferenceDao(handlers ...gdb.ModelHandler) *CustomerContactPreferenceDao {
	return &CustomerContactPreferenceDao{
		group:    "default",
		table:    "plugin_idc_core_customer_contact_preference",
		columns:  customerContactPreferenceColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CustomerContactPreferenceDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CustomerContactPreferenceDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CustomerContactPreferenceDao) Columns() CustomerContactPreferenceColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CustomerContactPreferenceDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CustomerContactPreferenceDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CustomerContactPreferenceDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
