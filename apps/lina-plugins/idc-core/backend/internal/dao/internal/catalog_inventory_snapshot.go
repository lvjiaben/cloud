// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CatalogInventorySnapshotDao is the data access object for the table plugin_idc_core_catalog_inventory_snapshot.
type CatalogInventorySnapshotDao struct {
	table    string                          // table is the underlying table name of the DAO.
	group    string                          // group is the database configuration group name of the current DAO.
	columns  CatalogInventorySnapshotColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler              // handlers for customized model modification.
}

// CatalogInventorySnapshotColumns defines and stores column names for the table plugin_idc_core_catalog_inventory_snapshot.
type CatalogInventorySnapshotColumns struct {
	Id                string //
	TenantId          string //
	SnapshotNo        string //
	BindingNo         string //
	SourceSnapshotNo  string //
	SourceFingerprint string //
	AvailabilityCode  string //
	AvailableQuantity string //
	SafetyMargin      string //
	SellableQuantity  string //
	ObservedAt        string //
	ExpiresAt         string //
	CreatedAt         string //
	SourceCompletedAt string // Completion time of the authoritative source snapshot used for this local projection
}

// catalogInventorySnapshotColumns holds the columns for the table plugin_idc_core_catalog_inventory_snapshot.
var catalogInventorySnapshotColumns = CatalogInventorySnapshotColumns{
	Id:                "id",
	TenantId:          "tenant_id",
	SnapshotNo:        "snapshot_no",
	BindingNo:         "binding_no",
	SourceSnapshotNo:  "source_snapshot_no",
	SourceFingerprint: "source_fingerprint",
	AvailabilityCode:  "availability_code",
	AvailableQuantity: "available_quantity",
	SafetyMargin:      "safety_margin",
	SellableQuantity:  "sellable_quantity",
	ObservedAt:        "observed_at",
	ExpiresAt:         "expires_at",
	CreatedAt:         "created_at",
	SourceCompletedAt: "source_completed_at",
}

// NewCatalogInventorySnapshotDao creates and returns a new DAO object for table data access.
func NewCatalogInventorySnapshotDao(handlers ...gdb.ModelHandler) *CatalogInventorySnapshotDao {
	return &CatalogInventorySnapshotDao{
		group:    "default",
		table:    "plugin_idc_core_catalog_inventory_snapshot",
		columns:  catalogInventorySnapshotColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CatalogInventorySnapshotDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CatalogInventorySnapshotDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CatalogInventorySnapshotDao) Columns() CatalogInventorySnapshotColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CatalogInventorySnapshotDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CatalogInventorySnapshotDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CatalogInventorySnapshotDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
