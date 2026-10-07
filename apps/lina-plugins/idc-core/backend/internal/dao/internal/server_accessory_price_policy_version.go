// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ServerAccessoryPricePolicyVersionDao is the data access object for the table plugin_idc_core_server_accessory_price_policy_version.
type ServerAccessoryPricePolicyVersionDao struct {
	table    string                                   // table is the underlying table name of the DAO.
	group    string                                   // group is the database configuration group name of the current DAO.
	columns  ServerAccessoryPricePolicyVersionColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                       // handlers for customized model modification.
}

// ServerAccessoryPricePolicyVersionColumns defines and stores column names for the table plugin_idc_core_server_accessory_price_policy_version.
type ServerAccessoryPricePolicyVersionColumns struct {
	Id                 string //
	TenantId           string //
	PolicyNo           string //
	PolicyVersion      string //
	AccessoryKind      string //
	CurrencyCode       string //
	FreeCountPerServer string //
	UnitAmountMinor    string //
	PeriodUnitCode     string //
	PeriodCount        string //
	ContentDigest      string //
	PublishedBy        string //
	PublishedAt        string //
	TrafficGb          string //
	NameZhCn           string //
	NameEnUs           string //
}

// serverAccessoryPricePolicyVersionColumns holds the columns for the table plugin_idc_core_server_accessory_price_policy_version.
var serverAccessoryPricePolicyVersionColumns = ServerAccessoryPricePolicyVersionColumns{
	Id:                 "id",
	TenantId:           "tenant_id",
	PolicyNo:           "policy_no",
	PolicyVersion:      "policy_version",
	AccessoryKind:      "accessory_kind",
	CurrencyCode:       "currency_code",
	FreeCountPerServer: "free_count_per_server",
	UnitAmountMinor:    "unit_amount_minor",
	PeriodUnitCode:     "period_unit_code",
	PeriodCount:        "period_count",
	ContentDigest:      "content_digest",
	PublishedBy:        "published_by",
	PublishedAt:        "published_at",
	TrafficGb:          "traffic_gb",
	NameZhCn:           "name_zh_cn",
	NameEnUs:           "name_en_us",
}

// NewServerAccessoryPricePolicyVersionDao creates and returns a new DAO object for table data access.
func NewServerAccessoryPricePolicyVersionDao(handlers ...gdb.ModelHandler) *ServerAccessoryPricePolicyVersionDao {
	return &ServerAccessoryPricePolicyVersionDao{
		group:    "default",
		table:    "plugin_idc_core_server_accessory_price_policy_version",
		columns:  serverAccessoryPricePolicyVersionColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ServerAccessoryPricePolicyVersionDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ServerAccessoryPricePolicyVersionDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ServerAccessoryPricePolicyVersionDao) Columns() ServerAccessoryPricePolicyVersionColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ServerAccessoryPricePolicyVersionDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ServerAccessoryPricePolicyVersionDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ServerAccessoryPricePolicyVersionDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
