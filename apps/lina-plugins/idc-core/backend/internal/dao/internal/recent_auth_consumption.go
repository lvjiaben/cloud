// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// RecentAuthConsumptionDao is the data access object for the table plugin_idc_core_recent_auth_consumption.
type RecentAuthConsumptionDao struct {
	table    string                       // table is the underlying table name of the DAO.
	group    string                       // group is the database configuration group name of the current DAO.
	columns  RecentAuthConsumptionColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler           // handlers for customized model modification.
}

// RecentAuthConsumptionColumns defines and stores column names for the table plugin_idc_core_recent_auth_consumption.
type RecentAuthConsumptionColumns struct {
	Id            string //
	TenantId      string //
	NonceDigest   string // Domain-separated SHA-256 digest of a random proof nonce; raw nonce and proof are never persisted
	PurposeCode   string //
	TargetDigest  string // Digest of the immutable dangerous-operation target or preview
	SessionDigest string // Digest binding to the host session without persisting its raw identifier
	UserId        string //
	LoginAt       string //
	IssuedAt      string //
	ExpiresAt     string //
	ConsumedAt    string //
}

// recentAuthConsumptionColumns holds the columns for the table plugin_idc_core_recent_auth_consumption.
var recentAuthConsumptionColumns = RecentAuthConsumptionColumns{
	Id:            "id",
	TenantId:      "tenant_id",
	NonceDigest:   "nonce_digest",
	PurposeCode:   "purpose_code",
	TargetDigest:  "target_digest",
	SessionDigest: "session_digest",
	UserId:        "user_id",
	LoginAt:       "login_at",
	IssuedAt:      "issued_at",
	ExpiresAt:     "expires_at",
	ConsumedAt:    "consumed_at",
}

// NewRecentAuthConsumptionDao creates and returns a new DAO object for table data access.
func NewRecentAuthConsumptionDao(handlers ...gdb.ModelHandler) *RecentAuthConsumptionDao {
	return &RecentAuthConsumptionDao{
		group:    "default",
		table:    "plugin_idc_core_recent_auth_consumption",
		columns:  recentAuthConsumptionColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *RecentAuthConsumptionDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *RecentAuthConsumptionDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *RecentAuthConsumptionDao) Columns() RecentAuthConsumptionColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *RecentAuthConsumptionDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *RecentAuthConsumptionDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *RecentAuthConsumptionDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
