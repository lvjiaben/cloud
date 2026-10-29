// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ProductFulfillmentCommandDao is the data access object for the table plugin_idc_core_product_fulfillment_command.
type ProductFulfillmentCommandDao struct {
	table    string                           // table is the underlying table name of the DAO.
	group    string                           // group is the database configuration group name of the current DAO.
	columns  ProductFulfillmentCommandColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler               // handlers for customized model modification.
}

// ProductFulfillmentCommandColumns defines and stores column names for the table plugin_idc_core_product_fulfillment_command.
type ProductFulfillmentCommandColumns struct {
	TenantId              string //
	CommandNo             string //
	IntentNo              string //
	ActionCode            string //
	IdempotencyKey        string //
	RequestDigest         string //
	ReasonCode            string //
	ReasonLabelZhCn       string //
	ReasonLabelEnUs       string //
	SafeNote              string //
	ActorUserId           string //
	ResultingVersion      string //
	CreatedAt             string //
	RecoveryTaskVersion   string //
	RecoveryQueryCount    string //
	RecoverySubmitCount   string //
	RecoveryConfiguration string //
}

// productFulfillmentCommandColumns holds the columns for the table plugin_idc_core_product_fulfillment_command.
var productFulfillmentCommandColumns = ProductFulfillmentCommandColumns{
	TenantId:              "tenant_id",
	CommandNo:             "command_no",
	IntentNo:              "intent_no",
	ActionCode:            "action_code",
	IdempotencyKey:        "idempotency_key",
	RequestDigest:         "request_digest",
	ReasonCode:            "reason_code",
	ReasonLabelZhCn:       "reason_label_zh_cn",
	ReasonLabelEnUs:       "reason_label_en_us",
	SafeNote:              "safe_note",
	ActorUserId:           "actor_user_id",
	ResultingVersion:      "resulting_version",
	CreatedAt:             "created_at",
	RecoveryTaskVersion:   "recovery_task_version",
	RecoveryQueryCount:    "recovery_query_count",
	RecoverySubmitCount:   "recovery_submit_count",
	RecoveryConfiguration: "recovery_configuration",
}

// NewProductFulfillmentCommandDao creates and returns a new DAO object for table data access.
func NewProductFulfillmentCommandDao(handlers ...gdb.ModelHandler) *ProductFulfillmentCommandDao {
	return &ProductFulfillmentCommandDao{
		group:    "default",
		table:    "plugin_idc_core_product_fulfillment_command",
		columns:  productFulfillmentCommandColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ProductFulfillmentCommandDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ProductFulfillmentCommandDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ProductFulfillmentCommandDao) Columns() ProductFulfillmentCommandColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ProductFulfillmentCommandDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ProductFulfillmentCommandDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ProductFulfillmentCommandDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
