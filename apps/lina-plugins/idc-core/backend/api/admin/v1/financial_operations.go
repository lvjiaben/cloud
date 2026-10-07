package v1

import (
	"encoding/json"

	"github.com/gogf/gf/v2/frame/g"
)

type FinancialInvoiceDeliveryPreview struct {
	InvoiceNo      string `json:"invoiceNo" dc:"Stable billing invoice number"`
	MaskedEmail    string `json:"maskedEmail" dc:"Masked verified billing email"`
	InvoiceVersion int64  `json:"invoiceVersion" dc:"Current billing invoice version"`
}

type FinancialInvoiceDeliveryPreviewReq struct {
	g.Meta    `path:"/admin/billing/invoices/{invoiceNo}/delivery-preview" method:"get" tags:"IDC Financial Management" summary:"Preview billing invoice resend" dc:"Return only a masked verified billing email before confirmation." permission:"idc:admin:billing:invoice:resend"`
	InvoiceNo string `json:"invoiceNo" in:"path" v:"required|max-length:40" dc:"Stable billing invoice number"`
}
type FinancialInvoiceDeliveryPreviewRes struct {
	Preview *FinancialInvoiceDeliveryPreview `json:"preview" dc:"Bounded invoice resend preview"`
}

type FinancialInvoiceResendReq struct {
	g.Meta    `path:"/admin/billing/invoices/{invoiceNo}/resends" method:"post" tags:"IDC Financial Management" summary:"Resend a billing invoice" dc:"Submit one notification intent to the currently verified billing email; no temporary address is accepted." permission:"idc:admin:billing:invoice:resend"`
	InvoiceNo string `json:"invoiceNo" in:"path" v:"required|max-length:40" dc:"Stable billing invoice number"`
	RequestNo string `json:"requestNo" v:"required|min-length:16|max-length:128" dc:"Retry-stable random request key"`
}
type FinancialInvoiceResendRes struct {
	IntentNo string `json:"intentNo" dc:"Stable resend intent number"`
	Reused   bool   `json:"reused" dc:"Whether the same intent was replayed"`
}

type FinancialInternalNote struct {
	NoteNo         string `json:"noteNo" dc:"Stable internal note number"`
	TargetType     string `json:"targetType" dc:"Order, invoice, or service target type"`
	TargetNo       string `json:"targetNo" dc:"Stable target number"`
	Content        string `json:"content" dc:"Staff-only note content"`
	CorrectsNoteNo string `json:"correctsNoteNo" dc:"Earlier note corrected by this entry, or empty"`
	CreatedBy      int    `json:"createdBy" dc:"Direct employee identifier"`
	CreatedAt      int64  `json:"createdAt" dc:"Append time in Unix milliseconds"`
}
type FinancialInternalNoteListReq struct {
	g.Meta     `path:"/admin/billing/internal-notes" method:"get" tags:"IDC Financial Management" summary:"List internal financial notes" dc:"Return staff-only append-only notes for one exact order, invoice, or service." permission:"idc:admin:billing:note:list"`
	TargetType string `json:"targetType" v:"required|in:order,invoice,service" dc:"Business target type"`
	TargetNo   string `json:"targetNo" v:"required|max-length:40" dc:"Stable target number"`
	Limit      int    `json:"limit" d:"50" v:"min:1|max:100" dc:"Maximum notes returned"`
}
type FinancialInternalNoteListRes struct {
	Notes []*FinancialInternalNote `json:"notes" dc:"Staff-only append-only notes"`
}
type FinancialInternalNoteCreateReq struct {
	g.Meta         `path:"/admin/billing/internal-notes" method:"post" tags:"IDC Financial Management" summary:"Append an internal financial note" dc:"Append a staff-only note or correction without changing or deleting history." permission:"idc:admin:billing:note:create"`
	TargetType     string `json:"targetType" v:"required|in:order,invoice,service" dc:"Business target type"`
	TargetNo       string `json:"targetNo" v:"required|max-length:40" dc:"Stable target number"`
	Content        string `json:"content" v:"required|max-length:2000" dc:"Internal note content"`
	CorrectsNoteNo string `json:"correctsNoteNo" v:"max-length:40" dc:"Optional earlier note being corrected"`
	RequestNo      string `json:"requestNo" v:"required|min-length:16|max-length:128" dc:"Retry-stable random request key"`
}
type FinancialInternalNoteCreateRes struct {
	Note *FinancialInternalNote `json:"note" dc:"New append-only internal note"`
}

