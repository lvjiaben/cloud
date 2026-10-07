// This file provides bounded persistence projections for red-letter tasks and history.

package taxinvoiceredletter

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/taxinvoicecap"
)

type taskRow struct {
	ID                           int64      `json:"id"`
	TaskNo                       string     `json:"task_no"`
	TaxInvoiceNo                 *string    `json:"tax_invoice_no"`
	RefundOperationNo            string     `json:"refund_operation_no"`
	CustomerNo                   *string    `json:"customer_no"`
	CurrencyCode                 *string    `json:"currency_code"`
	StatusCode                   string     `json:"status_code"`
	ManualReasonCode             string     `json:"manual_reason_code"`
	AllocationResolved           bool       `json:"allocation_resolved"`
	CustomerConfirmationRequired bool       `json:"customer_confirmation_required"`
	ProviderCode                 *string    `json:"provider_code"`
	ExternalTaskRef              *string    `json:"external_task_ref"`
	GrossAmountMinor             int64      `json:"gross_amount_minor"`
	NetAmountMinor               int64      `json:"net_amount_minor"`
	TaxAmountMinor               int64      `json:"tax_amount_minor"`
	Version                      int64      `json:"version"`
	CreatedAt                    time.Time  `json:"created_at"`
	CompletedAt                  *time.Time `json:"completed_at"`
	CompletionEvidenceNo         *string    `json:"completion_evidence_no"`
}

type candidateRow struct {
	RefundAllocationNo     string `json:"refund_allocation_no"`
	TaxInvoiceNo           string `json:"tax_invoice_no"`
	TaxInvoiceAllocationNo string `json:"tax_invoice_allocation_no"`
	TaxInvoiceItemNo       string `json:"tax_invoice_item_no"`
	RequestedGrossMinor    int64  `json:"requested_gross_minor"`
	AvailableGrossMinor    int64  `json:"available_gross_minor"`
	AvailableNetMinor      int64  `json:"available_net_minor"`
	AvailableTaxMinor      int64  `json:"available_tax_minor"`
}

type sourceCandidate struct {
	BillingInvoiceItemNo   string `json:"billing_invoice_item_no"`
	CustomerNo             string `json:"customer_no"`
	CurrencyCode           string `json:"currency_code"`
	TaxInvoiceNo           string `json:"tax_invoice_no"`
	TaxInvoiceAllocationNo string `json:"tax_invoice_allocation_no"`
	TaxInvoiceItemNo       string `json:"tax_invoice_item_no"`
	AvailableGrossMinor    int64  `json:"available_gross_minor"`
	AvailableNetMinor      int64  `json:"available_net_minor"`
	AvailableTaxMinor      int64  `json:"available_tax_minor"`
}

type replayRow struct {
	Fingerprint string `json:"request_fingerprint"`
}

func loadTask(ctx context.Context, database gdb.DB, taskNo string, lock bool) (*taskRow, error) {
	return scanTask(func(pointer any, query string, arguments ...any) error {
		return database.GetScan(ctx, pointer, query, arguments...)
	}, taskNo, lock)
}

func loadTaskTX(ctx context.Context, tx gdb.TX, taskNo string, lock bool) (*taskRow, error) {
	return scanTask(func(pointer any, query string, arguments ...any) error {
		return tx.Ctx(ctx).GetScan(pointer, query, arguments...)
	}, taskNo, lock)
}

type scanFunction func(any, string, ...any) error

func scanTask(scan scanFunction, taskNo string, lock bool) (*taskRow, error) {
	query := `
SELECT id, task_no, tax_invoice_no, refund_operation_no, customer_no, currency_code,
       status_code, manual_reason_code, allocation_resolved,
       customer_confirmation_required, provider_code, external_task_ref,
       gross_amount_minor, net_amount_minor, tax_amount_minor, version,
       created_at, completed_at, completion_evidence_no
FROM plugin_idc_compliance_red_letter_task
WHERE tenant_id = ? AND task_no = ?`
	if lock {
		query += " FOR UPDATE"
	}
	var row *taskRow
	err := scan(&row, query, idccap.DefaultTenantID, taskNo)
	return row, err
}

