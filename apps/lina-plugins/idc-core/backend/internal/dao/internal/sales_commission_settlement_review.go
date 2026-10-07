// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SalesCommissionSettlementReviewDao is the data access object for the table plugin_idc_core_sales_commission_settlement_review.
type SalesCommissionSettlementReviewDao struct {
	table    string                                 // table is the underlying table name of the DAO.
	group    string                                 // group is the database configuration group name of the current DAO.
	columns  SalesCommissionSettlementReviewColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                     // handlers for customized model modification.
}

// SalesCommissionSettlementReviewColumns defines and stores column names for the table plugin_idc_core_sales_commission_settlement_review.
type SalesCommissionSettlementReviewColumns struct {
	Id             string //
	TenantId       string //
	ReviewNo       string //
	SettlementNo   string //
	DecisionCode   string //
	ReasonCode     string //
	ReviewerUserId string //
	EvidenceDigest string //
	ReviewedAt     string //
}

// salesCommissionSettlementReviewColumns holds the columns for the table plugin_idc_core_sales_commission_settlement_review.
var salesCommissionSettlementReviewColumns = SalesCommissionSettlementReviewColumns{
	Id:             "id",
	TenantId:       "tenant_id",
	ReviewNo:       "review_no",
	SettlementNo:   "settlement_no",
	DecisionCode:   "decision_code",
	ReasonCode:     "reason_code",
	ReviewerUserId: "reviewer_user_id",
	EvidenceDigest: "evidence_digest",
	ReviewedAt:     "reviewed_at",
}

// NewSalesCommissionSettlementReviewDao creates and returns a new DAO object for table data access.
func NewSalesCommissionSettlementReviewDao(handlers ...gdb.ModelHandler) *SalesCommissionSettlementReviewDao {
	return &SalesCommissionSettlementReviewDao{
		group:    "default",
		table:    "plugin_idc_core_sales_commission_settlement_review",
		columns:  salesCommissionSettlementReviewColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SalesCommissionSettlementReviewDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SalesCommissionSettlementReviewDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SalesCommissionSettlementReviewDao) Columns() SalesCommissionSettlementReviewColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SalesCommissionSettlementReviewDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SalesCommissionSettlementReviewDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SalesCommissionSettlementReviewDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
