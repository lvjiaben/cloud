// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ServerAccessoryPricePolicyDao is the data access object for the table plugin_idc_core_server_accessory_price_policy.
type ServerAccessoryPricePolicyDao struct {
	table    string                            // table is the underlying table name of the DAO.
	group    string                            // group is the database configuration group name of the current DAO.
	columns  ServerAccessoryPricePolicyColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                // handlers for customized model modification.
}

// ServerAccessoryPricePolicyColumns defines and stores column names for the table plugin_idc_core_server_accessory_price_policy.
type ServerAccessoryPricePolicyColumns struct {
	Id             string //
	TenantId       string //
	PolicyNo       string //
	AccessoryKind  string //
	CurrencyCode   string //
	CurrentVersion string //
	CreatedAt      string //
	UpdatedAt      string //
	Enabled        string //
	StockQuantity  string //
}

// serverAccessoryPricePolicyColumns holds the columns for the table plugin_idc_core_server_accessory_price_policy.
var serverAccessoryPricePolicyColumns = ServerAccessoryPricePolicyColumns{
	Id:             "id",
	TenantId:       "tenant_id",
	PolicyNo:       "policy_no",
	AccessoryKind:  "accessory_kind",
	CurrencyCode:   "currency_code",
	CurrentVersion: "current_version",
	CreatedAt:      "created_at",
	UpdatedAt:      "updated_at",
	Enabled:        "enabled",
	StockQuantity:  "stock_quantity",
}

// NewServerAccessoryPricePolicyDao creates and returns a new DAO object for table data access.
func NewServerAccessoryPricePolicyDao(handlers ...gdb.ModelHandler) *ServerAccessoryPricePolicyDao {
	return &ServerAccessoryPricePolicyDao{
		group:    "default",
		table:    "plugin_idc_core_server_accessory_price_policy",
		columns:  serverAccessoryPricePolicyColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ServerAccessoryPricePolicyDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ServerAccessoryPricePolicyDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ServerAccessoryPricePolicyDao) Columns() ServerAccessoryPricePolicyColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ServerAccessoryPricePolicyDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ServerAccessoryPricePolicyDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ServerAccessoryPricePolicyDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
