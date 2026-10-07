// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ReconciliationChannelHealthDao is the data access object for the table plugin_idc_payment_reconciliation_channel_health.
type ReconciliationChannelHealthDao struct {
	table    string                             // table is the underlying table name of the DAO.
	group    string                             // group is the database configuration group name of the current DAO.
	columns  ReconciliationChannelHealthColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                 // handlers for customized model modification.
}

// ReconciliationChannelHealthColumns defines and stores column names for the table plugin_idc_payment_reconciliation_channel_health.
type ReconciliationChannelHealthColumns struct {
	Id                      string //
	ChannelNo               string //
	CapabilityCode          string //
	HealthCode              string //
	LatestBatchNo           string //
	LatestBusinessDate      string //
	UnresolvedCount         string //
	ConsecutiveFailureCount string //
	ResultUnknown           string //
	FailureCode             string //
	Version                 string //
	UpdatedAt               string //
}

// reconciliationChannelHealthColumns holds the columns for the table plugin_idc_payment_reconciliation_channel_health.
var reconciliationChannelHealthColumns = ReconciliationChannelHealthColumns{
	Id:                      "id",
	ChannelNo:               "channel_no",
	CapabilityCode:          "capability_code",
	HealthCode:              "health_code",
	LatestBatchNo:           "latest_batch_no",
	LatestBusinessDate:      "latest_business_date",
	UnresolvedCount:         "unresolved_count",
	ConsecutiveFailureCount: "consecutive_failure_count",
	ResultUnknown:           "result_unknown",
	FailureCode:             "failure_code",
	Version:                 "version",
	UpdatedAt:               "updated_at",
}

// NewReconciliationChannelHealthDao creates and returns a new DAO object for table data access.
func NewReconciliationChannelHealthDao(handlers ...gdb.ModelHandler) *ReconciliationChannelHealthDao {
	return &ReconciliationChannelHealthDao{
		group:    "default",
		table:    "plugin_idc_payment_reconciliation_channel_health",
		columns:  reconciliationChannelHealthColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ReconciliationChannelHealthDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ReconciliationChannelHealthDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ReconciliationChannelHealthDao) Columns() ReconciliationChannelHealthColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ReconciliationChannelHealthDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ReconciliationChannelHealthDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ReconciliationChannelHealthDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