func loadCandidatesTX(ctx context.Context, tx gdb.TX, taskNo string) ([]candidateRow, error) {
	return scanCandidates(func(pointer any, query string, arguments ...any) error {
		return tx.Ctx(ctx).GetScan(pointer, query, arguments...)
	}, taskNo)
}

func scanCandidates(scan scanFunction, taskNo string) ([]candidateRow, error) {
	var rows []candidateRow
	err := scan(&rows, `
SELECT refund_allocation_no, tax_invoice_no, tax_invoice_allocation_no,
       tax_invoice_item_no, requested_gross_minor, available_gross_minor,
       available_net_minor, available_tax_minor
FROM plugin_idc_compliance_red_letter_candidate
WHERE tenant_id = ? AND task_no = ?
ORDER BY refund_allocation_no, tax_invoice_allocation_no, id`, idccap.DefaultTenantID, taskNo)
	return rows, err
}

func loadSourceCandidatesTX(
	ctx context.Context,
	tx gdb.TX,
	billingItemNos []string,
	customerNo string,
	currencyCode string,
) ([]sourceCandidate, error) {
	return scanSourceCandidates(func(pointer any, query string, arguments ...any) error {
		return tx.Ctx(ctx).GetScan(pointer, query, arguments...)
	}, billingItemNos, customerNo, currencyCode)
}

func scanSourceCandidates(
	scan scanFunction,
	billingItemNos []string,
	customerNo string,
	currencyCode string,
) ([]sourceCandidate, error) {
	if len(billingItemNos) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(billingItemNos)), ",")
	arguments := make([]any, 0, len(billingItemNos)+4)
	for _, itemNo := range billingItemNos {
		arguments = append(arguments, itemNo)
	}
	arguments = append(arguments, idccap.DefaultTenantID, customerNo, currencyCode,
		taxinvoicecap.MaximumItemsPerRequest+1)
	var rows []sourceCandidate
	err := scan(&rows, `
WITH reserved AS (
    SELECT tax_invoice_item_no, COALESCE(SUM(gross_amount_minor), 0) AS gross_minor
    FROM plugin_idc_compliance_red_letter_allocation
    WHERE tax_invoice_item_no IS NOT NULL
    GROUP BY tax_invoice_item_no
)
SELECT item.billing_invoice_item_no, invoice.customer_no,
       invoice.billing_currency_code AS currency_code,
       item.tax_invoice_no,
       item.allocation_no AS tax_invoice_allocation_no, item.item_no AS tax_invoice_item_no,
       item.gross_amount_minor - COALESCE(reserved.gross_minor, 0) AS available_gross_minor,
       item.net_amount_minor AS available_net_minor,
       item.tax_amount_minor AS available_tax_minor
FROM plugin_idc_compliance_tax_invoice_item AS item
JOIN plugin_idc_compliance_tax_invoice AS invoice
  ON invoice.tenant_id = item.tenant_id AND invoice.tax_invoice_no = item.tax_invoice_no
LEFT JOIN reserved ON reserved.tax_invoice_item_no = item.item_no
WHERE item.billing_invoice_item_no IN (`+placeholders+`)
  AND item.tenant_id = ?
  AND invoice.customer_no = ?
  AND invoice.billing_currency_code = ?
  AND invoice.status_code IN ('issued', 'delivered', 'red_letter_pending', 'partially_red_lettered')
  AND item.gross_amount_minor > COALESCE(reserved.gross_minor, 0)
ORDER BY item.billing_invoice_item_no, invoice.issued_at, item.tax_invoice_no
LIMIT ?`, arguments...)
	if err != nil {
		return nil, err
	}
	if len(rows) > taxinvoicecap.MaximumItemsPerRequest {
		return nil, errInconsistent
	}
	for index := range rows {
		row := &rows[index]
		originalGross := row.AvailableNetMinor + row.AvailableTaxMinor
		if originalGross <= 0 || row.AvailableGrossMinor <= 0 || row.AvailableGrossMinor > originalGross {
			return nil, errInconsistent
		}
		row.AvailableNetMinor = proportionalNet(row.AvailableGrossMinor, row.AvailableNetMinor, originalGross)
		row.AvailableTaxMinor = row.AvailableGrossMinor - row.AvailableNetMinor
	}
	return rows, nil
}

