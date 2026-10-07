// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CatalogCostSnapshotDao is the data access object for the table plugin_idc_core_catalog_cost_snapshot.
type CatalogCostSnapshotDao struct {
	table    string                     // table is the underlying table name of the DAO.
	group    string                     // group is the database configuration group name of the current DAO.
	columns  CatalogCostSnapshotColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler         // handlers for customized model modification.
}

// CatalogCostSnapshotColumns defines and stores column names for the table plugin_idc_core_catalog_cost_snapshot.
type CatalogCostSnapshotColumns struct {
	Id               string //
	TenantId         string //
	CostSnapshotNo   string //
	BindingNo        string //
	SourceCode       string //
	SourceSnapshotNo string //
	CurrencyCode     string //
	AmountMinor      string //
	PeriodUnitCode   string //
	PeriodCount      string //
	ObservedAt       string //
	ValidUntil       string //
	CreatedBy        string //
	CreatedAt        string //
}

// catalogCostSnapshotColumns holds the columns for the table plugin_idc_core_catalog_cost_snapshot.
var catalogCostSnapshotColumns = CatalogCostSnapshotColumns{
	Id:               "id",
	TenantId:         "tenant_id",
	CostSnapshotNo:   "cost_snapshot_no",
	BindingNo:        "binding_no",
	SourceCode:       "source_code",
	SourceSnapshotNo: "source_snapshot_no",
	CurrencyCode:     "currency_code",
	AmountMinor:      "amount_minor",
	PeriodUnitCode:   "period_unit_code",
	PeriodCount:      "period_count",
	ObservedAt:       "observed_at",
	ValidUntil:       "valid_until",
	CreatedBy:        "created_by",
	CreatedAt:        "created_at",
}

// NewCatalogCostSnapshotDao creates and returns a new DAO object for table data access.
func NewCatalogCostSnapshotDao(handlers ...gdb.ModelHandler) *CatalogCostSnapshotDao {
	return &CatalogCostSnapshotDao{
		group:    "default",
		table:    "plugin_idc_core_catalog_cost_snapshot",
		columns:  catalogCostSnapshotColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CatalogCostSnapshotDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CatalogCostSnapshotDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CatalogCostSnapshotDao) Columns() CatalogCostSnapshotColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CatalogCostSnapshotDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CatalogCostSnapshotDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CatalogCostSnapshotDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
