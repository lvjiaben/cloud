// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// MarketingPromotionAssetDao is the data access object for the table plugin_idc_core_marketing_promotion_asset.
type MarketingPromotionAssetDao struct {
	table    string                         // table is the underlying table name of the DAO.
	group    string                         // group is the database configuration group name of the current DAO.
	columns  MarketingPromotionAssetColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler             // handlers for customized model modification.
}

// MarketingPromotionAssetColumns defines and stores column names for the table plugin_idc_core_marketing_promotion_asset.
type MarketingPromotionAssetColumns struct {
	Id                    string //
	TenantId              string //
	AssetNo               string //
	CampaignNo            string //
	BenefitNo             string //
	BenefitTypeCode       string //
	AssetTypeCode         string //
	CustomerNo            string //
	ClaimCodeDigest       string //
	BenefitSnapshotDigest string //
	CurrencyCode          string //
	FaceAmountMinor       string //
	RateBasisPoints       string //
	UsageLimit            string //
	ReservedUses          string //
	ConsumedUses          string //
	ValidFrom             string //
	ValidUntil            string //
	StatusCode            string //
	ClaimedAt             string //
	CreatedAt             string //
	UpdatedAt             string //
	CouponBatchNo         string //
}

// marketingPromotionAssetColumns holds the columns for the table plugin_idc_core_marketing_promotion_asset.
var marketingPromotionAssetColumns = MarketingPromotionAssetColumns{
	Id:                    "id",
	TenantId:              "tenant_id",
	AssetNo:               "asset_no",
	CampaignNo:            "campaign_no",
	BenefitNo:             "benefit_no",
	BenefitTypeCode:       "benefit_type_code",
	AssetTypeCode:         "asset_type_code",
	CustomerNo:            "customer_no",
	ClaimCodeDigest:       "claim_code_digest",
	BenefitSnapshotDigest: "benefit_snapshot_digest",
	CurrencyCode:          "currency_code",
	FaceAmountMinor:       "face_amount_minor",
	RateBasisPoints:       "rate_basis_points",
	UsageLimit:            "usage_limit",
	ReservedUses:          "reserved_uses",
	ConsumedUses:          "consumed_uses",
	ValidFrom:             "valid_from",
	ValidUntil:            "valid_until",
	StatusCode:            "status_code",
	ClaimedAt:             "claimed_at",
	CreatedAt:             "created_at",
	UpdatedAt:             "updated_at",
	CouponBatchNo:         "coupon_batch_no",
}

// NewMarketingPromotionAssetDao creates and returns a new DAO object for table data access.
func NewMarketingPromotionAssetDao(handlers ...gdb.ModelHandler) *MarketingPromotionAssetDao {
	return &MarketingPromotionAssetDao{
		group:    "default",
		table:    "plugin_idc_core_marketing_promotion_asset",
		columns:  marketingPromotionAssetColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *MarketingPromotionAssetDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *MarketingPromotionAssetDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *MarketingPromotionAssetDao) Columns() MarketingPromotionAssetColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *MarketingPromotionAssetDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *MarketingPromotionAssetDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *MarketingPromotionAssetDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
