// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CustomerContactDao is the data access object for the table plugin_idc_core_customer_contact.
type CustomerContactDao struct {
	table    string                 // table is the underlying table name of the DAO.
	group    string                 // group is the database configuration group name of the current DAO.
	columns  CustomerContactColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler     // handlers for customized model modification.
}

// CustomerContactColumns defines and stores column names for the table plugin_idc_core_customer_contact.
type CustomerContactColumns struct {
	Id                string //
	TenantId          string //
	ContactNo         string //
	CustomerNo        string //
	DisplayName       string //
	Email             string //
	EmailLookupDigest string // Keyed normalized email lookup digest
	EmailVerifiedAt   string //
	PhoneCountryCode  string //
	Phone             string //
	PhoneLookupDigest string // Keyed E.164-style phone lookup digest
	PhoneVerifiedAt   string //
	PreferredLocale   string //
	StatusCode        string //
	Version           string //
	CreatedBy         string //
	UpdatedBy         string //
	CreatedAt         string //
	UpdatedAt         string //
}

// customerContactColumns holds the columns for the table plugin_idc_core_customer_contact.
var customerContactColumns = CustomerContactColumns{
	Id:                "id",
	TenantId:          "tenant_id",
	ContactNo:         "contact_no",
	CustomerNo:        "customer_no",
	DisplayName:       "display_name",
	Email:             "email",
	EmailLookupDigest: "email_lookup_digest",
	EmailVerifiedAt:   "email_verified_at",
	PhoneCountryCode:  "phone_country_code",
	Phone:             "phone",
	PhoneLookupDigest: "phone_lookup_digest",
	PhoneVerifiedAt:   "phone_verified_at",
	PreferredLocale:   "preferred_locale",
	StatusCode:        "status_code",
	Version:           "version",
	CreatedBy:         "created_by",
	UpdatedBy:         "updated_by",
	CreatedAt:         "created_at",
	UpdatedAt:         "updated_at",
}

// NewCustomerContactDao creates and returns a new DAO object for table data access.
func NewCustomerContactDao(handlers ...gdb.ModelHandler) *CustomerContactDao {
	return &CustomerContactDao{
		group:    "default",
		table:    "plugin_idc_core_customer_contact",
		columns:  customerContactColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CustomerContactDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CustomerContactDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CustomerContactDao) Columns() CustomerContactColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CustomerContactDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CustomerContactDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CustomerContactDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
