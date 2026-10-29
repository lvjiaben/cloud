// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// PurchaseServerLoginDao is the data access object for the table plugin_idc_core_purchase_server_login.
type PurchaseServerLoginDao struct {
	table    string                     // table is the underlying table name of the DAO.
	group    string                     // group is the database configuration group name of the current DAO.
	columns  PurchaseServerLoginColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler         // handlers for customized model modification.
}

// PurchaseServerLoginColumns defines and stores column names for the table plugin_idc_core_purchase_server_login.
type PurchaseServerLoginColumns struct {
	Id                    string //
	TenantId              string //
	ServerLoginToken      string //
	CustomerNo            string //
	UserId                string //
	ProductVersionNo      string //
	ConfigurationRevision string //
	CartItemNo            string //
	Hostname              string //
	PasswordSecretRef     string // Opaque resource-channel-scoped secretcap handle, never plaintext
	ChannelNo             string //
	CreatedAt             string //
}

// purchaseServerLoginColumns holds the columns for the table plugin_idc_core_purchase_server_login.
var purchaseServerLoginColumns = PurchaseServerLoginColumns{
	Id:                    "id",
	TenantId:              "tenant_id",
	ServerLoginToken:      "server_login_token",
	CustomerNo:            "customer_no",
	UserId:                "user_id",
	ProductVersionNo:      "product_version_no",
	ConfigurationRevision: "configuration_revision",
	CartItemNo:            "cart_item_no",
	Hostname:              "hostname",
	PasswordSecretRef:     "password_secret_ref",
	ChannelNo:             "channel_no",
	CreatedAt:             "created_at",
}

// NewPurchaseServerLoginDao creates and returns a new DAO object for table data access.
func NewPurchaseServerLoginDao(handlers ...gdb.ModelHandler) *PurchaseServerLoginDao {
	return &PurchaseServerLoginDao{
		group:    "default",
		table:    "plugin_idc_core_purchase_server_login",
		columns:  purchaseServerLoginColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *PurchaseServerLoginDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *PurchaseServerLoginDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *PurchaseServerLoginDao) Columns() PurchaseServerLoginColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *PurchaseServerLoginDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *PurchaseServerLoginDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *PurchaseServerLoginDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
