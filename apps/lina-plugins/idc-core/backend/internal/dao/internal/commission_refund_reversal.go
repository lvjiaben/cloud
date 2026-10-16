// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CommissionRefundReversalDao is the data access object for the table plugin_idc_core_commission_refund_reversal.
type CommissionRefundReversalDao struct {
	table    string                          // table is the underlying table name of the DAO.
	group    string                          // group is the database configuration group name of the current DAO.
	columns  CommissionRefundReversalColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler              // handlers for customized model modification.
}

// CommissionRefundReversalColumns defines and stores column names for the table plugin_idc_core_commission_refund_reversal.
type CommissionRefundReversalColumns struct {
	Id            string //
	TenantId      string //
	ReversalNo    string //
	CommissionNo  string //
	RefundNo      string //
	SourceEventNo string //
	AmountMinor   string //
	CreatedAt     string //
}

// commissionRefundReversalColumns holds the columns for the table plugin_idc_core_commission_refund_reversal.
var commissionRefundReversalColumns = CommissionRefundReversalColumns{
	Id:            "id",
	TenantId:      "tenant_id",
	ReversalNo:    "reversal_no",
	CommissionNo:  "commission_no",
	RefundNo:      "refund_no",
	SourceEventNo: "source_event_no",
	AmountMinor:   "amount_minor",
	CreatedAt:     "created_at",
}

// NewCommissionRefundReversalDao creates and returns a new DAO object for table data access.
func NewCommissionRefundReversalDao(handlers ...gdb.ModelHandler) *CommissionRefundReversalDao {
	return &CommissionRefundReversalDao{
		group:    "default",
		table:    "plugin_idc_core_commission_refund_reversal",
		columns:  commissionRefundReversalColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CommissionRefundReversalDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CommissionRefundReversalDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CommissionRefundReversalDao) Columns() CommissionRefundReversalColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CommissionRefundReversalDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CommissionRefundReversalDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CommissionRefundReversalDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
