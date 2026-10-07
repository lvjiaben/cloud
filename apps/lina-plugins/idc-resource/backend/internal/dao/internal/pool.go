// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// PoolDao is the data access object for the table plugin_idc_resource_pool.
type PoolDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  PoolColumns        // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// PoolColumns defines and stores column names for the table plugin_idc_resource_pool.
type PoolColumns struct {
	Id              string //
	TenantId        string //
	PoolNo          string //
	ChannelNo       string //
	DisplayName     string //
	ResourceKind    string //
	StatusCode      string //
	RoutingPriority string //
	RoutingWeight   string // Relative selection weight within the same routing priority
	Selector        string // Non-secret normalized placement selector evaluated in set-based routing
	Version         string //
	CreatedBy       string //
	UpdatedBy       string //
	CreatedAt       string //
	UpdatedAt       string //
	DeletedAt       string //
}

// poolColumns holds the columns for the table plugin_idc_resource_pool.
var poolColumns = PoolColumns{
	Id:              "id",
	TenantId:        "tenant_id",
	PoolNo:          "pool_no",
	ChannelNo:       "channel_no",
	DisplayName:     "display_name",
	ResourceKind:    "resource_kind",
	StatusCode:      "status_code",
	RoutingPriority: "routing_priority",
	RoutingWeight:   "routing_weight",
	Selector:        "selector",
	Version:         "version",
	CreatedBy:       "created_by",
	UpdatedBy:       "updated_by",
	CreatedAt:       "created_at",
	UpdatedAt:       "updated_at",
	DeletedAt:       "deleted_at",
}

// NewPoolDao creates and returns a new DAO object for table data access.
func NewPoolDao(handlers ...gdb.ModelHandler) *PoolDao {
	return &PoolDao{
		group:    "default",
		table:    "plugin_idc_resource_pool",
		columns:  poolColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *PoolDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *PoolDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *PoolDao) Columns() PoolColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *PoolDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *PoolDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *PoolDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
