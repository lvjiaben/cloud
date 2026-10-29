// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// NativeCommercialTaskDao is the data access object for the table plugin_idc_resource_native_commercial_task.
type NativeCommercialTaskDao struct {
	table    string                      // table is the underlying table name of the DAO.
	group    string                      // group is the database configuration group name of the current DAO.
	columns  NativeCommercialTaskColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler          // handlers for customized model modification.
}

// NativeCommercialTaskColumns defines and stores column names for the table plugin_idc_resource_native_commercial_task.
type NativeCommercialTaskColumns struct {
	OperationNo         string //
	TenantId            string //
	EntitlementNo       string //
	CustomerNo          string //
	TaskNo              string //
	LocalResourceNo     string //
	ChannelNo           string //
	PoolNo              string //
	ExternalResourceRef string //
	ActionCode          string //
	Parameters          string //
	RequestDigest       string //
	IntentDigest        string //
	EntitlementVersion  string //
	AdmittedAt          string //
	AdmittedAttempt     string //
	ResultStatus        string //
	NextReportAt        string //
	CompletedAt         string //
	CreatedAt           string //
	UpdatedAt           string //
}

// nativeCommercialTaskColumns holds the columns for the table plugin_idc_resource_native_commercial_task.
var nativeCommercialTaskColumns = NativeCommercialTaskColumns{
	OperationNo:         "operation_no",
	TenantId:            "tenant_id",
	EntitlementNo:       "entitlement_no",
	CustomerNo:          "customer_no",
	TaskNo:              "task_no",
	LocalResourceNo:     "local_resource_no",
	ChannelNo:           "channel_no",
	PoolNo:              "pool_no",
	ExternalResourceRef: "external_resource_ref",
	ActionCode:          "action_code",
	Parameters:          "parameters",
	RequestDigest:       "request_digest",
	IntentDigest:        "intent_digest",
	EntitlementVersion:  "entitlement_version",
	AdmittedAt:          "admitted_at",
	AdmittedAttempt:     "admitted_attempt",
	ResultStatus:        "result_status",
	NextReportAt:        "next_report_at",
	CompletedAt:         "completed_at",
	CreatedAt:           "created_at",
	UpdatedAt:           "updated_at",
}

// NewNativeCommercialTaskDao creates and returns a new DAO object for table data access.
func NewNativeCommercialTaskDao(handlers ...gdb.ModelHandler) *NativeCommercialTaskDao {
	return &NativeCommercialTaskDao{
		group:    "default",
		table:    "plugin_idc_resource_native_commercial_task",
		columns:  nativeCommercialTaskColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *NativeCommercialTaskDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *NativeCommercialTaskDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *NativeCommercialTaskDao) Columns() NativeCommercialTaskColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *NativeCommercialTaskDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *NativeCommercialTaskDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *NativeCommercialTaskDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
