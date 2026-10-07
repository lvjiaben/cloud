// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SalesCommissionPayoutRecoveryDao is the data access object for the table plugin_idc_core_sales_commission_payout_recovery.
type SalesCommissionPayoutRecoveryDao struct {
	table    string                               // table is the underlying table name of the DAO.
	group    string                               // group is the database configuration group name of the current DAO.
	columns  SalesCommissionPayoutRecoveryColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                   // handlers for customized model modification.
}

// SalesCommissionPayoutRecoveryColumns defines and stores column names for the table plugin_idc_core_sales_commission_payout_recovery.
type SalesCommissionPayoutRecoveryColumns struct {
	Id             string //
	TenantId       string //
	RecoveryNo     string //
	PayoutItemNo   string //
	AttemptNo      string //
	ActionCode     string //
	ResultCode     string //
	ReasonCode     string //
	EvidenceDigest string //
	NextAttemptAt  string //
	RecordedAt     string //
}

// salesCommissionPayoutRecoveryColumns holds the columns for the table plugin_idc_core_sales_commission_payout_recovery.
var salesCommissionPayoutRecoveryColumns = SalesCommissionPayoutRecoveryColumns{
	Id:             "id",
	TenantId:       "tenant_id",
	RecoveryNo:     "recovery_no",
	PayoutItemNo:   "payout_item_no",
	AttemptNo:      "attempt_no",
	ActionCode:     "action_code",
	ResultCode:     "result_code",
	ReasonCode:     "reason_code",
	EvidenceDigest: "evidence_digest",
	NextAttemptAt:  "next_attempt_at",
	RecordedAt:     "recorded_at",
}

// NewSalesCommissionPayoutRecoveryDao creates and returns a new DAO object for table data access.
func NewSalesCommissionPayoutRecoveryDao(handlers ...gdb.ModelHandler) *SalesCommissionPayoutRecoveryDao {
	return &SalesCommissionPayoutRecoveryDao{
		group:    "default",
		table:    "plugin_idc_core_sales_commission_payout_recovery",
		columns:  salesCommissionPayoutRecoveryColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SalesCommissionPayoutRecoveryDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SalesCommissionPayoutRecoveryDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SalesCommissionPayoutRecoveryDao) Columns() SalesCommissionPayoutRecoveryColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SalesCommissionPayoutRecoveryDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SalesCommissionPayoutRecoveryDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SalesCommissionPayoutRecoveryDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
