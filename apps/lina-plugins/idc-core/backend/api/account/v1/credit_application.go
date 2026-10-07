// This file defines customer credit application list and submission contracts.

package v1

import "github.com/gogf/gf/v2/frame/g"

// CreditApplicationListReq lists the current customer's bounded credit applications.
type CreditApplicationListReq struct {
	Page         int `json:"page" v:"min:0" dc:"Optional one-based page number; omit or zero for cursor paging; cannot combine with cursor" eg:"1"`
	g.Meta       `path:"/account/billing/credit/applications" method:"get" tags:"IDC Customer Credit" summary:"List my credit applications" dc:"Return one bounded page and exact filtered total owned by the current active customer."`
	StatusCode   string `json:"statusCode" dc:"Optional exact application status: pending_review, approved, rejected, cancelled, or expired" eg:"pending_review"`
	CurrencyCode string `json:"currencyCode" dc:"Optional exact configured currency" eg:"CNY"`
	Cursor       string `json:"cursor" v:"max-length:128" dc:"Encrypted cursor from the previous page" eg:"AQIDBA"`
	Limit        int    `json:"limit" d:"20" v:"min:1|max:100" dc:"Maximum applications returned in this page" eg:"20"`
}

// CreditApplicationListRes returns one bounded application page.
type CreditApplicationListRes struct {
	Total        int                  `json:"total" dc:"Exact authorized record count matching the current filters before pagination" eg:"123"`
	Applications []*CreditApplication `json:"applications" dc:"Bounded current-customer credit applications" eg:"[]"`
	NextCursor   string               `json:"nextCursor" dc:"Encrypted next-page cursor, or empty" eg:""`
}

// CreditApplicationSubmitReq submits one customer-owned credit application.
type CreditApplicationSubmitReq struct {
	g.Meta              `path:"/account/billing/credit/applications" method:"post" tags:"IDC Customer Credit" summary:"Submit my credit application" dc:"Create one current-customer application without accepting a customer number or staff authority from the request."`
	CurrencyCode        string   `json:"currencyCode" v:"required|regex:^[A-Z]{3}$" dc:"Exact requested configured currency" eg:"CNY"`
	RequestedLimitMinor string   `json:"requestedLimitMinor" v:"required|regex:^[1-9][0-9]{0,15}$" dc:"Exact requested limit in decimal minor units" eg:"100000"`
	RequestedValidFrom  int64    `json:"requestedValidFrom" v:"required|min:1" dc:"Requested validity start as Unix timestamp in milliseconds" eg:"1720000000000"`
	RequestedValidUntil int64    `json:"requestedValidUntil" v:"required|min:1" dc:"Requested validity end as Unix timestamp in milliseconds" eg:"1751536000000"`
	UploadNos           []string `json:"uploadNos" v:"required" dc:"One to ten private evidence uploads owned by the current customer; collection bounds are enforced before persistence" eg:"[\"CEU_01J2ABCDEF\"]"`
	IdempotencyKey      string   `json:"idempotencyKey" v:"required|min-length:8|max-length:128" dc:"Random request key stable across retries; the raw value is not persisted by the credit owner" eg:"credit-application-01J2"`
}

type CreditEvidenceUploadCreateReq struct {
	g.Meta    `path:"/account/billing/credit/evidence-uploads" method:"post" tags:"IDC Customer Credit" summary:"Reserve my private credit evidence upload"`
	FileName  string `json:"fileName" v:"required|max-length:255" dc:"Original PDF, JPG, JPEG, or PNG file name" eg:"bank-statement.pdf"`
	SizeBytes int64  `json:"sizeBytes" v:"required|min:1|max:20971520" dc:"Declared size from one byte through 20 MiB" eg:"2048"`
}
type CreditEvidenceUploadCreateRes struct {
	UploadNo  string `json:"uploadNo"`
	ExpiresAt int64  `json:"expiresAt"`
}

type CreditEvidenceUploadContentUpdateReq struct {
	g.Meta   `path:"/account/billing/credit/evidence-uploads/{uploadNo}/content" method:"put" mime:"application/octet-stream" tags:"IDC Customer Credit" summary:"Upload my reserved private credit evidence"`
	UploadNo string `json:"uploadNo" in:"path" v:"required|max-length:40"`
}
type CreditEvidenceUploadContentUpdateRes struct {
	UploadNo  string `json:"uploadNo"`
	FileName  string `json:"fileName"`
	MediaType string `json:"mediaType"`
	SizeBytes int64  `json:"sizeBytes"`
}

// CreditApplicationSubmitRes returns the committed customer-owned application.
type CreditApplicationSubmitRes struct {
	Application *CreditApplication `json:"application" dc:"Committed current-customer credit application" eg:"{}"`
}
