// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// PrivacyActionDao is the data access object for the table plugin_idc_compliance_privacy_action.
type PrivacyActionDao struct {
	table    string               // table is the underlying table name of the DAO.
	group    string               // group is the database configuration group name of the current DAO.
	columns  PrivacyActionColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler   // handlers for customized model modification.
}

// PrivacyActionColumns defines and stores column names for the table plugin_idc_compliance_privacy_action.
type PrivacyActionColumns struct {
	Id                 string //
	TenantId           string //
	ActionNo           string //
	ActionCode         string //
	TargetNo           string //
	IdempotencyDigest  string //
	RequestFingerprint string //
	ResultReference    string //
	ActorUserId        string //
	CreatedAt          string //
}

// privacyActionColumns holds the columns for the table plugin_idc_compliance_privacy_action.
var privacyActionColumns = PrivacyActionColumns{
	Id:                 "id",
	TenantId:           "tenant_id",
	ActionNo:           "action_no",
	ActionCode:         "action_code",
	TargetNo:           "target_no",
	IdempotencyDigest:  "idempotency_digest",
	RequestFingerprint: "request_fingerprint",
	ResultReference:    "result_reference",
	ActorUserId:        "actor_user_id",
	CreatedAt:          "created_at",
}

// NewPrivacyActionDao creates and returns a new DAO object for table data access.
func NewPrivacyActionDao(handlers ...gdb.ModelHandler) *PrivacyActionDao {
	return &PrivacyActionDao{
		group:    "default",
		table:    "plugin_idc_compliance_privacy_action",
		columns:  privacyActionColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *PrivacyActionDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *PrivacyActionDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *PrivacyActionDao) Columns() PrivacyActionColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *PrivacyActionDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *PrivacyActionDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *PrivacyActionDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
