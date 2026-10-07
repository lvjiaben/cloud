// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ProviderCapacityObservationItemDao is the data access object for the table plugin_idc_resource_provider_capacity_observation_item.
type ProviderCapacityObservationItemDao struct {
	table    string                                 // table is the underlying table name of the DAO.
	group    string                                 // group is the database configuration group name of the current DAO.
	columns  ProviderCapacityObservationItemColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                     // handlers for customized model modification.
}

// ProviderCapacityObservationItemColumns defines and stores column names for the table plugin_idc_resource_provider_capacity_observation_item.
type ProviderCapacityObservationItemColumns struct {
	Id            string //
	TenantId      string //
	ObservationNo string //
	CapacityKind  string //
	ExternalRef   string //
	Projection    string // Normalized non-secret projection; raw Provider responses are forbidden
	ObservedAt    string //
	CreatedAt     string //
}

// providerCapacityObservationItemColumns holds the columns for the table plugin_idc_resource_provider_capacity_observation_item.
var providerCapacityObservationItemColumns = ProviderCapacityObservationItemColumns{
	Id:            "id",
	TenantId:      "tenant_id",
	ObservationNo: "observation_no",
	CapacityKind:  "capacity_kind",
	ExternalRef:   "external_ref",
	Projection:    "projection",
	ObservedAt:    "observed_at",
	CreatedAt:     "created_at",
}

// NewProviderCapacityObservationItemDao creates and returns a new DAO object for table data access.
func NewProviderCapacityObservationItemDao(handlers ...gdb.ModelHandler) *ProviderCapacityObservationItemDao {
	return &ProviderCapacityObservationItemDao{
		group:    "default",
		table:    "plugin_idc_resource_provider_capacity_observation_item",
		columns:  providerCapacityObservationItemColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ProviderCapacityObservationItemDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ProviderCapacityObservationItemDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ProviderCapacityObservationItemDao) Columns() ProviderCapacityObservationItemColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ProviderCapacityObservationItemDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ProviderCapacityObservationItemDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ProviderCapacityObservationItemDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
