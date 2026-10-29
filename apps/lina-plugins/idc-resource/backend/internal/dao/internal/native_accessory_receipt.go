// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// NativeAccessoryReceiptDao is the data access object for the table plugin_idc_resource_native_accessory_receipt.
type NativeAccessoryReceiptDao struct {
	table    string                        // table is the underlying table name of the DAO.
	group    string                        // group is the database configuration group name of the current DAO.
	columns  NativeAccessoryReceiptColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler            // handlers for customized model modification.
}

// NativeAccessoryReceiptColumns defines and stores column names for the table plugin_idc_resource_native_accessory_receipt.
type NativeAccessoryReceiptColumns struct {
	TaskNo        string //
	TenantId      string //
	OperationNo   string //
	EntitlementNo string //
	CustomerNo    string //
	ObjectNo      string //
	AccessoryKind string //
	NextReportAt  string //
	CompletedAt   string //
	CreatedAt     string //
	UpdatedAt     string //
}

// nativeAccessoryReceiptColumns holds the columns for the table plugin_idc_resource_native_accessory_receipt.
var nativeAccessoryReceiptColumns = NativeAccessoryReceiptColumns{
	TaskNo:        "task_no",
	TenantId:      "tenant_id",
	OperationNo:   "operation_no",
	EntitlementNo: "entitlement_no",
	CustomerNo:    "customer_no",
	ObjectNo:      "object_no",
	AccessoryKind: "accessory_kind",
	NextReportAt:  "next_report_at",
	CompletedAt:   "completed_at",
	CreatedAt:     "created_at",
	UpdatedAt:     "updated_at",
}

// NewNativeAccessoryReceiptDao creates and returns a new DAO object for table data access.
func NewNativeAccessoryReceiptDao(handlers ...gdb.ModelHandler) *NativeAccessoryReceiptDao {
	return &NativeAccessoryReceiptDao{
		group:    "default",
		table:    "plugin_idc_resource_native_accessory_receipt",
		columns:  nativeAccessoryReceiptColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *NativeAccessoryReceiptDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *NativeAccessoryReceiptDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *NativeAccessoryReceiptDao) Columns() NativeAccessoryReceiptColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *NativeAccessoryReceiptDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *NativeAccessoryReceiptDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *NativeAccessoryReceiptDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
