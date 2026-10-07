// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// MarketingCampaignReservationDao is the data access object for the table plugin_idc_core_marketing_campaign_reservation.
type MarketingCampaignReservationDao struct {
	table    string                              // table is the underlying table name of the DAO.
	group    string                              // group is the database configuration group name of the current DAO.
	columns  MarketingCampaignReservationColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                  // handlers for customized model modification.
}

// MarketingCampaignReservationColumns defines and stores column names for the table plugin_idc_core_marketing_campaign_reservation.
type MarketingCampaignReservationColumns struct {
	Id                    string //
	TenantId              string //
	ReservationNo         string //
	CampaignNo            string //
	BenefitNo             string //
	CustomerNo            string //
	QuoteReference        string //
	RequestSnapshotDigest string //
	CurrencyCode          string //
	ReservedUnits         string //
	QuotedValueMinor      string //
	BenefitSnapshotDigest string //
	StatusCode            string //
	ExpiresAt             string //
	ConsumedAt            string //
	ReleasedAt            string //
	CreatedAt             string //
	UpdatedAt             string //
	ReversedAt            string //
}

// marketingCampaignReservationColumns holds the columns for the table plugin_idc_core_marketing_campaign_reservation.
var marketingCampaignReservationColumns = MarketingCampaignReservationColumns{
	Id:                    "id",
	TenantId:              "tenant_id",
	ReservationNo:         "reservation_no",
	CampaignNo:            "campaign_no",
	BenefitNo:             "benefit_no",
	CustomerNo:            "customer_no",
	QuoteReference:        "quote_reference",
	RequestSnapshotDigest: "request_snapshot_digest",
	CurrencyCode:          "currency_code",
	ReservedUnits:         "reserved_units",
	QuotedValueMinor:      "quoted_value_minor",
	BenefitSnapshotDigest: "benefit_snapshot_digest",
	StatusCode:            "status_code",
	ExpiresAt:             "expires_at",
	ConsumedAt:            "consumed_at",
	ReleasedAt:            "released_at",
	CreatedAt:             "created_at",
	UpdatedAt:             "updated_at",
	ReversedAt:            "reversed_at",
}

// NewMarketingCampaignReservationDao creates and returns a new DAO object for table data access.
func NewMarketingCampaignReservationDao(handlers ...gdb.ModelHandler) *MarketingCampaignReservationDao {
	return &MarketingCampaignReservationDao{
		group:    "default",
		table:    "plugin_idc_core_marketing_campaign_reservation",
		columns:  marketingCampaignReservationColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *MarketingCampaignReservationDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *MarketingCampaignReservationDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *MarketingCampaignReservationDao) Columns() MarketingCampaignReservationColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *MarketingCampaignReservationDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *MarketingCampaignReservationDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *MarketingCampaignReservationDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
