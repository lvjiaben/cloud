// This file defines read-only progress lookup for asynchronously executed operations.
package v1

import (
	"github.com/gogf/gf/v2/frame/g"
)

type RenewalByRequestReq struct {
	g.Meta    `path:"/channel/services/{serviceNo}/renewals/by-request/{requestId}" method:"get" tags:"Open API" summary:"Find a renewal by its original request identifier" dc:"Read the original renewal request without retrying payment. Scoped to the authenticated application and exact service; requires service.read."`
	ServiceNo string `json:"serviceNo" v:"required|max-length:40" dc:"Service identifier" eg:"ENT_10001"`
	RequestId string `json:"requestId" v:"required|length:8,120" dc:"Original renewal request identifier" eg:"peer-renewal-example"`
}
type RenewalByRequestRes struct {
	ObservedAt       int64  `json:"observedAt" dc:"Authenticated server observation time as Unix milliseconds" eg:"1791676800000"`
	Found            bool   `json:"found" dc:"Whether this application owns the original request" eg:"true"`
	OperationNo      string `json:"operationNo" dc:"Upstream renewal operation identifier; empty when not found" eg:"OPR_10001"`
	Status           string `json:"status" dc:"Renewal status: quoted, awaiting_financial_settlement, ready, executing, succeeded, failed or cancelled" eg:"succeeded"`
	CurrencyCode     string `json:"currencyCode" dc:"Frozen ISO renewal currency" eg:"CNY"`
	AmountMinor      int64  `json:"amountMinor" dc:"Renewal invoice amount in minor currency units" eg:"10000"`
	CurrentExpiresAt *int64 `json:"currentExpiresAt" dc:"Current service expiry as Unix timestamp in milliseconds; null when not available" eg:"1794355200000"`
}

type GetLifecycleReq struct {
	g.Meta      `path:"/channel/services/{serviceNo}/operations/{operationNo}" method:"get" tags:"Open API" summary:"Get service operation progress" dc:"Read the persisted progress of a service operation belonging to the authenticated application; does not execute or retry the operation."`
	ServiceNo   string `json:"serviceNo" v:"required|max-length:40" dc:"Service identifier from the application service list" eg:"ENT_10001"`
	OperationNo string `json:"operationNo" v:"required|max-length:40" dc:"Operation identifier returned when submitting the operation" eg:"OPR_10001"`
}
type GetLifecycleRes struct {
	OperationNo  string `json:"operationNo" dc:"Requested operation identifier" eg:"OPR_10001"`
	Status       string `json:"status" dc:"Operation status: quoted, awaiting_financial_settlement, financially_settled, ready, executing, succeeded, failed, compensating, refunding, manual_review or cancelled" eg:"succeeded"`
	CurrencyCode string `json:"currencyCode" dc:"Operation currency" eg:"CNY"`
	AmountMinor  int64  `json:"amountMinor" dc:"Invoice amount in minor currency units; zero for an unpriced action" eg:"10000"`
}
