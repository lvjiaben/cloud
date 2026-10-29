// This file stores reconciliation evidence in bounded batches inside the
// caller's transaction, retaining the existing immutable conflict behavior.

package reconciliation

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
)

// writeBatchSize keeps statements below database parameter limits.
const writeBatchSize = 500

// persistSources preserves provider evidence and local customer attribution.
func (s *Service) persistSources(ctx context.Context, batchNo string, records []SourceRecord, localPayments gdb.Result) error {
	// Bound SQL statement size without changing the batch transaction or
	// the existing source/line conflict semantics.
	customers := make(map[string]string, len(localPayments))
	for _, payment := range localPayments {
		customers[payment["attempt_no"].String()] = payment["customer_ref_digest"].String()
	}
	for start := 0; start < len(records); start += writeBatchSize {
		end := min(start+writeBatchSize, len(records))
		values := make([]string, 0, end-start)
		args := make([]any, 0, (end-start)*12)
		for _, record := range records[start:end] {
			customerDigest := strings.Repeat("0", 64)
			if record.RecordType == "payment" {
				if digest, ok := customers[record.LocalSubjectNo]; ok {
					customerDigest = digest
				}
			}
			values = append(values, "(?,?,?,?,?,?,?,?,?,?,?,?)")
			args = append(args, stableNo("prs", batchNo, record.RecordType, record.ProviderEventReference), batchNo, record.RecordType, record.LocalSubjectNo, record.ProviderEventReference, record.ExternalReference, customerDigest, record.CurrencyCode, record.AmountMinor, record.StatusCode, record.EvidenceDigest, record.ObservedAt.UTC())
		}
		if _, txErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_payment_reconciliation_source(source_no,batch_no,record_type_code,local_subject_no,provider_event_reference,external_reference,customer_ref_digest,currency_code,amount_minor,observed_status_code,evidence_digest,observed_at) VALUES `+strings.Join(values, ",")+` ON CONFLICT(source_no) DO NOTHING`, args...); txErr != nil {
			return txErr
		}
	}

	return nil
}

// persistLines stores every difference without changing its review state.
func (s *Service) persistLines(ctx context.Context, batchNo string, lines []builtLine) (int, error) {
	differenceCount := 0
	for start := 0; start < len(lines); start += writeBatchSize {
		end := min(start+writeBatchSize, len(lines))
		values := make([]string, 0, end-start)
		args := make([]any, 0, (end-start)*14)
		for _, line := range lines[start:end] {
			if line.DifferenceCode != "none" {
				differenceCount++
			}
			values = append(values, "(?,?,?,?,?,?,?,?,?,'pending',?,?,?,?,?)")
			args = append(args, line.LineNo, batchNo, line.AttemptNo, line.RefundAttemptNo, line.ExternalReference, line.DifferenceCode, line.LocalAmountMinor, line.ChannelAmountMinor, line.CurrencyCode, line.EvidenceDigest, line.RecordType, line.CustomerDigest, line.LocalStatus, line.ChannelStatus)
		}
		if _, txErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_payment_reconciliation_line(line_no,batch_no,attempt_no,refund_attempt_no,external_reference,difference_code,local_amount_minor,channel_amount_minor,currency_code,resolution_code,evidence_digest,record_type_code,customer_ref_digest,local_status_code,channel_status_code) VALUES `+strings.Join(values, ",")+` ON CONFLICT(line_no) DO NOTHING`, args...); txErr != nil {
			return 0, txErr
		}
	}
	return differenceCount, nil
}
