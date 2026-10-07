// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// MarketingChannelConsentDao is the data access object for the table plugin_idc_core_marketing_channel_consent.
type MarketingChannelConsentDao struct {
	table    string                         // table is the underlying table name of the DAO.
	group    string                         // group is the database configuration group name of the current DAO.
	columns  MarketingChannelConsentColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler             // handlers for customized model modification.
}

// MarketingChannelConsentColumns defines and stores column names for the table plugin_idc_core_marketing_channel_consent.
type MarketingChannelConsentColumns struct {
	Id                string //
	TenantId          string //
	CustomerNo        string //
	ContactNo         string //
	ChannelCode       string //
	StatusCode        string //
	CurrentEventNo    string //
	ConsentOccurredAt string //
	Version           string //
	CreatedAt         string //
	UpdatedAt         string //
}

// marketingChannelConsentColumns holds the columns for the table plugin_idc_core_marketing_channel_consent.
var marketingChannelConsentColumns = MarketingChannelConsentColumns{
	Id:                "id",
	TenantId:          "tenant_id",
	CustomerNo:        "customer_no",
	ContactNo:         "contact_no",
	ChannelCode:       "channel_code",
	StatusCode:        "status_code",
	CurrentEventNo:    "current_event_no",
	ConsentOccurredAt: "consent_occurred_at",
	Version:           "version",
	CreatedAt:         "created_at",
	UpdatedAt:         "updated_at",
}

// NewMarketingChannelConsentDao creates and returns a new DAO object for table data access.
func NewMarketingChannelConsentDao(handlers ...gdb.ModelHandler) *MarketingChannelConsentDao {
	return &MarketingChannelConsentDao{
		group:    "default",
		table:    "plugin_idc_core_marketing_channel_consent",
		columns:  marketingChannelConsentColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *MarketingChannelConsentDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *MarketingChannelConsentDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *MarketingChannelConsentDao) Columns() MarketingChannelConsentColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *MarketingChannelConsentDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *MarketingChannelConsentDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *MarketingChannelConsentDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