func proportionalNet(gross, sourceNet, sourceGross int64) int64 {
	if gross == sourceGross {
		return sourceNet
	}
	return gross * sourceNet / sourceGross
}

func projectTask(ctx context.Context, database gdb.DB, row *taskRow) (*Task, error) {
	if row == nil || row.Version <= 0 || row.GrossAmountMinor <= 0 || row.CreatedAt.IsZero() ||
		row.CustomerNo == nil || row.CurrencyCode == nil {
		return nil, errInconsistent
	}
	var counts struct {
		CandidateCount  int `json:"candidate_count"`
		AllocationCount int `json:"allocation_count"`
	}
	err := database.GetScan(ctx, &counts, `
SELECT
 (SELECT COUNT(*) FROM plugin_idc_compliance_red_letter_candidate
   WHERE tenant_id = ? AND task_no = ?) AS candidate_count,
 (SELECT COUNT(*) FROM plugin_idc_compliance_red_letter_allocation
   WHERE tenant_id = ? AND task_no = ?) AS allocation_count`,
		idccap.DefaultTenantID, row.TaskNo, idccap.DefaultTenantID, row.TaskNo)
	if err != nil {
		return nil, err
	}
	task := &Task{
		TaskNo: row.TaskNo, RefundOperationNo: row.RefundOperationNo,
		CustomerNo: *row.CustomerNo, CurrencyCode: *row.CurrencyCode,
		StatusCode: row.StatusCode, ManualReasonCode: row.ManualReasonCode,
		AllocationResolved:           row.AllocationResolved,
		CustomerConfirmationRequired: row.CustomerConfirmationRequired,
		GrossAmountMinor:             row.GrossAmountMinor, NetAmountMinor: row.NetAmountMinor,
		TaxAmountMinor: row.TaxAmountMinor, Version: row.Version,
		CreatedAt: row.CreatedAt.UTC(), CompletedAt: row.CompletedAt,
		CandidateCount: counts.CandidateCount, AllocationCount: counts.AllocationCount,
	}
	if row.TaxInvoiceNo != nil {
		task.TaxInvoiceNo = *row.TaxInvoiceNo
	}
	if row.CompletionEvidenceNo != nil {
		task.CompletionEvidenceNo = *row.CompletionEvidenceNo
	}
	if task.CompletedAt != nil {
		value := task.CompletedAt.UTC()
		task.CompletedAt = &value
	}
	return task, nil
}

func loadReplay(ctx context.Context, database gdb.DB, taskNo, action, digest string) (*replayRow, error) {
	return scanReplay(func(pointer any, query string, arguments ...any) error {
		return database.GetScan(ctx, pointer, query, arguments...)
	}, taskNo, action, digest)
}

func loadReplayTX(ctx context.Context, tx gdb.TX, taskNo, action, digest string) (*replayRow, error) {
	return scanReplay(func(pointer any, query string, arguments ...any) error {
		return tx.Ctx(ctx).GetScan(pointer, query, arguments...)
	}, taskNo, action, digest)
}

func scanReplay(scan scanFunction, taskNo, action, digest string) (*replayRow, error) {
	var row *replayRow
	err := scan(&row, `
SELECT request_fingerprint AS fingerprint
FROM plugin_idc_compliance_red_letter_action
WHERE tenant_id = ? AND task_no = ? AND action_code = ? AND idempotency_digest = ?`,
		idccap.DefaultTenantID, taskNo, action, digest)
	return row, err
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
