// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CreditApplicationReviewDao is the data access object for the table plugin_idc_core_credit_application_review.
type CreditApplicationReviewDao struct {
	table    string                         // table is the underlying table name of the DAO.
	group    string                         // group is the database configuration group name of the current DAO.
	columns  CreditApplicationReviewColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler             // handlers for customized model modification.
}

// CreditApplicationReviewColumns defines and stores column names for the table plugin_idc_core_credit_application_review.
type CreditApplicationReviewColumns struct {
	Id             string //
	TenantId       string //
	ReviewNo       string //
	ApplicationNo  string //
	CustomerNo     string //
	CurrencyCode   string //
	ReviewSequence string //
	DecisionCode   string //
	ReviewerId     string //
	CommentDigest  string //
	EvidenceDigest string //
	ContentDigest  string //
	CreatedAt      string //
}

// creditApplicationReviewColumns holds the columns for the table plugin_idc_core_credit_application_review.
var creditApplicationReviewColumns = CreditApplicationReviewColumns{
	Id:             "id",
	TenantId:       "tenant_id",
	ReviewNo:       "review_no",
	ApplicationNo:  "application_no",
	CustomerNo:     "customer_no",
	CurrencyCode:   "currency_code",
	ReviewSequence: "review_sequence",
	DecisionCode:   "decision_code",
	ReviewerId:     "reviewer_id",
	CommentDigest:  "comment_digest",
	EvidenceDigest: "evidence_digest",
	ContentDigest:  "content_digest",
	CreatedAt:      "created_at",
}

// NewCreditApplicationReviewDao creates and returns a new DAO object for table data access.
func NewCreditApplicationReviewDao(handlers ...gdb.ModelHandler) *CreditApplicationReviewDao {
	return &CreditApplicationReviewDao{
		group:    "default",
		table:    "plugin_idc_core_credit_application_review",
		columns:  creditApplicationReviewColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CreditApplicationReviewDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CreditApplicationReviewDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CreditApplicationReviewDao) Columns() CreditApplicationReviewColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CreditApplicationReviewDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CreditApplicationReviewDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CreditApplicationReviewDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
