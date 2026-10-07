// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CustomerAccountStateHistoryDao is the data access object for the table plugin_idc_core_customer_account_state_history.
type CustomerAccountStateHistoryDao struct {
	table    string                             // table is the underlying table name of the DAO.
	group    string                             // group is the database configuration group name of the current DAO.
	columns  CustomerAccountStateHistoryColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                 // handlers for customized model modification.
}

// CustomerAccountStateHistoryColumns defines and stores column names for the table plugin_idc_core_customer_account_state_history.
type CustomerAccountStateHistoryColumns struct {
	Id                   string //
	TenantId             string //
	TransitionNo         string //
	CustomerNo           string //
	FromStatusCode       string //
	ToStatusCode         string //
	ReasonCode           string //
	SourceCode           string //
	SourceNo             string //
	ActorUserId          string //
	SubjectVersion       string //
	IdempotencyKeyDigest string //
	RequestFingerprint   string //
	OccurredAt           string //
	CreatedAt            string //
}

// customerAccountStateHistoryColumns holds the columns for the table plugin_idc_core_customer_account_state_history.
var customerAccountStateHistoryColumns = CustomerAccountStateHistoryColumns{
	Id:                   "id",
	TenantId:             "tenant_id",
	TransitionNo:         "transition_no",
	CustomerNo:           "customer_no",
	FromStatusCode:       "from_status_code",
	ToStatusCode:         "to_status_code",
	ReasonCode:           "reason_code",
	SourceCode:           "source_code",
	SourceNo:             "source_no",
	ActorUserId:          "actor_user_id",
	SubjectVersion:       "subject_version",
	IdempotencyKeyDigest: "idempotency_key_digest",
	RequestFingerprint:   "request_fingerprint",
	OccurredAt:           "occurred_at",
	CreatedAt:            "created_at",
}

// NewCustomerAccountStateHistoryDao creates and returns a new DAO object for table data access.
func NewCustomerAccountStateHistoryDao(handlers ...gdb.ModelHandler) *CustomerAccountStateHistoryDao {
	return &CustomerAccountStateHistoryDao{
		group:    "default",
		table:    "plugin_idc_core_customer_account_state_history",
		columns:  customerAccountStateHistoryColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CustomerAccountStateHistoryDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CustomerAccountStateHistoryDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CustomerAccountStateHistoryDao) Columns() CustomerAccountStateHistoryColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CustomerAccountStateHistoryDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CustomerAccountStateHistoryDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CustomerAccountStateHistoryDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
