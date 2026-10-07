// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CustomerIndividualProfileDao is the data access object for the table plugin_idc_core_customer_individual_profile.
type CustomerIndividualProfileDao struct {
	table    string                           // table is the underlying table name of the DAO.
	group    string                           // group is the database configuration group name of the current DAO.
	columns  CustomerIndividualProfileColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler               // handlers for customized model modification.
}

// CustomerIndividualProfileColumns defines and stores column names for the table plugin_idc_core_customer_individual_profile.
type CustomerIndividualProfileColumns struct {
	Id                       string //
	TenantId                 string //
	CustomerNo               string //
	CustomerTypeCode         string //
	LegalName                string // Legal name protected by customer-profile access controls
	LegalNameMasked          string //
	IdentityIdentifierMasked string // Display-safe masked identity identifier; the raw identifier is not stored in this table
	IdentityLookupDigest     string // Keyed lookup digest; never a raw identity number or unsalted public hash
	IdentityCaseNo           string // Opaque compliance-owned verification case reference without a database foreign key
	IdentityStatusCode       string //
	IdentityVerifiedAt       string //
	IdentityExpiresAt        string //
	Version                  string //
	CreatedAt                string //
	UpdatedAt                string //
}

// customerIndividualProfileColumns holds the columns for the table plugin_idc_core_customer_individual_profile.
var customerIndividualProfileColumns = CustomerIndividualProfileColumns{
	Id:                       "id",
	TenantId:                 "tenant_id",
	CustomerNo:               "customer_no",
	CustomerTypeCode:         "customer_type_code",
	LegalName:                "legal_name",
	LegalNameMasked:          "legal_name_masked",
	IdentityIdentifierMasked: "identity_identifier_masked",
	IdentityLookupDigest:     "identity_lookup_digest",
	IdentityCaseNo:           "identity_case_no",
	IdentityStatusCode:       "identity_status_code",
	IdentityVerifiedAt:       "identity_verified_at",
	IdentityExpiresAt:        "identity_expires_at",
	Version:                  "version",
	CreatedAt:                "created_at",
	UpdatedAt:                "updated_at",
}

// NewCustomerIndividualProfileDao creates and returns a new DAO object for table data access.
func NewCustomerIndividualProfileDao(handlers ...gdb.ModelHandler) *CustomerIndividualProfileDao {
	return &CustomerIndividualProfileDao{
		group:    "default",
		table:    "plugin_idc_core_customer_individual_profile",
		columns:  customerIndividualProfileColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CustomerIndividualProfileDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CustomerIndividualProfileDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CustomerIndividualProfileDao) Columns() CustomerIndividualProfileColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CustomerIndividualProfileDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CustomerIndividualProfileDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CustomerIndividualProfileDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
