package v1

import "github.com/gogf/gf/v2/frame/g"

type ServerAccessoryPricePolicy struct {
	PolicyNo           string `json:"policyNo" dc:"Stable accessory price policy number"`
	Kind               string `json:"kind" dc:"Snapshot, backup, or traffic"`
	CurrencyCode       string `json:"currencyCode" dc:"Explicit price currency"`
	FreeCountPerServer int    `json:"freeCountPerServer" dc:"Free objects allowed on each server"`
	UnitAmountMinor    int64  `json:"unitAmountMinor" dc:"Price per object or per GB in minor units"`
	PeriodUnitCode     string `json:"periodUnitCode" dc:"Day, month, year, or once"`
	PeriodCount        int    `json:"periodCount" dc:"Number of period units"`
	Version            int64  `json:"version" dc:"Current published policy version"`
	PublishedAt        int64  `json:"publishedAt" dc:"Publish time as Unix milliseconds"`
}

type ServerAccessoryPriceListReq struct {
	g.Meta `path:"/admin/resource/accessory-prices" method:"get" tags:"IDC Server Operations" summary:"List server accessory prices" permission:"idc:admin:resource:accessory-price:read"`
}

type ServerAccessoryPriceListRes struct {
	Items []*ServerAccessoryPricePolicy `json:"items"`
}

type ServerAccessoryPricePublishReq struct {
	g.Meta             `path:"/admin/resource/accessory-prices/{kind}/{currencyCode}" method:"put" tags:"IDC Server Operations" summary:"Publish a server accessory price" permission:"idc:admin:resource:accessory-price:write"`
	Kind               string `json:"kind" in:"path" v:"required|in:snapshot,backup,traffic"`
	CurrencyCode       string `json:"currencyCode" in:"path" v:"required|regex:^[A-Z]{3}$"`
	FreeCountPerServer int    `json:"freeCountPerServer" v:"min:0|max:10000"`
	UnitAmountMinor    int64  `json:"unitAmountMinor" v:"min:0|max:1000000000"`
	PeriodUnitCode     string `json:"periodUnitCode" v:"required|in:day,month,year,once"`
	PeriodCount        int    `json:"periodCount" v:"required|min:1|max:3650"`
	ExpectedVersion    int64  `json:"expectedVersion" v:"min:0"`
}

type ServerAccessoryPricePublishRes struct {
	Policy *ServerAccessoryPricePolicy `json:"policy"`
}
