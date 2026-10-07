// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SalesTransferItemDao is the data access object for the table plugin_idc_core_sales_transfer_item.
type SalesTransferItemDao struct {
	table    string                   // table is the underlying table name of the DAO.
	group    string                   // group is the database configuration group name of the current DAO.
	columns  SalesTransferItemColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler       // handlers for customized model modification.
}

// SalesTransferItemColumns defines and stores column names for the table plugin_idc_core_sales_transfer_item.
type SalesTransferItemColumns struct {
	Id                string //
	TenantId          string //
	ItemNo            string //
	BatchNo           string //
	CustomerNo        string //
	OldAssignmentNo   string //
	NewAssignmentNo   string //
	StatusCode        string //
	FailureReasonCode string //
	ProcessedAt       string //
	CreatedAt         string //
}

// salesTransferItemColumns holds the columns for the table plugin_idc_core_sales_transfer_item.
var salesTransferItemColumns = SalesTransferItemColumns{
	Id:                "id",
	TenantId:          "tenant_id",
	ItemNo:            "item_no",
	BatchNo:           "batch_no",
	CustomerNo:        "customer_no",
	OldAssignmentNo:   "old_assignment_no",
	NewAssignmentNo:   "new_assignment_no",
	StatusCode:        "status_code",
	FailureReasonCode: "failure_reason_code",
	ProcessedAt:       "processed_at",
	CreatedAt:         "created_at",
}

// NewSalesTransferItemDao creates and returns a new DAO object for table data access.
func NewSalesTransferItemDao(handlers ...gdb.ModelHandler) *SalesTransferItemDao {
	return &SalesTransferItemDao{
		group:    "default",
		table:    "plugin_idc_core_sales_transfer_item",
		columns:  salesTransferItemColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SalesTransferItemDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SalesTransferItemDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SalesTransferItemDao) Columns() SalesTransferItemColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SalesTransferItemDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SalesTransferItemDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SalesTransferItemDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
