// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// UpstreamTaskBindingDao is the data access object for the table plugin_idc_resource_upstream_task_binding.
type UpstreamTaskBindingDao struct {
	table    string                     // table is the underlying table name of the DAO.
	group    string                     // group is the database configuration group name of the current DAO.
	columns  UpstreamTaskBindingColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler         // handlers for customized model modification.
}

// UpstreamTaskBindingColumns defines and stores column names for the table plugin_idc_resource_upstream_task_binding.
type UpstreamTaskBindingColumns struct {
	Id                  string //
	TenantId            string //
	BindingNo           string //
	TaskNo              string //
	StepNo              string //
	ChannelNo           string //
	ExternalTaskRef     string //
	ExternalResourceRef string //
	StatusCode          string //
	LastProgress        string //
	LastObservedAt      string //
	CreatedAt           string //
	UpdatedAt           string //
}

// upstreamTaskBindingColumns holds the columns for the table plugin_idc_resource_upstream_task_binding.
var upstreamTaskBindingColumns = UpstreamTaskBindingColumns{
	Id:                  "id",
	TenantId:            "tenant_id",
	BindingNo:           "binding_no",
	TaskNo:              "task_no",
	StepNo:              "step_no",
	ChannelNo:           "channel_no",
	ExternalTaskRef:     "external_task_ref",
	ExternalResourceRef: "external_resource_ref",
	StatusCode:          "status_code",
	LastProgress:        "last_progress",
	LastObservedAt:      "last_observed_at",
	CreatedAt:           "created_at",
	UpdatedAt:           "updated_at",
}

// NewUpstreamTaskBindingDao creates and returns a new DAO object for table data access.
func NewUpstreamTaskBindingDao(handlers ...gdb.ModelHandler) *UpstreamTaskBindingDao {
	return &UpstreamTaskBindingDao{
		group:    "default",
		table:    "plugin_idc_resource_upstream_task_binding",
		columns:  upstreamTaskBindingColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *UpstreamTaskBindingDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *UpstreamTaskBindingDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *UpstreamTaskBindingDao) Columns() UpstreamTaskBindingColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *UpstreamTaskBindingDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *UpstreamTaskBindingDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *UpstreamTaskBindingDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
