// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SalesPaymentOwnerDao is the data access object for the table plugin_idc_core_sales_payment_owner.
type SalesPaymentOwnerDao struct {
	table    string                   // table is the underlying table name of the DAO.
	group    string                   // group is the database configuration group name of the current DAO.
	columns  SalesPaymentOwnerColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler       // handlers for customized model modification.
}

// SalesPaymentOwnerColumns defines and stores column names for the table plugin_idc_core_sales_payment_owner.
type SalesPaymentOwnerColumns struct {
	Id                     string //
	TenantId               string //
	InvoiceNo              string //
	BeneficiaryTypeCode    string //
	BeneficiaryReference   string //
	SalesAssignmentNo      string //
	AffiliateAttributionNo string //
	ManualReferrerId       string //
	CreatedAt              string //
}

// salesPaymentOwnerColumns holds the columns for the table plugin_idc_core_sales_payment_owner.
var salesPaymentOwnerColumns = SalesPaymentOwnerColumns{
	Id:                     "id",
	TenantId:               "tenant_id",
	InvoiceNo:              "invoice_no",
	BeneficiaryTypeCode:    "beneficiary_type_code",
	BeneficiaryReference:   "beneficiary_reference",
	SalesAssignmentNo:      "sales_assignment_no",
	AffiliateAttributionNo: "affiliate_attribution_no",
	ManualReferrerId:       "manual_referrer_id",
	CreatedAt:              "created_at",
}

// NewSalesPaymentOwnerDao creates and returns a new DAO object for table data access.
func NewSalesPaymentOwnerDao(handlers ...gdb.ModelHandler) *SalesPaymentOwnerDao {
	return &SalesPaymentOwnerDao{
		group:    "default",
		table:    "plugin_idc_core_sales_payment_owner",
		columns:  salesPaymentOwnerColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SalesPaymentOwnerDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SalesPaymentOwnerDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SalesPaymentOwnerDao) Columns() SalesPaymentOwnerColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SalesPaymentOwnerDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SalesPaymentOwnerDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SalesPaymentOwnerDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
