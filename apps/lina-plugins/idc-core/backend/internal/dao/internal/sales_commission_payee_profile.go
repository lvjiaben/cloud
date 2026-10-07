// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SalesCommissionPayeeProfileDao is the data access object for the table plugin_idc_core_sales_commission_payee_profile.
type SalesCommissionPayeeProfileDao struct {
	table    string                             // table is the underlying table name of the DAO.
	group    string                             // group is the database configuration group name of the current DAO.
	columns  SalesCommissionPayeeProfileColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                 // handlers for customized model modification.
}

// SalesCommissionPayeeProfileColumns defines and stores column names for the table plugin_idc_core_sales_commission_payee_profile.
type SalesCommissionPayeeProfileColumns struct {
	Id                       string //
	TenantId                 string //
	ProfileNo                string //
	BeneficiaryTypeCode      string //
	BeneficiaryReference     string //
	PayeeTypeCode            string //
	ProtectedObjectReference string //
	AccountDigest            string //
	StatusCode               string //
	Version                  string //
	VerifiedAt               string //
	CreatedAt                string //
	UpdatedAt                string //
}

// salesCommissionPayeeProfileColumns holds the columns for the table plugin_idc_core_sales_commission_payee_profile.
var salesCommissionPayeeProfileColumns = SalesCommissionPayeeProfileColumns{
	Id:                       "id",
	TenantId:                 "tenant_id",
	ProfileNo:                "profile_no",
	BeneficiaryTypeCode:      "beneficiary_type_code",
	BeneficiaryReference:     "beneficiary_reference",
	PayeeTypeCode:            "payee_type_code",
	ProtectedObjectReference: "protected_object_reference",
	AccountDigest:            "account_digest",
	StatusCode:               "status_code",
	Version:                  "version",
	VerifiedAt:               "verified_at",
	CreatedAt:                "created_at",
	UpdatedAt:                "updated_at",
}

// NewSalesCommissionPayeeProfileDao creates and returns a new DAO object for table data access.
func NewSalesCommissionPayeeProfileDao(handlers ...gdb.ModelHandler) *SalesCommissionPayeeProfileDao {
	return &SalesCommissionPayeeProfileDao{
		group:    "default",
		table:    "plugin_idc_core_sales_commission_payee_profile",
		columns:  salesCommissionPayeeProfileColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SalesCommissionPayeeProfileDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SalesCommissionPayeeProfileDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SalesCommissionPayeeProfileDao) Columns() SalesCommissionPayeeProfileColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SalesCommissionPayeeProfileDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SalesCommissionPayeeProfileDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SalesCommissionPayeeProfileDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
