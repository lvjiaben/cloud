// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SalesCommissionOfflinePayoutDao is the data access object for the table plugin_idc_core_sales_commission_offline_payout.
type SalesCommissionOfflinePayoutDao struct {
	table    string                              // table is the underlying table name of the DAO.
	group    string                              // group is the database configuration group name of the current DAO.
	columns  SalesCommissionOfflinePayoutColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                  // handlers for customized model modification.
}

// SalesCommissionOfflinePayoutColumns defines and stores column names for the table plugin_idc_core_sales_commission_offline_payout.
type SalesCommissionOfflinePayoutColumns struct {
	Id                string //
	TenantId          string //
	PayoutNo          string //
	AccountNo         string //
	CurrencyCode      string //
	AmountMinor       string //
	EvidenceReference string //
	PaidAt            string //
	RecordedBy        string //
	RequestDigest     string //
	CreatedAt         string //
}

// salesCommissionOfflinePayoutColumns holds the columns for the table plugin_idc_core_sales_commission_offline_payout.
var salesCommissionOfflinePayoutColumns = SalesCommissionOfflinePayoutColumns{
	Id:                "id",
	TenantId:          "tenant_id",
	PayoutNo:          "payout_no",
	AccountNo:         "account_no",
	CurrencyCode:      "currency_code",
	AmountMinor:       "amount_minor",
	EvidenceReference: "evidence_reference",
	PaidAt:            "paid_at",
	RecordedBy:        "recorded_by",
	RequestDigest:     "request_digest",
	CreatedAt:         "created_at",
}

// NewSalesCommissionOfflinePayoutDao creates and returns a new DAO object for table data access.
func NewSalesCommissionOfflinePayoutDao(handlers ...gdb.ModelHandler) *SalesCommissionOfflinePayoutDao {
	return &SalesCommissionOfflinePayoutDao{
		group:    "default",
		table:    "plugin_idc_core_sales_commission_offline_payout",
		columns:  salesCommissionOfflinePayoutColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SalesCommissionOfflinePayoutDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SalesCommissionOfflinePayoutDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SalesCommissionOfflinePayoutDao) Columns() SalesCommissionOfflinePayoutColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SalesCommissionOfflinePayoutDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SalesCommissionOfflinePayoutDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SalesCommissionOfflinePayoutDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
