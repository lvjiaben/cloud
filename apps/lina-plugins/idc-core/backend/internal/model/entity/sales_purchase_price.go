// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesPurchasePrice is the golang structure for table sales_purchase_price.
type SalesPurchasePrice struct {
	Id                  int64      `json:"id"                  orm:"id"                    description:""`
	TenantId            int        `json:"tenantId"            orm:"tenant_id"             description:""`
	PriceNo             string     `json:"priceNo"             orm:"price_no"              description:""`
	CustomerNo          string     `json:"customerNo"          orm:"customer_no"           description:""`
	LevelNo             string     `json:"levelNo"             orm:"level_no"              description:""`
	ProductNo           string     `json:"productNo"           orm:"product_no"            description:""`
	CurrencyCode        string     `json:"currencyCode"        orm:"currency_code"         description:""`
	ChargeTypeCode      string     `json:"chargeTypeCode"      orm:"charge_type_code"      description:""`
	PeriodUnitCode      string     `json:"periodUnitCode"      orm:"period_unit_code"      description:""`
	PeriodCount         int        `json:"periodCount"         orm:"period_count"          description:""`
	AdjustmentTypeCode  string     `json:"adjustmentTypeCode"  orm:"adjustment_type_code"  description:""`
	FixedPriceMinor     int64      `json:"fixedPriceMinor"     orm:"fixed_price_minor"     description:""`
	DiscountBasisPoints int        `json:"discountBasisPoints" orm:"discount_basis_points" description:""`
	StatusCode          string     `json:"statusCode"          orm:"status_code"           description:""`
	Version             int64      `json:"version"             orm:"version"               description:""`
	ContentDigest       string     `json:"contentDigest"       orm:"content_digest"        description:""`
	CreatedAt           *time.Time `json:"createdAt"           orm:"created_at"            description:""`
}
