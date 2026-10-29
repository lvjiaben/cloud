// This file exposes persisted commercial records without supplier refreshes.
package v1

import "github.com/gogf/gf/v2/frame/g"

type NativeServerCommercialOperationsReq struct {
	g.Meta      `path:"/account/native-servers/{serverNo}/commercial-operations" method:"get" tags:"IDC Account Servers" summary:"Read my server commercial operations" dc:"Read a bounded descending page of current-owner commercial history, including uncertain results. Paused and expired parents remain readable; no supplier is called."`
	ServerNo    string `json:"serverNo" in:"path" v:"required|regex:^SVR-[0-9A-F]{32}$" dc:"Owned public server number" eg:"SVR-0123456789ABCDEF0123456789ABCDEF"`
	OperationNo string `json:"operationNo" v:"max-length:40" dc:"Optional exact commercial operation filter; omit for all owned operations" eg:""`
	Cursor      string `json:"cursor" v:"max-length:256" dc:"Opaque next-page cursor bound to this owner, server and filters; omit for the first page" eg:""`
	Limit       int    `json:"limit" d:"20" v:"between:1,100" dc:"Page size from one to one hundred; default twenty" eg:"20"`
}
type NativeServerCommercialOperationsRes struct {
	Items      []NativeServerCommercialOperation `json:"items" dc:"Current-owner persisted commercial operations in descending order" eg:"[]"`
	NextCursor string                            `json:"nextCursor" dc:"Opaque next-page cursor; empty when no more records are available" eg:""`
}
type NativeServerCommercialOperation struct {
	OperationNo      string `json:"operationNo" dc:"Core commercial operation number" eg:"BOP_example"`
	QuoteNo          string `json:"quoteNo" dc:"Frozen commercial quote number" eg:"nqt_example"`
	OrderNo          string `json:"orderNo" dc:"Visible current-owner order number; empty when no document is visible" eg:""`
	InvoiceNo        string `json:"invoiceNo" dc:"Visible current-owner invoice number; empty when no document is visible" eg:""`
	CurrencyCode     string `json:"currencyCode" dc:"Operation currency code" eg:"CNY"`
	TotalAmountMinor string `json:"totalAmountMinor" dc:"Frozen final inclusive amount as a decimal string in minor currency units" eg:"9007199254740993"`
	Action           string `json:"action" dc:"Commercial action: snapshot_create, backup_create, traffic_add, ip_change or resize" eg:"resize"`
	Status           string `json:"status" dc:"Recorded commercial state: awaiting_financial_settlement, financially_settled, ready, executing, succeeded, failed, canceled or manual_review" eg:"manual_review"`
	LastErrorCode    string `json:"lastErrorCode" dc:"Owner-whitelisted handling reason, never supplier text; empty when unavailable" eg:""`
	CreatedAt        int64  `json:"createdAt" dc:"Creation time as Unix timestamp in milliseconds" eg:"1792382400000"`
	UpdatedAt        int64  `json:"updatedAt" dc:"Latest update as Unix timestamp in milliseconds" eg:"1792382400000"`
	CompletedAt      *int64 `json:"completedAt" dc:"Completion time as Unix timestamp in milliseconds; null when unfinished" eg:"1792382400000"`
}

type NativeServerAccessoryBillingReq struct {
	g.Meta   `path:"/account/native-servers/{serverNo}/accessory-billing" method:"get" tags:"IDC Account Servers" summary:"Read my server accessory billing" dc:"Read only billing records bound to verified snapshot or backup object numbers. Free objects have no invented period dates; pending invoices belong to the current owner. No supplier is called."`
	ServerNo string `json:"serverNo" in:"path" v:"required|regex:^SVR-[0-9A-F]{32}$" dc:"Owned public server number" eg:"SVR-0123456789ABCDEF0123456789ABCDEF"`
	ObjectNo string `json:"objectNo" v:"max-length:128" dc:"Optional exact local object filter; omit for all bound objects" eg:""`
	Kind     string `json:"kind" v:"in:snapshot,backup" dc:"Object kind: snapshot or backup; omit for both" eg:"snapshot"`
	Cursor   string `json:"cursor" v:"max-length:256" dc:"Opaque next-page cursor bound to this owner, server and filters; omit for the first page" eg:""`
	Limit    int    `json:"limit" d:"20" v:"between:1,100" dc:"Page size from one to one hundred; default twenty" eg:"20"`
}
type NativeServerAccessoryBillingRes struct {
	Items      []NativeServerAccessoryBill `json:"items" dc:"Only verified bound object billing records in descending order" eg:"[]"`
	NextCursor string                      `json:"nextCursor" dc:"Opaque next-page cursor; empty when no more records are available" eg:""`
}
type NativeServerAccessoryBill struct {
	AccessoryEntitlementNo string  `json:"accessoryEntitlementNo" dc:"Core accessory billing entitlement number, distinct from the object number" eg:"AEN_example"`
	ObjectNo               string  `json:"objectNo" dc:"Verified local object number used to match the resource directory" eg:"ACO_example"`
	Kind                   string  `json:"kind" dc:"Object kind: snapshot or backup" eg:"snapshot"`
	BillingMode            string  `json:"billingMode" dc:"Billing mode: free or paid" eg:"paid"`
	Status                 string  `json:"status" dc:"Billing lifecycle state: pending_binding, active, payment_pending, manual_review or deleted; independent of supplier runtime state" eg:"active"`
	CurrencyCode           string  `json:"currencyCode" dc:"Accessory billing currency code" eg:"CNY"`
	PeriodUnit             string  `json:"periodUnit" dc:"Billing period unit: day, month, year or empty for a free object" eg:"month"`
	PeriodCount            int     `json:"periodCount" dc:"Billing period unit count; zero when no paid period applies" eg:"1"`
	CurrentPeriodStartAt   *int64  `json:"currentPeriodStartAt" dc:"Current paid period start as Unix timestamp in milliseconds; null for a free object" eg:"1792382400000"`
	CurrentPeriodEndAt     *int64  `json:"currentPeriodEndAt" dc:"Current paid period end as Unix timestamp in milliseconds; null for a free object" eg:"1795060800000"`
	AutoRenewEnabled       bool    `json:"autoRenewEnabled" dc:"The owner currently schedules renewal for this bound object" eg:"true"`
	PendingInvoiceNo       string  `json:"pendingInvoiceNo" dc:"Visible current-owner pending renewal invoice; empty when none is visible" eg:""`
	PendingInvoiceStatus   string  `json:"pendingInvoiceStatus" dc:"Actual pending invoice state when visible, otherwise empty" eg:""`
	PendingAmountMinor     *string `json:"pendingAmountMinor" dc:"Visible frozen pending invoice amount as a decimal minor-unit string; null means no visible invoice, not a zero price" eg:"9007199254740993"`
	CreatedAt              int64   `json:"createdAt" dc:"Creation time as Unix timestamp in milliseconds" eg:"1792382400000"`
	UpdatedAt              int64   `json:"updatedAt" dc:"Latest update as Unix timestamp in milliseconds" eg:"1792382400000"`
}
