// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SalesCommissionPayoutItemDao is the data access object for the table plugin_idc_core_sales_commission_payout_item.
type SalesCommissionPayoutItemDao struct {
	table    string                           // table is the underlying table name of the DAO.
	group    string                           // group is the database configuration group name of the current DAO.
	columns  SalesCommissionPayoutItemColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler               // handlers for customized model modification.
}

// SalesCommissionPayoutItemColumns defines and stores column names for the table plugin_idc_core_sales_commission_payout_item.
type SalesCommissionPayoutItemColumns struct {
	Id                string //
	TenantId          string //
	ItemNo            string //
	BatchNo           string //
	SettlementNo      string //
	CurrencyCode      string //
	AmountMinor       string //
	StatusCode        string //
	AttemptNo         string //
	FailureReasonCode string //
	ProcessedAt       string //
}

// salesCommissionPayoutItemColumns holds the columns for the table plugin_idc_core_sales_commission_payout_item.
var salesCommissionPayoutItemColumns = SalesCommissionPayoutItemColumns{
	Id:                "id",
	TenantId:          "tenant_id",
	ItemNo:            "item_no",
	BatchNo:           "batch_no",
	SettlementNo:      "settlement_no",
	CurrencyCode:      "currency_code",
	AmountMinor:       "amount_minor",
	StatusCode:        "status_code",
	AttemptNo:         "attempt_no",
	FailureReasonCode: "failure_reason_code",
	ProcessedAt:       "processed_at",
}

// NewSalesCommissionPayoutItemDao creates and returns a new DAO object for table data access.
func NewSalesCommissionPayoutItemDao(handlers ...gdb.ModelHandler) *SalesCommissionPayoutItemDao {
	return &SalesCommissionPayoutItemDao{
		group:    "default",
		table:    "plugin_idc_core_sales_commission_payout_item",
		columns:  salesCommissionPayoutItemColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SalesCommissionPayoutItemDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SalesCommissionPayoutItemDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SalesCommissionPayoutItemDao) Columns() SalesCommissionPayoutItemColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SalesCommissionPayoutItemDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SalesCommissionPayoutItemDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SalesCommissionPayoutItemDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
