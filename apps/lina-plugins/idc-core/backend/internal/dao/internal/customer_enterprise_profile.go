// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CustomerEnterpriseProfileDao is the data access object for the table plugin_idc_core_customer_enterprise_profile.
type CustomerEnterpriseProfileDao struct {
	table    string                           // table is the underlying table name of the DAO.
	group    string                           // group is the database configuration group name of the current DAO.
	columns  CustomerEnterpriseProfileColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler               // handlers for customized model modification.
}

// CustomerEnterpriseProfileColumns defines and stores column names for the table plugin_idc_core_customer_enterprise_profile.
type CustomerEnterpriseProfileColumns struct {
	Id                           string //
	TenantId                     string //
	CustomerNo                   string //
	CustomerTypeCode             string //
	LegalName                    string //
	RegistrationCountryCode      string //
	RegistrationIdentifier       string // Enterprise registration identifier protected as customer legal profile data
	RegistrationIdentifierMasked string //
	RegistrationLookupDigest     string // Keyed lookup digest used for duplicate review without query-time normalization
	IndustryCode                 string //
	VerificationCaseNo           string // Opaque compliance-owned verification case reference without a database foreign key
	VerificationStatusCode       string //
	VerifiedAt                   string //
	ExpiresAt                    string //
	Version                      string //
	CreatedAt                    string //
	UpdatedAt                    string //
}

// customerEnterpriseProfileColumns holds the columns for the table plugin_idc_core_customer_enterprise_profile.
var customerEnterpriseProfileColumns = CustomerEnterpriseProfileColumns{
	Id:                           "id",
	TenantId:                     "tenant_id",
	CustomerNo:                   "customer_no",
	CustomerTypeCode:             "customer_type_code",
	LegalName:                    "legal_name",
	RegistrationCountryCode:      "registration_country_code",
	RegistrationIdentifier:       "registration_identifier",
	RegistrationIdentifierMasked: "registration_identifier_masked",
	RegistrationLookupDigest:     "registration_lookup_digest",
	IndustryCode:                 "industry_code",
	VerificationCaseNo:           "verification_case_no",
	VerificationStatusCode:       "verification_status_code",
	VerifiedAt:                   "verified_at",
	ExpiresAt:                    "expires_at",
	Version:                      "version",
	CreatedAt:                    "created_at",
	UpdatedAt:                    "updated_at",
}

// NewCustomerEnterpriseProfileDao creates and returns a new DAO object for table data access.
func NewCustomerEnterpriseProfileDao(handlers ...gdb.ModelHandler) *CustomerEnterpriseProfileDao {
	return &CustomerEnterpriseProfileDao{
		group:    "default",
		table:    "plugin_idc_core_customer_enterprise_profile",
		columns:  customerEnterpriseProfileColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CustomerEnterpriseProfileDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CustomerEnterpriseProfileDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CustomerEnterpriseProfileDao) Columns() CustomerEnterpriseProfileColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CustomerEnterpriseProfileDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CustomerEnterpriseProfileDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CustomerEnterpriseProfileDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
