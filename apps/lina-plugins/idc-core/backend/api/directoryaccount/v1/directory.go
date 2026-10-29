// This file defines current-customer merchant navigation and service reads.

package v1

import "github.com/gogf/gf/v2/frame/g"

type Directory struct {
	ParentDirectoryNo string `json:"parentDirectoryNo" dc:"Parent menu reference; empty means top level; at most two levels" eg:"pdr_services"`
	NavigationType    string `json:"navigationType" dc:"Member destination: product, system, custom or plugin; default product" eg:"product"`
	TargetURL         string `json:"targetUrl" dc:"Internal page path for system or plugin; HTTP or HTTPS URL for custom; empty for product" eg:"/account/overview"`
	Icon              string `json:"icon" dc:"Iconify icon identifier used in member navigation" eg:"lucide:folder"`

	DirectoryNo  string `json:"directoryNo" dc:"Opaque merchant directory reference" eg:"pdr_example"`
	NameZhCn     string `json:"nameZhCn" dc:"Chinese directory name" eg:"My cloud services"`
	NameEnUs     string `json:"nameEnUs" dc:"English directory name" eg:"My cloud services"`
	SortOrder    int    `json:"sortOrder" dc:"Navigation order from 0 to 1000000" eg:"10"`
	ServiceCount int    `json:"serviceCount" dc:"Purchased entitlements of the current customer in this directory" eg:"2"`
}
type ListReq struct {
	NavigationType string `json:"navigationType" v:"in:product,system,custom,plugin" dc:"Optional navigation type filter: product, system, custom or plugin; omitted includes all enabled types, with pagination and totals scoped to the same filter" eg:"product"`
	g.Meta         `path:"/account/product-directories" method:"get" tags:"IDC Product Directories" summary:"List current customer product directories" dc:"List enabled merchant navigation menus and customer-scoped service counts"`
	Page           int `json:"page" d:"1" v:"min:1|max:1000000" dc:"Page number; defaults to 1" eg:"1"`
	PageSize       int `json:"pageSize" d:"100" v:"min:1|max:100" dc:"Page size from 1 to 100; defaults to 100" eg:"100"`
}
type ListRes struct {
	Items    []Directory `json:"items" dc:"Merchant navigation menus on the current page" eg:"[]"`
	Total    int         `json:"total" dc:"Total enabled directories matching the requested type filter" eg:"1"`
	Page     int         `json:"page" dc:"Current page number" eg:"1"`
	PageSize int         `json:"pageSize" dc:"Current page size" eg:"100"`
}
type Entitlement struct {
	CustomerRemark           string  `json:"customerRemark" dc:"Current customer non-sensitive service note" eg:"Production website"`
	CustomerRemarkRevision   int64   `json:"customerRemarkRevision" dc:"Independent optimistic revision for the note" eg:"1"`
	CurrentPriceAmountMinor  *string `json:"currentPriceAmountMinor" dc:"Total amount in the current effective price record in minor units, null when unavailable; not a cash receipt or a renewal quote" eg:"1000"`
	CurrencyCode             string  `json:"currencyCode" dc:"Currency of the actual price snapshot" eg:"CNY"`
	PeriodUnitCode           string  `json:"periodUnitCode" dc:"Frozen charged period unit; empty when unavailable" eg:"month"`
	PeriodCount              int     `json:"periodCount" dc:"Frozen units per charged cycle; zero when unavailable" eg:"1"`
	ChargeCycles             int     `json:"chargeCycles" dc:"Actual number of charged cycles represented by the amount; zero when unavailable" eg:"1"`
	EntitlementOwnerID       string  `json:"entitlementOwnerId,omitempty" dc:"Frozen owner plugin for service management navigation" eg:"idc-resource"`
	ManagementPreparation    string  `json:"managementPreparation,omitempty" dc:"Optional local service management readiness: preparing, ready, or needs_attention; absent for historical services without a notification; does not change delivery, lifecycle or billing" eg:"preparing"`
	EntitlementNo            string  `json:"entitlementNo" dc:"Opaque purchased service reference" eg:"ent_example"`
	ProductNo                string  `json:"productNo" dc:"Opaque product reference" eg:"prd_example"`
	ProductNameZhCn          string  `json:"productNameZhCn" dc:"Chinese frozen purchased product name" eg:"Cloud server"`
	ProductNameEnUs          string  `json:"productNameEnUs" dc:"English frozen purchased product name" eg:"Cloud server"`
	PluginId                 string  `json:"pluginId" dc:"Frozen fulfillment plugin identifier" eg:"yunsu-ssl"`
	StatusCode               string  `json:"statusCode" dc:"Service status: pending_activation, active, suspended, expired, terminated or cancelled" eg:"active"`
	RequiresManualProcessing bool    `json:"requiresManualProcessing" dc:"True when this pending-activation service needs merchant processing; does not change service lifecycle or billing dates" eg:"false"`
	OrderNo                  string  `json:"orderNo" dc:"Current customer order reference for service details" eg:"ord_example"`
	StartedAt                int64   `json:"startedAt" dc:"Service start Unix timestamp in milliseconds" eg:"1791475200000"`
	ExpiresAt                *int64  `json:"expiresAt" dc:"Service expiry Unix timestamp in milliseconds; null for no expiry" eg:"1794153600000"`
}
type ServicesReq struct {
	ProviderID  string `json:"providerId" v:"max-length:128" dc:"Optional plugin owning the purchased-services content block; omitted includes all providers; filtering happens before pagination" eg:"idc-resource"`
	Keyword     string `json:"keyword" v:"max-length:100" dc:"Literal case-insensitive product name, service number or customer note search; omitted lists all matching services" eg:"Production"`
	g.Meta      `path:"/account/product-directories/{directoryNo}/services" method:"get" tags:"IDC Product Directories" summary:"List purchased services by product directory" dc:"Read only current customer entitlements; the reserved all directory includes unclassified and disabled-directory services"`
	DirectoryNo string `json:"directoryNo" in:"path" v:"required|max-length:40" dc:"Merchant directory reference or reserved all fallback" eg:"all"`
	Page        int    `json:"page" d:"1" v:"min:1|max:1000000" dc:"Page number; defaults to 1" eg:"1"`
	PageSize    int    `json:"pageSize" d:"20" v:"min:1|max:100" dc:"Page size from 1 to 100; defaults to 20" eg:"20"`
}
type ServiceProvider struct {
	PluginID string `json:"pluginId" dc:"Plugin owning one purchased-services content block" eg:"idc-resource"`
	Total    int    `json:"total" dc:"Paid customer-owned services matching this block and current filters" eg:"2"`
}
type ServicesRes struct {
	Providers []ServiceProvider `json:"providers" dc:"Plugin-owned blocks matching the same paid customer and directory boundary, independent of page rows" eg:"[]"`
	Items     []Entitlement     `json:"items" dc:"Customer purchased services on the current page" eg:"[]"`
	Total     int               `json:"total" dc:"Total matching purchased services" eg:"1"`
	Page      int               `json:"page" dc:"Current page number" eg:"1"`
	PageSize  int               `json:"pageSize" dc:"Current page size" eg:"20"`
}
