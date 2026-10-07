// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SalesTransferBatchDao is the data access object for the table plugin_idc_core_sales_transfer_batch.
type SalesTransferBatchDao struct {
	table    string                    // table is the underlying table name of the DAO.
	group    string                    // group is the database configuration group name of the current DAO.
	columns  SalesTransferBatchColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler        // handlers for customized model modification.
}

// SalesTransferBatchColumns defines and stores column names for the table plugin_idc_core_sales_transfer_batch.
type SalesTransferBatchColumns struct {
	Id                 string //
	TenantId           string //
	BatchNo            string //
	SourceMembershipNo string //
	TargetMembershipNo string //
	RequestedCount     string //
	SucceededCount     string //
	FailedCount        string //
	ReasonCode         string //
	RequestedBy        string //
	StatusCode         string //
	CreatedAt          string //
	CompletedAt        string //
}

// salesTransferBatchColumns holds the columns for the table plugin_idc_core_sales_transfer_batch.
var salesTransferBatchColumns = SalesTransferBatchColumns{
	Id:                 "id",
	TenantId:           "tenant_id",
	BatchNo:            "batch_no",
	SourceMembershipNo: "source_membership_no",
	TargetMembershipNo: "target_membership_no",
	RequestedCount:     "requested_count",
	SucceededCount:     "succeeded_count",
	FailedCount:        "failed_count",
	ReasonCode:         "reason_code",
	RequestedBy:        "requested_by",
	StatusCode:         "status_code",
	CreatedAt:          "created_at",
	CompletedAt:        "completed_at",
}

// NewSalesTransferBatchDao creates and returns a new DAO object for table data access.
func NewSalesTransferBatchDao(handlers ...gdb.ModelHandler) *SalesTransferBatchDao {
	return &SalesTransferBatchDao{
		group:    "default",
		table:    "plugin_idc_core_sales_transfer_batch",
		columns:  salesTransferBatchColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SalesTransferBatchDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SalesTransferBatchDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SalesTransferBatchDao) Columns() SalesTransferBatchColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SalesTransferBatchDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SalesTransferBatchDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SalesTransferBatchDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
