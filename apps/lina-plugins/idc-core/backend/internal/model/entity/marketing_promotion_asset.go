// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// MarketingPromotionAsset is the golang structure for table marketing_promotion_asset.
type MarketingPromotionAsset struct {
	Id                    int64      `json:"id"                    orm:"id"                      description:""`
	TenantId              int        `json:"tenantId"              orm:"tenant_id"               description:""`
	AssetNo               string     `json:"assetNo"               orm:"asset_no"                description:""`
	CampaignNo            string     `json:"campaignNo"            orm:"campaign_no"             description:""`
	BenefitNo             string     `json:"benefitNo"             orm:"benefit_no"              description:""`
	BenefitTypeCode       string     `json:"benefitTypeCode"       orm:"benefit_type_code"       description:""`
	AssetTypeCode         string     `json:"assetTypeCode"         orm:"asset_type_code"         description:""`
	CustomerNo            *string    `json:"customerNo"            orm:"customer_no"             description:""`
	ClaimCodeDigest       []byte     `json:"claimCodeDigest"       orm:"claim_code_digest"       description:""`
	BenefitSnapshotDigest string     `json:"benefitSnapshotDigest" orm:"benefit_snapshot_digest" description:""`
	CurrencyCode          string     `json:"currencyCode"          orm:"currency_code"           description:""`
	FaceAmountMinor       *int64     `json:"faceAmountMinor"       orm:"face_amount_minor"       description:""`
	RateBasisPoints       *int       `json:"rateBasisPoints"       orm:"rate_basis_points"       description:""`
	UsageLimit            int        `json:"usageLimit"            orm:"usage_limit"             description:""`
	ReservedUses          int        `json:"reservedUses"          orm:"reserved_uses"           description:""`
	ConsumedUses          int        `json:"consumedUses"          orm:"consumed_uses"           description:""`
	ValidFrom             *time.Time `json:"validFrom"             orm:"valid_from"              description:""`
	ValidUntil            *time.Time `json:"validUntil"            orm:"valid_until"             description:""`
	StatusCode            string     `json:"statusCode"            orm:"status_code"             description:""`
	ClaimedAt             *time.Time `json:"claimedAt"             orm:"claimed_at"              description:""`
	CreatedAt             *time.Time `json:"createdAt"             orm:"created_at"              description:""`
	UpdatedAt             *time.Time `json:"updatedAt"             orm:"updated_at"              description:""`
	CouponBatchNo         string     `json:"couponBatchNo"         orm:"coupon_batch_no"         description:""`
}
