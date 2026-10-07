// This file defines explicit, idempotent import of an unpublished catalog draft.
// Monetary input is decimal text; supplier credentials and source metadata are
// never accepted from the browser.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogProductImportReq struct {
	g.Meta              `path:"/admin/catalog/product-imports" method:"post" tags:"IDC Catalog Management" summary:"Import upstream product draft" dc:"Read one explicitly selected upstream product and atomically create an unpublished draft; unchanged request-key retries return the original identity without another supplier read." permission:"idc:admin:catalog:product:create"`
	RequestKey          string               `json:"requestKey" v:"required|length:16,128" dc:"Unique submission key; reuse only for an unchanged retry" eg:"9efb8e0a-fdda-4eb7-949b-6d1381937d93"`
	InterfaceInstanceNo string               `json:"interfaceInstanceNo" v:"required|length:36,36" dc:"Selected enabled server interface business number" eg:"pfi_0123456789abcdef0123456789abcdef"`
	Reference           string               `json:"reference" v:"required|max-length:256" dc:"Opaque selected upstream product reference" eg:"101"`
	CategoryNo          string               `json:"categoryNo" v:"required|length:36,36" dc:"Selected active local product category" eg:"ctg_0123456789abcdef0123456789abcdef"`
	NameZhCn            string               `json:"nameZhCn" v:"max-length:256" dc:"Chinese product name; empty uses the original upstream name without translation" eg:"Cloud server"`
	NameEnUs            string               `json:"nameEnUs" v:"max-length:256" dc:"English product name; empty uses the original upstream name without translation" eg:"Cloud server"`
	PricingMode         string               `json:"pricingMode" v:"required|in:markup,manual" dc:"Pricing mode: markup increases proven complete upstream prices; manual uses explicit final new-purchase prices" eg:"markup"`
	MarkupPercent       string               `json:"markupPercent" v:"max-length:128" dc:"Nonnegative decimal uplift percentage; 10 means upstream cost plus ten percent, required only for markup" eg:"10"`
	Prices              []CatalogImportPrice `json:"prices" dc:"At most 120 explicit final new-purchase schedules, required only for manual pricing; never inferred renewal prices" eg:"[]"`
}

type CatalogImportPrice struct {
	CurrencyCode string `json:"currencyCode" dc:"Explicit ISO currency code using the local standard minor-unit precision" eg:"CNY"`
	Period       string `json:"period" dc:"Billing interval: once, hour, day, month or year" eg:"month"`
	PeriodCount  int    `json:"periodCount" dc:"Positive number of billing interval units; once requires one" eg:"1"`
	Amount       string `json:"amount" dc:"Nonnegative exact final amount in major currency units, rounded half up to the currency minor unit" eg:"110.00"`
	SetupFee     string `json:"setupFee" dc:"Explicit nonnegative final setup fee in major currency units; unknown is not zero" eg:"0.00"`
}

type CatalogImportMissingConfiguration struct {
	FieldKey string `json:"fieldKey" dc:"Declared unfilled field key, empty for an unmapped upstream option" eg:"cpu"`
	NameZhCn string `json:"nameZhCn" dc:"Chinese declaration label or original upstream option name" eg:"CPU cores"`
	NameEnUs string `json:"nameEnUs" dc:"English declaration label or original upstream option name" eg:"CPU cores"`
}

type CatalogProductImportRes struct {
	Item                 *CatalogProductCreateRes            `json:"item" dc:"Original unpublished draft creation result; import never publishes" eg:"{}"`
	MissingConfiguration []CatalogImportMissingConfiguration `json:"missingConfiguration" dc:"Initial missing or unmapped configuration requiring administrator review before publication" eg:"[]"`
	PricingIncomplete    bool                                `json:"pricingIncomplete" dc:"True when no complete final local price could be imported; base prices are not full machine quotes" eg:"true"`
}
