// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CommissionAccrualDao is the data access object for the table plugin_idc_core_commission_accrual.
type CommissionAccrualDao struct {
	table    string                   // table is the underlying table name of the DAO.
	group    string                   // group is the database configuration group name of the current DAO.
	columns  CommissionAccrualColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler       // handlers for customized model modification.
}

// CommissionAccrualColumns defines and stores column names for the table plugin_idc_core_commission_accrual.
type CommissionAccrualColumns struct {
	Id                     string //
	TenantId               string //
	CommissionNo           string //
	OrderNo                string //
	InvoiceNo              string //
	InvoiceItemNo          string //
	BeneficiaryTypeCode    string //
	BeneficiaryReference   string //
	CurrencyCode           string //
	RuleVersion            string //
	BasisAmountMinor       string //
	InvoiceItemAmountMinor string //
	OriginalAmountMinor    string //
	PendingAmountMinor     string //
	MaturedAmountMinor     string //
	ReversedAmountMinor    string //
	StatusCode             string //
	MaturesAt              string //
	CreatedAt              string //
	UpdatedAt              string //
	AgentQualificationNo   string //
	AffiliateAttributionNo string //
	SalesAssignmentNo      string //
	CommissionRuleNo       string //
	CommissionTierNo       string //
	OperationTypeCode      string //
}

// commissionAccrualColumns holds the columns for the table plugin_idc_core_commission_accrual.
var commissionAccrualColumns = CommissionAccrualColumns{
	Id:                     "id",
	TenantId:               "tenant_id",
	CommissionNo:           "commission_no",
	OrderNo:                "order_no",
	InvoiceNo:              "invoice_no",
	InvoiceItemNo:          "invoice_item_no",
	BeneficiaryTypeCode:    "beneficiary_type_code",
	BeneficiaryReference:   "beneficiary_reference",
	CurrencyCode:           "currency_code",
	RuleVersion:            "rule_version",
	BasisAmountMinor:       "basis_amount_minor",
	InvoiceItemAmountMinor: "invoice_item_amount_minor",
	OriginalAmountMinor:    "original_amount_minor",
	PendingAmountMinor:     "pending_amount_minor",
	MaturedAmountMinor:     "matured_amount_minor",
	ReversedAmountMinor:    "reversed_amount_minor",
	StatusCode:             "status_code",
	MaturesAt:              "matures_at",
	CreatedAt:              "created_at",
	UpdatedAt:              "updated_at",
	AgentQualificationNo:   "agent_qualification_no",
	AffiliateAttributionNo: "affiliate_attribution_no",
	SalesAssignmentNo:      "sales_assignment_no",
	CommissionRuleNo:       "commission_rule_no",
	CommissionTierNo:       "commission_tier_no",
	OperationTypeCode:      "operation_type_code",
}

// NewCommissionAccrualDao creates and returns a new DAO object for table data access.
func NewCommissionAccrualDao(handlers ...gdb.ModelHandler) *CommissionAccrualDao {
	return &CommissionAccrualDao{
		group:    "default",
		table:    "plugin_idc_core_commission_accrual",
		columns:  commissionAccrualColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CommissionAccrualDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CommissionAccrualDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CommissionAccrualDao) Columns() CommissionAccrualColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CommissionAccrualDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CommissionAccrualDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CommissionAccrualDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
