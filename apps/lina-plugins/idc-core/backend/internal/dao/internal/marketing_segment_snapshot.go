// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// MarketingSegmentSnapshotDao is the data access object for the table plugin_idc_core_marketing_segment_snapshot.
type MarketingSegmentSnapshotDao struct {
	table    string                          // table is the underlying table name of the DAO.
	group    string                          // group is the database configuration group name of the current DAO.
	columns  MarketingSegmentSnapshotColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler              // handlers for customized model modification.
}

// MarketingSegmentSnapshotColumns defines and stores column names for the table plugin_idc_core_marketing_segment_snapshot.
type MarketingSegmentSnapshotColumns struct {
	Id               string //
	TenantId         string //
	SnapshotNo       string //
	SegmentNo        string //
	SegmentVersion   string //
	DefinitionDigest string //
	StatusCode       string //
	MemberCount      string //
	ContentDigest    string //
	StartedAt        string //
	CompletedAt      string //
	CreatedAt        string //
}

// marketingSegmentSnapshotColumns holds the columns for the table plugin_idc_core_marketing_segment_snapshot.
var marketingSegmentSnapshotColumns = MarketingSegmentSnapshotColumns{
	Id:               "id",
	TenantId:         "tenant_id",
	SnapshotNo:       "snapshot_no",
	SegmentNo:        "segment_no",
	SegmentVersion:   "segment_version",
	DefinitionDigest: "definition_digest",
	StatusCode:       "status_code",
	MemberCount:      "member_count",
	ContentDigest:    "content_digest",
	StartedAt:        "started_at",
	CompletedAt:      "completed_at",
	CreatedAt:        "created_at",
}

// NewMarketingSegmentSnapshotDao creates and returns a new DAO object for table data access.
func NewMarketingSegmentSnapshotDao(handlers ...gdb.ModelHandler) *MarketingSegmentSnapshotDao {
	return &MarketingSegmentSnapshotDao{
		group:    "default",
		table:    "plugin_idc_core_marketing_segment_snapshot",
		columns:  marketingSegmentSnapshotColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *MarketingSegmentSnapshotDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *MarketingSegmentSnapshotDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *MarketingSegmentSnapshotDao) Columns() MarketingSegmentSnapshotColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *MarketingSegmentSnapshotDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *MarketingSegmentSnapshotDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *MarketingSegmentSnapshotDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
