// This file replaces expiring import-time quote tokens only after matching the
// current upstream term to the interval frozen on the paid local order.
package peer

import (
	"net/http"
	"net/url"

	"lina-core/pkg/plugin/capability/confidentialoutboundcap"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
)

// PurchaseForSchedule selects one unambiguous current term without changing
// the paid order's quantity, currency, billing interval or purchase cycles.
func PurchaseForSchedule(executor confidentialoutboundcap.Service, contextID, key string, purchase channelcommercecap.Purchase, billing *productfulfillmentcap.BillingSchedule) (*productfulfillmentcap.Result, error) {
	if billing == nil || billing.Validate() != nil || purchase.CurrencyCode != billing.CurrencyCode || purchase.PurchaseCycles != billing.PurchaseCycles || purchase.Quantity < 1 || purchase.Quantity > 1000 {
		return nil, errInvalid
	}
	input := channelcommercecap.ConfigurationInput{PublicSlug: purchase.PublicSlug, ConfigurationRevision: purchase.ConfigurationRevision, CurrencyCode: purchase.CurrencyCode, Values: purchase.Values}
	if input.Validate() != nil {
		return nil, errInvalid
	}
	c := &client{executor: executor, contextID: contextID}
	var terms channelcommercecap.BillingTerms
	query, err := configurationQuery(input)
	if err != nil {
		return nil, err
	}
	if err := c.call(http.MethodGet, "/catalog/products/"+url.PathEscape(purchase.PublicSlug)+"/billing-terms", query, nil, billingFields, &terms); err != nil {
		return nil, err
	}
	if terms.Validate() != nil || terms.ConfigurationRevision != purchase.ConfigurationRevision || terms.CurrencyCode != billing.CurrencyCode {
		return nil, errUnavailable
	}
	selected := ""
	for _, term := range terms.Terms {
		if (term.Status != "final" && term.Status != "requires_customer_context") || term.ChargeType != string(billing.ChargeType) || term.PeriodUnit != string(billing.PeriodUnit) || term.PeriodCount != billing.PeriodCount || term.MinimumPurchaseCycles > billing.PurchaseCycles {
			continue
		}
		if selected != "" {
			return nil, errUnavailable
		}
		selected = term.BillingTermToken
	}
	if selected == "" {
		return nil, errUnavailable
	}
	purchase.BillingTermToken = selected
	return purchaseWithClient(c, key, purchase)
}
