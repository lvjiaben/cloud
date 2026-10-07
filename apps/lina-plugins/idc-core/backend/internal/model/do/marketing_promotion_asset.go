// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// MarketingPromotionAsset is the golang structure of table plugin_idc_core_marketing_promotion_asset for DAO operations like Where/Data.
type MarketingPromotionAsset struct {
	g.Meta                `orm:"table:plugin_idc_core_marketing_promotion_asset, do:true"`
	Id                    any        //
	TenantId              any        //
	AssetNo               any        //
	CampaignNo            any        //
	BenefitNo             any        //
	BenefitTypeCode       any        //
	AssetTypeCode         any        //
	CustomerNo            *string    //
	ClaimCodeDigest       []byte     //
	BenefitSnapshotDigest any        //
	CurrencyCode          any        //
	FaceAmountMinor       *int64     //
	RateBasisPoints       *int       //
	UsageLimit            any        //
	ReservedUses          any        //
	ConsumedUses          any        //
	ValidFrom             *time.Time //
	ValidUntil            *time.Time //
	StatusCode            any        //
	ClaimedAt             *time.Time //
	CreatedAt             *time.Time //
	UpdatedAt             *time.Time //
	CouponBatchNo         any        //
}
