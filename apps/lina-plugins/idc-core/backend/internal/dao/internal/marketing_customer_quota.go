// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// MarketingCustomerQuotaDao is the data access object for the table plugin_idc_core_marketing_customer_quota.
type MarketingCustomerQuotaDao struct {
	table    string                        // table is the underlying table name of the DAO.
	group    string                        // group is the database configuration group name of the current DAO.
	columns  MarketingCustomerQuotaColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler            // handlers for customized model modification.
}

// MarketingCustomerQuotaColumns defines and stores column names for the table plugin_idc_core_marketing_customer_quota.
type MarketingCustomerQuotaColumns struct {
	Id            string //
	TenantId      string //
	CampaignNo    string //
	CustomerNo    string //
	QuotaLimit    string //
	ReservedCount string //
	ConsumedCount string //
	Version       string //
	CreatedAt     string //
	UpdatedAt     string //
}

// marketingCustomerQuotaColumns holds the columns for the table plugin_idc_core_marketing_customer_quota.
var marketingCustomerQuotaColumns = MarketingCustomerQuotaColumns{
	Id:            "id",
	TenantId:      "tenant_id",
	CampaignNo:    "campaign_no",
	CustomerNo:    "customer_no",
	QuotaLimit:    "quota_limit",
	ReservedCount: "reserved_count",
	ConsumedCount: "consumed_count",
	Version:       "version",
	CreatedAt:     "created_at",
	UpdatedAt:     "updated_at",
}

// NewMarketingCustomerQuotaDao creates and returns a new DAO object for table data access.
func NewMarketingCustomerQuotaDao(handlers ...gdb.ModelHandler) *MarketingCustomerQuotaDao {
	return &MarketingCustomerQuotaDao{
		group:    "default",
		table:    "plugin_idc_core_marketing_customer_quota",
		columns:  marketingCustomerQuotaColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *MarketingCustomerQuotaDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *MarketingCustomerQuotaDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *MarketingCustomerQuotaDao) Columns() MarketingCustomerQuotaColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *MarketingCustomerQuotaDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *MarketingCustomerQuotaDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *MarketingCustomerQuotaDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
