// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// BillingAdjustmentApprovalDao is the data access object for the table plugin_idc_core_billing_adjustment_approval.
type BillingAdjustmentApprovalDao struct {
	table    string                           // table is the underlying table name of the DAO.
	group    string                           // group is the database configuration group name of the current DAO.
	columns  BillingAdjustmentApprovalColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler               // handlers for customized model modification.
}

// BillingAdjustmentApprovalColumns defines and stores column names for the table plugin_idc_core_billing_adjustment_approval.
type BillingAdjustmentApprovalColumns struct {
	Id                  string //
	TenantId            string //
	ApprovalNo          string //
	AdjustmentNo        string //
	RequesterIdSnapshot string //
	ReviewerId          string //
	DecisionCode        string //
	CommentDigest       string //
	EvidenceDigest      string //
	DecidedAt           string //
	CreatedAt           string //
}

// billingAdjustmentApprovalColumns holds the columns for the table plugin_idc_core_billing_adjustment_approval.
var billingAdjustmentApprovalColumns = BillingAdjustmentApprovalColumns{
	Id:                  "id",
	TenantId:            "tenant_id",
	ApprovalNo:          "approval_no",
	AdjustmentNo:        "adjustment_no",
	RequesterIdSnapshot: "requester_id_snapshot",
	ReviewerId:          "reviewer_id",
	DecisionCode:        "decision_code",
	CommentDigest:       "comment_digest",
	EvidenceDigest:      "evidence_digest",
	DecidedAt:           "decided_at",
	CreatedAt:           "created_at",
}

// NewBillingAdjustmentApprovalDao creates and returns a new DAO object for table data access.
func NewBillingAdjustmentApprovalDao(handlers ...gdb.ModelHandler) *BillingAdjustmentApprovalDao {
	return &BillingAdjustmentApprovalDao{
		group:    "default",
		table:    "plugin_idc_core_billing_adjustment_approval",
		columns:  billingAdjustmentApprovalColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *BillingAdjustmentApprovalDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *BillingAdjustmentApprovalDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *BillingAdjustmentApprovalDao) Columns() BillingAdjustmentApprovalColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *BillingAdjustmentApprovalDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *BillingAdjustmentApprovalDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *BillingAdjustmentApprovalDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
