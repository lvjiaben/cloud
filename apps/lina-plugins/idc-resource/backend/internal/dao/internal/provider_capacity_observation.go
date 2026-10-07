// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ProviderCapacityObservationDao is the data access object for the table plugin_idc_resource_provider_capacity_observation.
type ProviderCapacityObservationDao struct {
	table    string                             // table is the underlying table name of the DAO.
	group    string                             // group is the database configuration group name of the current DAO.
	columns  ProviderCapacityObservationColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                 // handlers for customized model modification.
}

// ProviderCapacityObservationColumns defines and stores column names for the table plugin_idc_resource_provider_capacity_observation.
type ProviderCapacityObservationColumns struct {
	Id                     string //
	TenantId               string //
	ObservationNo          string //
	ChannelNo              string //
	CapacityKind           string //
	StatusCode             string //
	ObservationFingerprint string //
	ItemCount              string //
	ObservedAt             string //
	CompletedAt            string //
	CreatedAt              string //
	PageSize               string // Fixed Provider page size for the exact collecting observation
	NextCursor             string // Exact opaque Provider cursor required for the next staged page
	PageCount              string // Number of validated pages durably staged into this observation
	TriggerCode            string // Trusted operation source: system or direct staff manual request
	RequestedBy            string // Trusted host staff user identifier for a manual request; zero for system work
	AttemptCount           string // Bounded number of claimed Provider page attempts for this observation
	NextAttemptAt          string // PostgreSQL due time for the next single-page attempt; null means not scheduled
	LeaseTokenDigest       string // SHA-256 digest of an ephemeral worker lease token; the token itself is never stored
	LeaseExpiresAt         string // PostgreSQL time after which another worker may reclaim this collecting observation
	FailureCode            string // Stable non-secret terminal failure code; raw Provider errors are forbidden
}

// providerCapacityObservationColumns holds the columns for the table plugin_idc_resource_provider_capacity_observation.
var providerCapacityObservationColumns = ProviderCapacityObservationColumns{
	Id:                     "id",
	TenantId:               "tenant_id",
	ObservationNo:          "observation_no",
	ChannelNo:              "channel_no",
	CapacityKind:           "capacity_kind",
	StatusCode:             "status_code",
	ObservationFingerprint: "observation_fingerprint",
	ItemCount:              "item_count",
	ObservedAt:             "observed_at",
	CompletedAt:            "completed_at",
	CreatedAt:              "created_at",
	PageSize:               "page_size",
	NextCursor:             "next_cursor",
	PageCount:              "page_count",
	TriggerCode:            "trigger_code",
	RequestedBy:            "requested_by",
	AttemptCount:           "attempt_count",
	NextAttemptAt:          "next_attempt_at",
	LeaseTokenDigest:       "lease_token_digest",
	LeaseExpiresAt:         "lease_expires_at",
	FailureCode:            "failure_code",
}

// NewProviderCapacityObservationDao creates and returns a new DAO object for table data access.
func NewProviderCapacityObservationDao(handlers ...gdb.ModelHandler) *ProviderCapacityObservationDao {
	return &ProviderCapacityObservationDao{
		group:    "default",
		table:    "plugin_idc_resource_provider_capacity_observation",
		columns:  providerCapacityObservationColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ProviderCapacityObservationDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ProviderCapacityObservationDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ProviderCapacityObservationDao) Columns() ProviderCapacityObservationColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ProviderCapacityObservationDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ProviderCapacityObservationDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ProviderCapacityObservationDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
