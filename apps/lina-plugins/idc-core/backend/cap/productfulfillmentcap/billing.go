// This file carries the immutable purchased interval to fulfillment connectors.
// It contains no retail amounts or mutable catalog prices. A connector requiring
// upstream procurement must reject a missing schedule rather than infer one.

package productfulfillmentcap

import (
	"regexp"

	"github.com/gogf/gf/v2/errors/gerror"
)

// BillingChargeType and BillingPeriodUnit use the catalog's public vocabulary.
type BillingChargeType string
type BillingPeriodUnit string

const (
	BillingChargeOneTime   BillingChargeType = "one_time"
	BillingChargeRecurring BillingChargeType = "recurring"
	BillingChargeTiered    BillingChargeType = "tiered"
	BillingChargeUsage     BillingChargeType = "usage"
	BillingChargeTrial     BillingChargeType = "trial"
	BillingPeriodOnce      BillingPeriodUnit = "once"
	BillingPeriodHour      BillingPeriodUnit = "hour"
	BillingPeriodDay       BillingPeriodUnit = "day"
	BillingPeriodMonth     BillingPeriodUnit = "month"
	BillingPeriodYear      BillingPeriodUnit = "year"
)

// BillingSchedule identifies the interval and currency frozen on an order item.
// Existing consumers may ignore this additive field; procurement consumers must
// validate it before choosing an upstream billing term.
type BillingSchedule struct {
	CurrencyCode   string            `json:"currencyCode"`
	ChargeType     BillingChargeType `json:"chargeType"`
	PeriodUnit     BillingPeriodUnit `json:"periodUnit"`
	PeriodCount    int               `json:"periodCount"`
	PurchaseCycles int               `json:"purchaseCycles"`
}

var billingCurrencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

// Validate checks the frozen schedule without consulting current product prices.
func (b *BillingSchedule) Validate() error {
	if b == nil || !billingCurrencyPattern.MatchString(b.CurrencyCode) || b.PurchaseCycles < 1 || b.PurchaseCycles > 120 {
		return gerror.New("fulfillment billing schedule is invalid")
	}
	if b.PeriodCount < 1 || b.PeriodCount > maximumBillingPeriodCount(b.PeriodUnit) {
		return gerror.New("fulfillment billing period is invalid")
	}
	return b.validateChargePeriod()
}

// maximumBillingPeriodCount mirrors the catalog's bounded interval vocabulary.
func maximumBillingPeriodCount(unit BillingPeriodUnit) int {
	switch unit {
	case BillingPeriodOnce:
		return 1
	case BillingPeriodHour:
		return 87600
	case BillingPeriodDay:
		return 3650
	case BillingPeriodMonth:
		return 120
	case BillingPeriodYear:
		return 10
	default:
		return 0
	}
}

// validateChargePeriod rejects intervals incompatible with the charge type.
func (b *BillingSchedule) validateChargePeriod() error {
	switch b.ChargeType {
	case BillingChargeOneTime:
		if b.PeriodUnit != BillingPeriodOnce || b.PurchaseCycles != 1 {
			return gerror.New("one-time fulfillment billing period is invalid")
		}
	case BillingChargeRecurring, BillingChargeTiered, BillingChargeUsage, BillingChargeTrial:
		if b.PeriodUnit == BillingPeriodOnce {
			return gerror.New("recurring fulfillment billing period is invalid")
		}
		if b.ChargeType == BillingChargeTrial && b.PurchaseCycles != 1 {
			return gerror.New("trial fulfillment must have one purchase cycle")
		}
	default:
		return gerror.New("fulfillment charge type is invalid")
	}
	return nil
}
