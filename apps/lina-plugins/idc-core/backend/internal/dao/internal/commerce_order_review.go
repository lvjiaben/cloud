// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CommerceOrderReviewDao is the data access object for the table plugin_idc_core_commerce_order_review.
type CommerceOrderReviewDao struct {
	table    string                     // table is the underlying table name of the DAO.
	group    string                     // group is the database configuration group name of the current DAO.
	columns  CommerceOrderReviewColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler         // handlers for customized model modification.
}

// CommerceOrderReviewColumns defines and stores column names for the table plugin_idc_core_commerce_order_review.
type CommerceOrderReviewColumns struct {
	Id             string //
	TenantId       string //
	ReviewNo       string //
	OrderNo        string //
	CustomerNo     string //
	ReviewTypeCode string //
	StatusCode     string //
	ReasonCode     string //
	RequestDigest  string //
	EvidenceDigest string //
	ReviewedBy     string //
	RequestedAt    string //
	ReviewedAt     string //
	CreatedAt      string //
	UpdatedAt      string //
}

// commerceOrderReviewColumns holds the columns for the table plugin_idc_core_commerce_order_review.
var commerceOrderReviewColumns = CommerceOrderReviewColumns{
	Id:             "id",
	TenantId:       "tenant_id",
	ReviewNo:       "review_no",
	OrderNo:        "order_no",
	CustomerNo:     "customer_no",
	ReviewTypeCode: "review_type_code",
	StatusCode:     "status_code",
	ReasonCode:     "reason_code",
	RequestDigest:  "request_digest",
	EvidenceDigest: "evidence_digest",
	ReviewedBy:     "reviewed_by",
	RequestedAt:    "requested_at",
	ReviewedAt:     "reviewed_at",
	CreatedAt:      "created_at",
	UpdatedAt:      "updated_at",
}

// NewCommerceOrderReviewDao creates and returns a new DAO object for table data access.
func NewCommerceOrderReviewDao(handlers ...gdb.ModelHandler) *CommerceOrderReviewDao {
	return &CommerceOrderReviewDao{
		group:    "default",
		table:    "plugin_idc_core_commerce_order_review",
		columns:  commerceOrderReviewColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CommerceOrderReviewDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CommerceOrderReviewDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CommerceOrderReviewDao) Columns() CommerceOrderReviewColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CommerceOrderReviewDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CommerceOrderReviewDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CommerceOrderReviewDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
