// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ProductFulfillmentTaskDao is the data access object for the table plugin_idc_core_product_fulfillment_task.
type ProductFulfillmentTaskDao struct {
	table    string                        // table is the underlying table name of the DAO.
	group    string                        // group is the database configuration group name of the current DAO.
	columns  ProductFulfillmentTaskColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler            // handlers for customized model modification.
}

// ProductFulfillmentTaskColumns defines and stores column names for the table plugin_idc_core_product_fulfillment_task.
type ProductFulfillmentTaskColumns struct {
	TenantId               string //
	TaskNo                 string //
	IntentNo               string //
	StatusCode             string //
	PhaseCode              string //
	AttemptCount           string //
	QueryAttemptCount      string //
	SubmitAttemptCount     string //
	MaximumAttempts        string //
	RetryBaseSeconds       string //
	NextAttemptAt          string //
	LeaseToken             string //
	LeaseExpiresAt         string //
	ExternalReference      string //
	LastResultCode         string //
	TakeoverRequested      string //
	CancelRequested        string //
	Version                string //
	CreatedAt              string //
	UpdatedAt              string //
	UpstreamDeliveries     string //
	RecoveryCount          string //
	RecoveryQueryBaseline  string //
	RecoverySubmitBaseline string //
}

// productFulfillmentTaskColumns holds the columns for the table plugin_idc_core_product_fulfillment_task.
var productFulfillmentTaskColumns = ProductFulfillmentTaskColumns{
	TenantId:               "tenant_id",
	TaskNo:                 "task_no",
	IntentNo:               "intent_no",
	StatusCode:             "status_code",
	PhaseCode:              "phase_code",
	AttemptCount:           "attempt_count",
	QueryAttemptCount:      "query_attempt_count",
	SubmitAttemptCount:     "submit_attempt_count",
	MaximumAttempts:        "maximum_attempts",
	RetryBaseSeconds:       "retry_base_seconds",
	NextAttemptAt:          "next_attempt_at",
	LeaseToken:             "lease_token",
	LeaseExpiresAt:         "lease_expires_at",
	ExternalReference:      "external_reference",
	LastResultCode:         "last_result_code",
	TakeoverRequested:      "takeover_requested",
	CancelRequested:        "cancel_requested",
	Version:                "version",
	CreatedAt:              "created_at",
	UpdatedAt:              "updated_at",
	UpstreamDeliveries:     "upstream_deliveries",
	RecoveryCount:          "recovery_count",
	RecoveryQueryBaseline:  "recovery_query_baseline",
	RecoverySubmitBaseline: "recovery_submit_baseline",
}

// NewProductFulfillmentTaskDao creates and returns a new DAO object for table data access.
func NewProductFulfillmentTaskDao(handlers ...gdb.ModelHandler) *ProductFulfillmentTaskDao {
	return &ProductFulfillmentTaskDao{
		group:    "default",
		table:    "plugin_idc_core_product_fulfillment_task",
		columns:  productFulfillmentTaskColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ProductFulfillmentTaskDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ProductFulfillmentTaskDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ProductFulfillmentTaskDao) Columns() ProductFulfillmentTaskColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ProductFulfillmentTaskDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ProductFulfillmentTaskDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ProductFulfillmentTaskDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
