// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// MarketingSegmentMemberDao is the data access object for the table plugin_idc_core_marketing_segment_member.
type MarketingSegmentMemberDao struct {
	table    string                        // table is the underlying table name of the DAO.
	group    string                        // group is the database configuration group name of the current DAO.
	columns  MarketingSegmentMemberColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler            // handlers for customized model modification.
}

// MarketingSegmentMemberColumns defines and stores column names for the table plugin_idc_core_marketing_segment_member.
type MarketingSegmentMemberColumns struct {
	Id             string //
	TenantId       string //
	SegmentNo      string //
	CustomerNo     string //
	SourceCode     string //
	SegmentVersion string //
	StatusCode     string //
	EvaluatedAt    string //
	RevokedAt      string //
	CreatedAt      string //
	UpdatedAt      string //
}

// marketingSegmentMemberColumns holds the columns for the table plugin_idc_core_marketing_segment_member.
var marketingSegmentMemberColumns = MarketingSegmentMemberColumns{
	Id:             "id",
	TenantId:       "tenant_id",
	SegmentNo:      "segment_no",
	CustomerNo:     "customer_no",
	SourceCode:     "source_code",
	SegmentVersion: "segment_version",
	StatusCode:     "status_code",
	EvaluatedAt:    "evaluated_at",
	RevokedAt:      "revoked_at",
	CreatedAt:      "created_at",
	UpdatedAt:      "updated_at",
}

// NewMarketingSegmentMemberDao creates and returns a new DAO object for table data access.
func NewMarketingSegmentMemberDao(handlers ...gdb.ModelHandler) *MarketingSegmentMemberDao {
	return &MarketingSegmentMemberDao{
		group:    "default",
		table:    "plugin_idc_core_marketing_segment_member",
		columns:  marketingSegmentMemberColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *MarketingSegmentMemberDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *MarketingSegmentMemberDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *MarketingSegmentMemberDao) Columns() MarketingSegmentMemberColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *MarketingSegmentMemberDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *MarketingSegmentMemberDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *MarketingSegmentMemberDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
