// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// MerchantProfileDao is the data access object for the table plugin_idc_core_merchant_profile.
type MerchantProfileDao struct {
	table    string                 // table is the underlying table name of the DAO.
	group    string                 // group is the database configuration group name of the current DAO.
	columns  MerchantProfileColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler     // handlers for customized model modification.
}

// MerchantProfileColumns defines and stores column names for the table plugin_idc_core_merchant_profile.
type MerchantProfileColumns struct {
	Id              string //
	TenantId        string // Host tenant context; the single-merchant edition requires tenant 0
	MerchantNo      string // Opaque public merchant business number
	DisplayName     string // Merchant display name
	LegalName       string // Merchant legal entity name
	CountryCode     string // ISO 3166-1 alpha-2 country code
	DefaultLocale   string // Default IETF locale tag
	DefaultCurrency string // Default ISO 4217 currency code
	Timezone        string // Default IANA time zone
	Enabled         string // Whether the merchant operating context accepts normal business traffic
	Version         string // Optimistic concurrency version
	CreatedBy       string //
	UpdatedBy       string //
	CreatedAt       string //
	UpdatedAt       string //
}

// merchantProfileColumns holds the columns for the table plugin_idc_core_merchant_profile.
var merchantProfileColumns = MerchantProfileColumns{
	Id:              "id",
	TenantId:        "tenant_id",
	MerchantNo:      "merchant_no",
	DisplayName:     "display_name",
	LegalName:       "legal_name",
	CountryCode:     "country_code",
	DefaultLocale:   "default_locale",
	DefaultCurrency: "default_currency",
	Timezone:        "timezone",
	Enabled:         "enabled",
	Version:         "version",
	CreatedBy:       "created_by",
	UpdatedBy:       "updated_by",
	CreatedAt:       "created_at",
	UpdatedAt:       "updated_at",
}

// NewMerchantProfileDao creates and returns a new DAO object for table data access.
func NewMerchantProfileDao(handlers ...gdb.ModelHandler) *MerchantProfileDao {
	return &MerchantProfileDao{
		group:    "default",
		table:    "plugin_idc_core_merchant_profile",
		columns:  merchantProfileColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *MerchantProfileDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *MerchantProfileDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *MerchantProfileDao) Columns() MerchantProfileColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *MerchantProfileDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *MerchantProfileDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *MerchantProfileDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
