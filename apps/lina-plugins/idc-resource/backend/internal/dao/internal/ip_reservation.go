// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// IpReservationDao is the data access object for the table plugin_idc_resource_ip_reservation.
type IpReservationDao struct {
	table    string               // table is the underlying table name of the DAO.
	group    string               // group is the database configuration group name of the current DAO.
	columns  IpReservationColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler   // handlers for customized model modification.
}

// IpReservationColumns defines and stores column names for the table plugin_idc_resource_ip_reservation.
type IpReservationColumns struct {
	Id                    string //
	TenantId              string //
	ReservationNo         string //
	ServiceNo             string //
	ChannelNo             string //
	PoolNo                string //
	CapacityObservationNo string //
	CapacityKind          string //
	Quantity              string //
	StatusCode            string //
	IdempotencyDigest     string //
	RequestDigest         string //
	ProviderTaskNo        string //
	ReleaseReasonCode     string //
	ExpiresAt             string //
	ConsumedAt            string //
	ReleasedAt            string //
	CreatedAt             string //
	UpdatedAt             string //
	Version               string //
	CommercialOperationNo string //
}

// ipReservationColumns holds the columns for the table plugin_idc_resource_ip_reservation.
var ipReservationColumns = IpReservationColumns{
	Id:                    "id",
	TenantId:              "tenant_id",
	ReservationNo:         "reservation_no",
	ServiceNo:             "service_no",
	ChannelNo:             "channel_no",
	PoolNo:                "pool_no",
	CapacityObservationNo: "capacity_observation_no",
	CapacityKind:          "capacity_kind",
	Quantity:              "quantity",
	StatusCode:            "status_code",
	IdempotencyDigest:     "idempotency_digest",
	RequestDigest:         "request_digest",
	ProviderTaskNo:        "provider_task_no",
	ReleaseReasonCode:     "release_reason_code",
	ExpiresAt:             "expires_at",
	ConsumedAt:            "consumed_at",
	ReleasedAt:            "released_at",
	CreatedAt:             "created_at",
	UpdatedAt:             "updated_at",
	Version:               "version",
	CommercialOperationNo: "commercial_operation_no",
}

// NewIpReservationDao creates and returns a new DAO object for table data access.
func NewIpReservationDao(handlers ...gdb.ModelHandler) *IpReservationDao {
	return &IpReservationDao{
		group:    "default",
		table:    "plugin_idc_resource_ip_reservation",
		columns:  ipReservationColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *IpReservationDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *IpReservationDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *IpReservationDao) Columns() IpReservationColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *IpReservationDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *IpReservationDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *IpReservationDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
