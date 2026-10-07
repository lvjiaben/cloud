// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// MarketingPointsLotDao is the data access object for the table plugin_idc_core_marketing_points_lot.
type MarketingPointsLotDao struct {
	table    string                    // table is the underlying table name of the DAO.
	group    string                    // group is the database configuration group name of the current DAO.
	columns  MarketingPointsLotColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler        // handlers for customized model modification.
}

// MarketingPointsLotColumns defines and stores column names for the table plugin_idc_core_marketing_points_lot.
type MarketingPointsLotColumns struct {
	Id                      string //
	TenantId                string //
	LotNo                   string //
	CustomerNo              string //
	CampaignNo              string //
	CampaignTypeCode        string //
	SourceTypeCode          string //
	SourceReference         string //
	OriginalPoints          string //
	AvailablePoints         string //
	FrozenPoints            string //
	ConsumedPoints          string //
	ExpiredPoints           string //
	ReversedPoints          string //
	ReversalShortfallPoints string // Cumulative refunded award points already consumed and therefore represented as account debt
	ExpiresAt               string //
	StatusCode              string //
	CreatedAt               string //
	UpdatedAt               string //
}

// marketingPointsLotColumns holds the columns for the table plugin_idc_core_marketing_points_lot.
var marketingPointsLotColumns = MarketingPointsLotColumns{
	Id:                      "id",
	TenantId:                "tenant_id",
	LotNo:                   "lot_no",
	CustomerNo:              "customer_no",
	CampaignNo:              "campaign_no",
	CampaignTypeCode:        "campaign_type_code",
	SourceTypeCode:          "source_type_code",
	SourceReference:         "source_reference",
	OriginalPoints:          "original_points",
	AvailablePoints:         "available_points",
	FrozenPoints:            "frozen_points",
	ConsumedPoints:          "consumed_points",
	ExpiredPoints:           "expired_points",
	ReversedPoints:          "reversed_points",
	ReversalShortfallPoints: "reversal_shortfall_points",
	ExpiresAt:               "expires_at",
	StatusCode:              "status_code",
	CreatedAt:               "created_at",
	UpdatedAt:               "updated_at",
}

// NewMarketingPointsLotDao creates and returns a new DAO object for table data access.
func NewMarketingPointsLotDao(handlers ...gdb.ModelHandler) *MarketingPointsLotDao {
	return &MarketingPointsLotDao{
		group:    "default",
		table:    "plugin_idc_core_marketing_points_lot",
		columns:  marketingPointsLotColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *MarketingPointsLotDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *MarketingPointsLotDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *MarketingPointsLotDao) Columns() MarketingPointsLotColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *MarketingPointsLotDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *MarketingPointsLotDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *MarketingPointsLotDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
