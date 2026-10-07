// This file loads safe projections and exact append-only action replays.
package taxinvoicemanagement

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
)

type requestRow struct {
	ID                   int64      `json:"id"`
	TaxInvoiceNo         string     `json:"tax_invoice_no"`
	CustomerNo           string     `json:"customer_no"`
	StatusCode           string     `json:"status_code"`
	InvoiceTypeCode      string     `json:"invoice_type_code"`
	ProviderCode         *string    `json:"provider_code"`
	ExternalInvoiceRef   *string    `json:"external_invoice_ref"`
	DecisionReasonCode   string     `json:"decision_reason_code"`
	DecisionInternalNote string     `json:"decision_internal_note"`
	GrossAmountMinor     int64      `json:"gross_amount_minor"`
	NetAmountMinor       int64      `json:"net_amount_minor"`
	TaxAmountMinor       int64      `json:"tax_amount_minor"`
	TaxCurrencyCode      string     `json:"tax_currency_code"`
	MaskedInvoiceNumber  string     `json:"masked_invoice_number"`
	Version              int64      `json:"version"`
	SubmittedAt          *time.Time `json:"submitted_at"`
	ReviewedAt           *time.Time `json:"reviewed_at"`
	IssuedAt             *time.Time `json:"issued_at"`
	DeliveredAt          *time.Time `json:"delivered_at"`
	CanceledAt           *time.Time `json:"canceled_at"`
	NotIssuedConfirmedAt *time.Time `json:"provider_not_issued_confirmed_at"`
}

type actionRow struct {
	RequestFingerprint string    `json:"request_fingerprint"`
	AfterStatusCode    string    `json:"after_status_code"`
	AfterVersion       int64     `json:"after_version"`
	InvoiceTypeCode    string    `json:"invoice_type_code"`
	ProviderCode       string    `json:"provider_code"`
	ReasonCode         string    `json:"reason_code"`
	InternalNote       string    `json:"internal_note"`
	CreatedAt          time.Time `json:"created_at"`
}

func (s *serviceImpl) load(ctx context.Context, taxInvoiceNo string, lock bool) (*requestRow, error) {
	query := `
SELECT id, tax_invoice_no, customer_no, status_code, invoice_type_code, provider_code,
       external_invoice_ref, decision_reason_code, decision_internal_note,
       gross_amount_minor, net_amount_minor, tax_amount_minor, tax_currency_code,
       masked_invoice_number, version, submitted_at, reviewed_at, issued_at, delivered_at, canceled_at,
       provider_not_issued_confirmed_at
FROM plugin_idc_compliance_tax_invoice
WHERE tenant_id = ? AND tax_invoice_no = ?`
	if lock {
		query += " FOR UPDATE"
	}
	var row *requestRow
	if err := s.database.GetScan(ctx, &row, query, idccap.DefaultTenantID, taxInvoiceNo); err != nil {
		return nil, gerror.Wrap(err, "load tax invoice management request")
	}
	return row, nil
}

func loadTX(ctx context.Context, tx gdb.TX, taxInvoiceNo string) (*requestRow, error) {
	var row *requestRow
	err := tx.Ctx(ctx).GetScan(&row, `
SELECT id, tax_invoice_no, customer_no, status_code, invoice_type_code, provider_code,
       external_invoice_ref, decision_reason_code, decision_internal_note,
       gross_amount_minor, net_amount_minor, tax_amount_minor, tax_currency_code,
       masked_invoice_number, version, submitted_at, reviewed_at, issued_at, delivered_at, canceled_at,
       provider_not_issued_confirmed_at
FROM plugin_idc_compliance_tax_invoice
WHERE tenant_id = ? AND tax_invoice_no = ?
FOR UPDATE`, idccap.DefaultTenantID, taxInvoiceNo)
	return row, err
}

func loadActionTX(
	ctx context.Context,
	tx gdb.TX,
	taxInvoiceNo, action, digest string,
) (*actionRow, error) {
	var row *actionRow
	err := tx.Ctx(ctx).GetScan(&row, `
SELECT request_fingerprint, after_status_code, after_version, invoice_type_code,
       provider_code, reason_code, internal_note, created_at
FROM plugin_idc_compliance_tax_invoice_action
WHERE tenant_id = ? AND tax_invoice_no = ? AND action_code = ? AND idempotency_digest = ?`,
		idccap.DefaultTenantID, taxInvoiceNo, action, digest)
	return row, err
}

func project(row *requestRow, capability Capability) (*Request, error) {
	if row == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	if !businessNoPattern.MatchString(row.TaxInvoiceNo) ||
		!businessNoPattern.MatchString(row.CustomerNo) || row.Version <= 0 ||
		row.GrossAmountMinor <= 0 || row.SubmittedAt == nil || row.SubmittedAt.IsZero() {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	providerCode := ""
	if row.ProviderCode != nil {
		providerCode = *row.ProviderCode
	}
	result := &Request{
		TaxInvoiceNo: row.TaxInvoiceNo, CustomerNo: row.CustomerNo,
		StatusCode: row.StatusCode, InvoiceTypeCode: row.InvoiceTypeCode,
		ProviderCode: providerCode, DecisionReasonCode: row.DecisionReasonCode,
		DecisionInternalNote: row.DecisionInternalNote,
		GrossAmountMinor:     row.GrossAmountMinor, NetAmountMinor: row.NetAmountMinor,
		TaxAmountMinor: row.TaxAmountMinor, CurrencyCode: row.TaxCurrencyCode,
		MaskedInvoiceNumber: row.MaskedInvoiceNumber,
		Version:             row.Version, SubmittedAt: row.SubmittedAt.UTC(), ReviewedAt: row.ReviewedAt,
		IssuedAt: row.IssuedAt, DeliveredAt: row.DeliveredAt, CanceledAt: row.CanceledAt,
		ProviderNotIssuedAt: row.NotIssuedConfirmedAt,
		CapabilityReady:     capability.Ready, CapabilityReasonCode: capability.ReasonCode,
		SupportedInvoiceTypes: append([]string(nil), capability.SupportedInvoiceTypes...),
	}
	if result.ReviewedAt != nil {
		value := result.ReviewedAt.UTC()
		result.ReviewedAt = &value
	}
	for _, value := range []**time.Time{&result.IssuedAt, &result.DeliveredAt, &result.CanceledAt, &result.ProviderNotIssuedAt} {
		if *value != nil {
			normalized := (*value).UTC()
			*value = &normalized
		}
	}
	return result, nil
}

func applyReplay(row *requestRow, action *actionRow) {
	if row == nil || action == nil {
		return
	}
	row.StatusCode = action.AfterStatusCode
	row.Version = action.AfterVersion
	row.InvoiceTypeCode = action.InvoiceTypeCode
	row.DecisionReasonCode = action.ReasonCode
	row.DecisionInternalNote = action.InternalNote
	row.ReviewedAt = &action.CreatedAt
	if action.ProviderCode == "" {
		row.ProviderCode = nil
	} else {
		value := action.ProviderCode
		row.ProviderCode = &value
	}
}
