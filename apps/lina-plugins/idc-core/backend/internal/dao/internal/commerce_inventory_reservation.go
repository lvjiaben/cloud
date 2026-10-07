// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CommerceInventoryReservationDao is the data access object for the table plugin_idc_core_commerce_inventory_reservation.
type CommerceInventoryReservationDao struct {
	table    string                              // table is the underlying table name of the DAO.
	group    string                              // group is the database configuration group name of the current DAO.
	columns  CommerceInventoryReservationColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                  // handlers for customized model modification.
}

// CommerceInventoryReservationColumns defines and stores column names for the table plugin_idc_core_commerce_inventory_reservation.
type CommerceInventoryReservationColumns struct {
	Id                  string //
	TenantId            string //
	ReservationNo       string //
	OrderNo             string //
	OrderItemNo         string //
	BindingNo           string //
	InventorySnapshotNo string //
	ReservedQuantity    string //
	StatusCode          string //
	ReleaseReasonCode   string //
	ContentDigest       string //
	HoldExpiresAt       string //
	CommittedAt         string //
	ConsumedAt          string //
	ReleasedAt          string //
	Version             string //
	CreatedAt           string //
	UpdatedAt           string //
	ConsumedQuantity    string //
	ReleasedQuantity    string //
	VersionNo           string //
	InventoryVersion    string //
}

// commerceInventoryReservationColumns holds the columns for the table plugin_idc_core_commerce_inventory_reservation.
var commerceInventoryReservationColumns = CommerceInventoryReservationColumns{
	Id:                  "id",
	TenantId:            "tenant_id",
	ReservationNo:       "reservation_no",
	OrderNo:             "order_no",
	OrderItemNo:         "order_item_no",
	BindingNo:           "binding_no",
	InventorySnapshotNo: "inventory_snapshot_no",
	ReservedQuantity:    "reserved_quantity",
	StatusCode:          "status_code",
	ReleaseReasonCode:   "release_reason_code",
	ContentDigest:       "content_digest",
	HoldExpiresAt:       "hold_expires_at",
	CommittedAt:         "committed_at",
	ConsumedAt:          "consumed_at",
	ReleasedAt:          "released_at",
	Version:             "version",
	CreatedAt:           "created_at",
	UpdatedAt:           "updated_at",
	ConsumedQuantity:    "consumed_quantity",
	ReleasedQuantity:    "released_quantity",
	VersionNo:           "version_no",
	InventoryVersion:    "inventory_version",
}

// NewCommerceInventoryReservationDao creates and returns a new DAO object for table data access.
func NewCommerceInventoryReservationDao(handlers ...gdb.ModelHandler) *CommerceInventoryReservationDao {
	return &CommerceInventoryReservationDao{
		group:    "default",
		table:    "plugin_idc_core_commerce_inventory_reservation",
		columns:  commerceInventoryReservationColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CommerceInventoryReservationDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CommerceInventoryReservationDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CommerceInventoryReservationDao) Columns() CommerceInventoryReservationColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CommerceInventoryReservationDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CommerceInventoryReservationDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CommerceInventoryReservationDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
