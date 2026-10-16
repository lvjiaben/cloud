// This file defines bounded drill-down filters shared with the operating overview.

package v1

import "github.com/gogf/gf/v2/frame/g"

// OperationsDataRecordsGetReq reads one metric with the original business permission.
type OperationsDataRecordsGetReq struct {
	g.Meta       `path:"/admin/operations/data/records" method:"get" tags:"IDC Operations" summary:"List operating records" dc:"Read a bounded page matching an operating metric; the selected business domain permission is additionally required." permission:"idc:admin:operations:workbench:view"`
	Metric       string `json:"metric" v:"required|in:income,orders,customers,servers,tickets,products" dc:"Metric: income, orders, customers, servers, tickets or products" eg:"income"`
	StartAt      int64  `json:"startAt" v:"min:0" dc:"Inclusive filter start, Unix timestamp in milliseconds; zero means all history" eg:"0"`
	EndAt        int64  `json:"endAt" v:"min:0" dc:"Exclusive filter end, Unix timestamp in milliseconds; defaults to now" eg:"1791561600000"`
	AsOf         int64  `json:"asOf" v:"min:0" dc:"Overview read boundary, Unix timestamp in milliseconds; defaults to now" eg:"1791561600000"`
	CurrencyCode string `json:"currencyCode" dc:"Original currency required for income, orders and products" eg:"CNY"`
	Status       string `json:"status" v:"in:,today,online,pending,processed" dc:"Optional metric subset: today, online, pending or processed" eg:"online"`
	ProductNo    string `json:"productNo" v:"length:0,100" dc:"Exact product business number; omitted for all products" eg:"PRD_1"`
	Page         int    `json:"page" d:"1" v:"min:1|max:1000000" dc:"Page number starting at one" eg:"1"`
	PageSize     int    `json:"pageSize" d:"20" v:"min:1|max:100" dc:"Records per page, from one to one hundred" eg:"20"`
}

// OperationsDataRecordsGetRes contains an accurate total and the selected page.
type OperationsDataRecordsGetRes struct{ OperationsDataRecords }
