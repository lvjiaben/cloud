// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CustomerAddressDao is the data access object for the table plugin_idc_core_customer_address.
type CustomerAddressDao struct {
	table    string                 // table is the underlying table name of the DAO.
	group    string                 // group is the database configuration group name of the current DAO.
	columns  CustomerAddressColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler     // handlers for customized model modification.
}

// CustomerAddressColumns defines and stores column names for the table plugin_idc_core_customer_address.
type CustomerAddressColumns struct {
	Id              string //
	TenantId        string //
	AddressNo       string //
	CustomerNo      string //
	AddressTypeCode string //
	Label           string //
	RecipientName   string //
	RecipientPhone  string //
	CountryCode     string //
	Province        string //
	City            string //
	District        string //
	AddressLine1    string //
	AddressLine2    string //
	PostalCode      string //
	IsDefault       string //
	StatusCode      string //
	Version         string //
	CreatedBy       string //
	UpdatedBy       string //
	CreatedAt       string //
	UpdatedAt       string //
}

// customerAddressColumns holds the columns for the table plugin_idc_core_customer_address.
var customerAddressColumns = CustomerAddressColumns{
	Id:              "id",
	TenantId:        "tenant_id",
	AddressNo:       "address_no",
	CustomerNo:      "customer_no",
	AddressTypeCode: "address_type_code",
	Label:           "label",
	RecipientName:   "recipient_name",
	RecipientPhone:  "recipient_phone",
	CountryCode:     "country_code",
	Province:        "province",
	City:            "city",
	District:        "district",
	AddressLine1:    "address_line1",
	AddressLine2:    "address_line2",
	PostalCode:      "postal_code",
	IsDefault:       "is_default",
	StatusCode:      "status_code",
	Version:         "version",
	CreatedBy:       "created_by",
	UpdatedBy:       "updated_by",
	CreatedAt:       "created_at",
	UpdatedAt:       "updated_at",
}

// NewCustomerAddressDao creates and returns a new DAO object for table data access.
func NewCustomerAddressDao(handlers ...gdb.ModelHandler) *CustomerAddressDao {
	return &CustomerAddressDao{
		group:    "default",
		table:    "plugin_idc_core_customer_address",
		columns:  customerAddressColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CustomerAddressDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CustomerAddressDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CustomerAddressDao) Columns() CustomerAddressColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CustomerAddressDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CustomerAddressDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CustomerAddressDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
