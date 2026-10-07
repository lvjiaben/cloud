// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// BusinessNumberDao is the data access object for the table plugin_idc_core_business_number.
type BusinessNumberDao struct {
	table    string                // table is the underlying table name of the DAO.
	group    string                // group is the database configuration group name of the current DAO.
	columns  BusinessNumberColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler    // handlers for customized model modification.
}

// BusinessNumberColumns defines and stores column names for the table plugin_idc_core_business_number.
type BusinessNumberColumns struct {
	Id                     string //
	TenantId               string //
	NumberType             string // Stable business number namespace
	BusinessNo             string // Cryptographically random opaque business number
	ReservationScopeDigest string // Keyed digest of the normalized caller or command scope; audit reservations use the stable installation lookup key
	IdempotencyKeyDigest   string // Keyed digest of the reservation idempotency key; audit reservations use the stable installation lookup key
	OwnerType              string // Owning aggregate type after assignment
	OwnerInternalId        string // Internal owner ID that is never exposed across plugins
	AssignedAt             string //
	CreatedAt              string //
	UpdatedAt              string //
}

// businessNumberColumns holds the columns for the table plugin_idc_core_business_number.
var businessNumberColumns = BusinessNumberColumns{
	Id:                     "id",
	TenantId:               "tenant_id",
	NumberType:             "number_type",
	BusinessNo:             "business_no",
	ReservationScopeDigest: "reservation_scope_digest",
	IdempotencyKeyDigest:   "idempotency_key_digest",
	OwnerType:              "owner_type",
	OwnerInternalId:        "owner_internal_id",
	AssignedAt:             "assigned_at",
	CreatedAt:              "created_at",
	UpdatedAt:              "updated_at",
}

// NewBusinessNumberDao creates and returns a new DAO object for table data access.
func NewBusinessNumberDao(handlers ...gdb.ModelHandler) *BusinessNumberDao {
	return &BusinessNumberDao{
		group:    "default",
		table:    "plugin_idc_core_business_number",
		columns:  businessNumberColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *BusinessNumberDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *BusinessNumberDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *BusinessNumberDao) Columns() BusinessNumberColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *BusinessNumberDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *BusinessNumberDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *BusinessNumberDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