type FinancialSavedFilter struct {
	FilterNo    string          `json:"filterNo" dc:"Stable personal filter number"`
	ViewCode    string          `json:"viewCode" dc:"Financial page scope"`
	DisplayName string          `json:"displayName" dc:"Personal filter name"`
	Filter      json.RawMessage `json:"filter" dc:"Normalized filter object"`
	Version     int64           `json:"version" dc:"Personal filter version"`
	UpdatedAt   int64           `json:"updatedAt" dc:"Last save time in Unix milliseconds"`
}
type FinancialSavedFilterListReq struct {
	g.Meta   `path:"/admin/billing/saved-filters" method:"get" tags:"IDC Financial Management" summary:"List my saved financial filters" dc:"Return only the current employee's saved filters; each later query still applies current data permissions." permission:"idc:admin:billing:filter:personal"`
	ViewCode string `json:"viewCode" v:"required|in:invoice,transaction,adjustment,receivable" dc:"Financial page scope"`
}
type FinancialSavedFilterListRes struct {
	Filters []*FinancialSavedFilter `json:"filters" dc:"Current employee's saved filters"`
}
type FinancialSavedFilterSaveReq struct {
	g.Meta      `path:"/admin/billing/saved-filters" method:"post" tags:"IDC Financial Management" summary:"Save my financial filter" dc:"Create or replace one same-name personal filter without sharing or granting access." permission:"idc:admin:billing:filter:personal"`
	ViewCode    string          `json:"viewCode" v:"required|in:invoice,transaction,adjustment,receivable" dc:"Financial page scope"`
	DisplayName string          `json:"displayName" v:"required|max-length:80" dc:"Personal filter name"`
	Filter      json.RawMessage `json:"filter" v:"required" dc:"Bounded normalized filter object"`
}
type FinancialSavedFilterSaveRes struct {
	Filter *FinancialSavedFilter `json:"filter" dc:"Saved personal filter"`
}

type FinancialExportStatus struct {
	RequestNo   string `json:"requestNo" dc:"Stable financial export request number"`
	ExportType  string `json:"exportType" dc:"Invoice, transaction, adjustment, or receivable export type"`
	Status      string `json:"status" dc:"Current asynchronous export status"`
	FailureCode string `json:"failureCode" dc:"Stable failure code without internal details"`
	RequestedAt int64  `json:"requestedAt" dc:"Request time in Unix milliseconds"`
	GeneratedAt int64  `json:"generatedAt" dc:"Generation time in Unix milliseconds, or zero"`
	ExpiresAt   int64  `json:"expiresAt" dc:"Private XLSX expiry in Unix milliseconds, or zero"`
	RowCount    int    `json:"rowCount" dc:"Generated data row count"`
	Reused      bool   `json:"reused" dc:"Whether an identical request was replayed"`
}
type FinancialExportCreateReq struct {
	g.Meta     `path:"/admin/billing/exports" method:"post" tags:"IDC Financial Management" summary:"Create a financial XLSX export" dc:"Queue one private XLSX-only export after current exact permission and data-scope checks." permission:"idc:admin:billing:export"`
	ExportType string          `json:"exportType" v:"required|in:invoice,transaction,adjustment,receivable" dc:"Financial export type"`
	Filter     json.RawMessage `json:"filter" v:"required" dc:"Bounded export filter object"`
	RequestNo  string          `json:"requestNo" v:"required|min-length:16|max-length:128" dc:"Retry-stable random request key"`
}
type FinancialExportCreateRes struct {
	Export *FinancialExportStatus `json:"export" dc:"Queued asynchronous XLSX export"`
}
type FinancialExportGenerateReq struct {
	g.Meta    `path:"/admin/billing/exports/{requestNo}/generation" method:"post" tags:"IDC Financial Management" summary:"Generate a financial XLSX export" dc:"Recheck current permission and data scope before generating the queued private XLSX." permission:"idc:admin:billing:export"`
	RequestNo string `json:"requestNo" in:"path" v:"required|max-length:40" dc:"Stable financial export request number"`
}
type FinancialExportGenerateRes struct {
	Export *FinancialExportStatus `json:"export" dc:"Generated asynchronous XLSX export"`
}
type FinancialExportStatusReq struct {
	g.Meta    `path:"/admin/billing/exports/{requestNo}" method:"get" tags:"IDC Financial Management" summary:"Get my financial export status" dc:"Return one current-employee export only after fresh permission and scope checks." permission:"idc:admin:billing:export"`
	RequestNo string `json:"requestNo" in:"path" v:"required|max-length:40" dc:"Stable financial export request number"`
}
type FinancialExportStatusRes struct {
	Export *FinancialExportStatus `json:"export" dc:"Authorized financial export status"`
}
type FinancialExportTicketReq struct {
	g.Meta    `path:"/admin/billing/exports/{requestNo}/tickets" method:"post" tags:"IDC Financial Management" summary:"Issue a financial export download ticket" dc:"Recheck permission and scope before issuing a short single-use ticket for a private XLSX that expires after 24 hours." permission:"idc:admin:billing:export"`
	RequestNo string `json:"requestNo" in:"path" v:"required|max-length:40" dc:"Stable financial export request number"`
}
type FinancialExportTicketRes struct {
	Token     string `json:"token" dc:"Single-use private download token"`
	ExpiresAt int64  `json:"expiresAt" dc:"Ticket expiry in Unix milliseconds"`
}
type FinancialExportOpenReq struct {
	g.Meta `path:"/admin/billing/export-downloads/{token}" method:"get" tags:"IDC Financial Management" summary:"Download a financial XLSX" dc:"Recheck current permission and data scope, consume one ticket, and stream the verified private XLSX." permission:"idc:admin:billing:export"`
	Token  string `json:"token" in:"path" v:"required|min-length:32|max-length:128" dc:"Single-use private download token"`
}
type FinancialExportOpenRes struct{}
