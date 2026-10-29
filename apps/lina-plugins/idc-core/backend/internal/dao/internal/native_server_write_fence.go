// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// NativeServerWriteFenceDao is the data access object for the table plugin_idc_core_native_server_write_fence.
type NativeServerWriteFenceDao struct {
	table    string                        // table is the underlying table name of the DAO.
	group    string                        // group is the database configuration group name of the current DAO.
	columns  NativeServerWriteFenceColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler            // handlers for customized model modification.
}

// NativeServerWriteFenceColumns defines and stores column names for the table plugin_idc_core_native_server_write_fence.
type NativeServerWriteFenceColumns struct {
	TenantId                   string //
	TaskNo                     string //
	OperationNo                string //
	EntitlementNo              string //
	CustomerNo                 string //
	ExpectedEntitlementVersion string //
	ActionCode                 string //
	RequestDigest              string //
	StatusCode                 string //
	EvidenceDigest             string //
	CreatedAt                  string //
	UpdatedAt                  string //
	CompletedAt                string //
}

// nativeServerWriteFenceColumns holds the columns for the table plugin_idc_core_native_server_write_fence.
var nativeServerWriteFenceColumns = NativeServerWriteFenceColumns{
	TenantId:                   "tenant_id",
	TaskNo:                     "task_no",
	OperationNo:                "operation_no",
	EntitlementNo:              "entitlement_no",
	CustomerNo:                 "customer_no",
	ExpectedEntitlementVersion: "expected_entitlement_version",
	ActionCode:                 "action_code",
	RequestDigest:              "request_digest",
	StatusCode:                 "status_code",
	EvidenceDigest:             "evidence_digest",
	CreatedAt:                  "created_at",
	UpdatedAt:                  "updated_at",
	CompletedAt:                "completed_at",
}

// NewNativeServerWriteFenceDao creates and returns a new DAO object for table data access.
func NewNativeServerWriteFenceDao(handlers ...gdb.ModelHandler) *NativeServerWriteFenceDao {
	return &NativeServerWriteFenceDao{
		group:    "default",
		table:    "plugin_idc_core_native_server_write_fence",
		columns:  nativeServerWriteFenceColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *NativeServerWriteFenceDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *NativeServerWriteFenceDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *NativeServerWriteFenceDao) Columns() NativeServerWriteFenceColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *NativeServerWriteFenceDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *NativeServerWriteFenceDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *NativeServerWriteFenceDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
