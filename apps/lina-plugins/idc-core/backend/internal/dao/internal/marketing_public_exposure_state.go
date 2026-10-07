// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// MarketingPublicExposureStateDao is the data access object for the table plugin_idc_core_marketing_public_exposure_state.
type MarketingPublicExposureStateDao struct {
	table    string                              // table is the underlying table name of the DAO.
	group    string                              // group is the database configuration group name of the current DAO.
	columns  MarketingPublicExposureStateColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                  // handlers for customized model modification.
}

// MarketingPublicExposureStateColumns defines and stores column names for the table plugin_idc_core_marketing_public_exposure_state.
type MarketingPublicExposureStateColumns struct {
	Id              string //
	TenantId        string //
	DeviceDigest    string //
	CampaignNo      string //
	CurrencyCode    string //
	WindowStartedAt string //
	CreatedAt       string //
	UpdatedAt       string //
}

// marketingPublicExposureStateColumns holds the columns for the table plugin_idc_core_marketing_public_exposure_state.
var marketingPublicExposureStateColumns = MarketingPublicExposureStateColumns{
	Id:              "id",
	TenantId:        "tenant_id",
	DeviceDigest:    "device_digest",
	CampaignNo:      "campaign_no",
	CurrencyCode:    "currency_code",
	WindowStartedAt: "window_started_at",
	CreatedAt:       "created_at",
	UpdatedAt:       "updated_at",
}

// NewMarketingPublicExposureStateDao creates and returns a new DAO object for table data access.
func NewMarketingPublicExposureStateDao(handlers ...gdb.ModelHandler) *MarketingPublicExposureStateDao {
	return &MarketingPublicExposureStateDao{
		group:    "default",
		table:    "plugin_idc_core_marketing_public_exposure_state",
		columns:  marketingPublicExposureStateColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *MarketingPublicExposureStateDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *MarketingPublicExposureStateDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *MarketingPublicExposureStateDao) Columns() MarketingPublicExposureStateColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *MarketingPublicExposureStateDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *MarketingPublicExposureStateDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *MarketingPublicExposureStateDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
