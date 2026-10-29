// This file preserves native hour/day multipliers. Unspecified durations and
// account-adjusted tariffs cannot become authoritative one-unit prices.
package finance

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"

	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
)

func catalogBillingPeriods(prices []resourceinterfaceprojectioncap.CatalogPrice, hour, day json.RawMessage) ([]resourceinterfaceprojectioncap.CatalogPrice, bool) {
	result := make([]resourceinterfaceprojectioncap.CatalogPrice, 0, len(prices))
	complete := true
	for _, price := range prices {
		var raw json.RawMessage
		switch price.Period {
		case resourceinterfaceprojectioncap.CatalogPeriodHour:
			raw = hour
		case resourceinterfaceprojectioncap.CatalogPeriodDay:
			raw = day
		default:
			result = append(result, price)
			continue
		}
		value := scalar(raw)
		count, err := strconv.Atoi(value)
		schedule := productfulfillmentcap.BillingSchedule{CurrencyCode: price.CurrencyCode, ChargeType: productfulfillmentcap.BillingChargeRecurring, PeriodUnit: productfulfillmentcap.BillingPeriodUnit(price.Period), PeriodCount: count, PurchaseCycles: 1}
		if err != nil || schedule.Validate() != nil || strconv.Itoa(count) != value {
			complete = false
			continue
		}
		price.PeriodCount = count
		result = append(result, price)
	}
	return result, complete
}

type catalogBillingMetadata struct {
	Type  catalogPaymentType `json:"pay_type"`
	Trial json.RawMessage    `json:"pay_ontrial_status"`
	Hour  json.RawMessage    `json:"pay_hour_cycle"`
	Day   json.RawMessage    `json:"pay_day_cycle"`
}

func catalogProductPrices(product catalogWireProduct, flag json.RawMessage, rows []map[string]json.RawMessage) (catalogBillingMetadata, []resourceinterfaceprojectioncap.CatalogPrice, bool, error) {
	var payment catalogBillingMetadata
	if product.PayType != "" && json.Unmarshal([]byte(product.PayType), &payment) != nil {
		return payment, nil, false, errInvalidResponse
	}
	prices, complete, err := projectCatalogPrices(rows, payment.Type)
	if err != nil {
		return payment, nil, false, err
	}
	prices, periodsComplete := catalogBillingPeriods(prices, payment.Hour, payment.Day)
	complete = complete && periodsComplete && bytes.Equal(bytes.TrimSpace(flag), []byte("false"))
	if product.APIType == "resource" || (product.APIType == "zjmf_api" && product.UpstreamPriceType == "percent") {
		complete = false
	}
	trial := strings.TrimSpace(string(payment.Trial))
	if trial != "" && trial != "null" && trial != "false" && trial != "0" && trial != `"0"` && trial != `""` {
		complete = false
	}
	return payment, prices, complete, nil
}
