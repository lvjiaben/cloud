// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SalesCommissionSettlementItemDao is the data access object for the table plugin_idc_core_sales_commission_settlement_item.
type SalesCommissionSettlementItemDao struct {
	table    string                               // table is the underlying table name of the DAO.
	group    string                               // group is the database configuration group name of the current DAO.
	columns  SalesCommissionSettlementItemColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                   // handlers for customized model modification.
}

// SalesCommissionSettlementItemColumns defines and stores column names for the table plugin_idc_core_sales_commission_settlement_item.
type SalesCommissionSettlementItemColumns struct {
	Id                   string //
	TenantId             string //
	ItemNo               string //
	SettlementNo         string //
	CommissionNo         string //
	CurrencyCode         string //
	BeneficiaryTypeCode  string //
	BeneficiaryReference string //
	AmountMinor          string //
	CreatedAt            string //
}

// salesCommissionSettlementItemColumns holds the columns for the table plugin_idc_core_sales_commission_settlement_item.
var salesCommissionSettlementItemColumns = SalesCommissionSettlementItemColumns{
	Id:                   "id",
	TenantId:             "tenant_id",
	ItemNo:               "item_no",
	SettlementNo:         "settlement_no",
	CommissionNo:         "commission_no",
	CurrencyCode:         "currency_code",
	BeneficiaryTypeCode:  "beneficiary_type_code",
	BeneficiaryReference: "beneficiary_reference",
	AmountMinor:          "amount_minor",
	CreatedAt:            "created_at",
}

// NewSalesCommissionSettlementItemDao creates and returns a new DAO object for table data access.
func NewSalesCommissionSettlementItemDao(handlers ...gdb.ModelHandler) *SalesCommissionSettlementItemDao {
	return &SalesCommissionSettlementItemDao{
		group:    "default",
		table:    "plugin_idc_core_sales_commission_settlement_item",
		columns:  salesCommissionSettlementItemColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SalesCommissionSettlementItemDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SalesCommissionSettlementItemDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SalesCommissionSettlementItemDao) Columns() SalesCommissionSettlementItemColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SalesCommissionSettlementItemDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SalesCommissionSettlementItemDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SalesCommissionSettlementItemDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
