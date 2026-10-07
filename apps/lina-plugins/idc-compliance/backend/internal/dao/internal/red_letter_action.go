// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// RedLetterActionDao is the data access object for the table plugin_idc_compliance_red_letter_action.
type RedLetterActionDao struct {
	table    string                 // table is the underlying table name of the DAO.
	group    string                 // group is the database configuration group name of the current DAO.
	columns  RedLetterActionColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler     // handlers for customized model modification.
}

// RedLetterActionColumns defines and stores column names for the table plugin_idc_compliance_red_letter_action.
type RedLetterActionColumns struct {
	Id                 string //
	TenantId           string //
	ActionNo           string //
	TaskNo             string //
	ActionCode         string //
	IdempotencyDigest  string //
	RequestFingerprint string //
	ActorTypeCode      string //
	ActorUserId        string //
	BeforeStatusCode   string //
	AfterStatusCode    string //
	BeforeVersion      string //
	AfterVersion       string //
	EvidenceNo         string //
	CreatedAt          string //
}

// redLetterActionColumns holds the columns for the table plugin_idc_compliance_red_letter_action.
var redLetterActionColumns = RedLetterActionColumns{
	Id:                 "id",
	TenantId:           "tenant_id",
	ActionNo:           "action_no",
	TaskNo:             "task_no",
	ActionCode:         "action_code",
	IdempotencyDigest:  "idempotency_digest",
	RequestFingerprint: "request_fingerprint",
	ActorTypeCode:      "actor_type_code",
	ActorUserId:        "actor_user_id",
	BeforeStatusCode:   "before_status_code",
	AfterStatusCode:    "after_status_code",
	BeforeVersion:      "before_version",
	AfterVersion:       "after_version",
	EvidenceNo:         "evidence_no",
	CreatedAt:          "created_at",
}

// NewRedLetterActionDao creates and returns a new DAO object for table data access.
func NewRedLetterActionDao(handlers ...gdb.ModelHandler) *RedLetterActionDao {
	return &RedLetterActionDao{
		group:    "default",
		table:    "plugin_idc_compliance_red_letter_action",
		columns:  redLetterActionColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *RedLetterActionDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *RedLetterActionDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *RedLetterActionDao) Columns() RedLetterActionColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *RedLetterActionDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *RedLetterActionDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *RedLetterActionDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
