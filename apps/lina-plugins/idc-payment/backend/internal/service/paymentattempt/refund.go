package paymentattempt

import (
	"context"
	"database/sql"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/billingcap"
	"lina-plugin-idc-payment/backend/cap/paymentcap"
)

type RefundPaymentContext struct {
	PaymentAttemptNo       string
	ChannelNo              string
	ModeCode               string
	CurrencyCode           string
	TotalAmountMinor       int64
	ChannelAmountMinor     int64
	ExternalTradeReference string
}

type RefundAttempt struct {
	RefundAttemptNo string
	ChannelNo       string
	CurrencyCode    string
	AmountMinor     int64
	StatusCode      string
	ResultCode      string
	ResultReference string
	EvidenceDigest  string
}

// RefundResultOutcome reports how a verified provider result affected the local attempt.
type RefundResultOutcome struct {
	RefundAttemptNo string
	StatusCode      string
	Reused          bool
}

func (s *Store) ClaimRefundExecution(
	ctx context.Context,
	workerNo string,
	leaseSeconds int,
) (*billingcap.RefundExecutionWork, error) {
	return s.billing.ClaimRefundExecution(ctx, billingcap.ClaimRefundExecution{
		WorkerNo: workerNo, LeaseSeconds: leaseSeconds,
	})
}

func (s *Store) ReportRefundExecution(
	ctx context.Context,
	input billingcap.ReportRefundExecution,
) (*billingcap.RefundExecutionReport, error) {
	return s.billing.ReportRefundExecution(ctx, input)
}

func (s *Store) GetRefundPaymentContext(
	ctx context.Context,
	paymentAttemptNo string,
) (*RefundPaymentContext, error) {
	paymentAttemptNo = strings.TrimSpace(paymentAttemptNo)
	row, err := s.database.GetOne(ctx, `
SELECT attempt_no,channel_no,mode_code,currency_code,amount_minor,
       amount_minor-balance_locked_minor AS channel_amount_minor,
       external_trade_reference
FROM plugin_idc_payment_attempt
WHERE tenant_id=0 AND attempt_no=? AND status_code='succeeded'
  AND external_trade_reference<>''`, paymentAttemptNo)
	if err != nil {
		return nil, err
	}
	if row.IsEmpty() {
		return nil, gerror.New("refundable payment attempt is unavailable")
	}
	return &RefundPaymentContext{
		PaymentAttemptNo: row["attempt_no"].String(), ChannelNo: row["channel_no"].String(),
		ModeCode: row["mode_code"].String(), CurrencyCode: row["currency_code"].String(),
		TotalAmountMinor:       row["amount_minor"].Int64(),
		ChannelAmountMinor:     row["channel_amount_minor"].Int64(),
		ExternalTradeReference: row["external_trade_reference"].String(),
	}, nil
}

func (s *Store) EnsureRefundAttempt(
	ctx context.Context,
	work billingcap.RefundExecutionWork,
	payment RefundPaymentContext,
) (*RefundAttempt, error) {
	refundAttemptNo := stableNo("prf", work.CommandNo)
	capabilityCode := "automatic"
	_, err := s.database.Exec(ctx, `
INSERT INTO plugin_idc_payment_refund_attempt
    (refund_attempt_no,refund_request_no,payment_attempt_no,channel_no,
     currency_code,amount_minor,request_digest,capability_code,status_code)
VALUES (?,?,?,?,?,?,? ,?,'pending')
ON CONFLICT (refund_request_no,payment_attempt_no) DO NOTHING`,
		refundAttemptNo, work.RefundSourceNo, payment.PaymentAttemptNo, payment.ChannelNo,
		work.CurrencyCode, work.AmountMinor, work.RequestDigest, capabilityCode)
	if err != nil {
		return nil, err
	}
	return s.GetRefundAttempt(ctx, work.RefundSourceNo, payment.PaymentAttemptNo)
}

func (s *Store) GetRefundAttempt(
	ctx context.Context,
	refundRequestNo string,
	paymentAttemptNo string,
) (*RefundAttempt, error) {
	row, err := s.database.GetOne(ctx, `
SELECT refund_attempt_no,channel_no,currency_code,amount_minor,status_code,result_code,external_refund_reference,
       CASE WHEN result_evidence_digest=repeat('0',64) THEN '' ELSE result_evidence_digest END AS evidence_digest
FROM plugin_idc_payment_refund_attempt
WHERE refund_request_no=? AND payment_attempt_no=?`, refundRequestNo, paymentAttemptNo)
	if err != nil || row.IsEmpty() {
		if err == nil {
			err = gerror.New("refund attempt is unavailable")
		}
		return nil, err
	}
	return &RefundAttempt{
		RefundAttemptNo: row["refund_attempt_no"].String(),
		ChannelNo:       row["channel_no"].String(),
		CurrencyCode:    row["currency_code"].String(),
		AmountMinor:     row["amount_minor"].Int64(),
		StatusCode:      row["status_code"].String(),
		ResultCode:      row["result_code"].String(),
		ResultReference: row["external_refund_reference"].String(),
		EvidenceDigest:  row["evidence_digest"].String(),
	}, nil
}

func (s *Store) CompleteRefundAttempt(
	ctx context.Context,
	refundAttemptNo string,
	status string,
	resultCode string,
	resultReference string,
	evidenceDigest string,
) error {
	resultUnknown := status == "result_unknown"
	result, err := s.database.Exec(ctx, `
UPDATE plugin_idc_payment_refund_attempt
SET status_code=?,external_refund_reference=CASE WHEN ?='' THEN external_refund_reference ELSE ? END,result_unknown=?,
    result_code=?,result_evidence_digest=?,
    confirmed_at=CASE WHEN ? IN ('succeeded','failed','manual_required','result_unknown') THEN COALESCE(confirmed_at,CURRENT_TIMESTAMP) ELSE confirmed_at END,
    finished_at=CASE WHEN ? IN ('succeeded','failed','manual_required','result_unknown')
                     THEN CURRENT_TIMESTAMP ELSE finished_at END,
    version=version+1
WHERE refund_attempt_no=? AND status_code IN ('pending','submitted')`,
		status, resultReference, resultReference, resultUnknown, resultCode, evidenceDigest, status, status, refundAttemptNo)
	if err != nil {
		return err
	}
	return requireOneRefundAttempt(result)
}

// ApplyVerifiedRefundResult stores one verified callback or query result without deciding refund eligibility.
func (s *Store) ApplyVerifiedRefundResult(
	ctx context.Context,
	channelNo string,
	result paymentcap.VerifiedResult,
	evidenceType string,
) (*RefundResultOutcome, error) {
	if strings.TrimSpace(channelNo) == "" || strings.TrimSpace(result.AttemptNo) == "" ||
		result.AmountMinor <= 0 || len(result.EvidenceDigest) != 64 || strings.TrimSpace(evidenceType) == "" {
		return nil, gerror.New("verified refund result is invalid")
	}
	var output *RefundResultOutcome
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		attempt, loadErr := s.getRefundAttemptByNoForUpdate(ctx, result.AttemptNo)
		if loadErr != nil {
			return loadErr
		}
		if attempt.ChannelNo != channelNo || attempt.CurrencyCode != result.Currency ||
			attempt.AmountMinor != result.AmountMinor {
			return gerror.New("verified refund result does not match the stored attempt")
		}
		status, resultCode, mapErr := projectVerifiedRefundStatus(result.Status)
		if mapErr != nil {
			return mapErr
		}
		if evidenceErr := s.RecordRefundEvidence(
			ctx, attempt.RefundAttemptNo, evidenceType, result.EvidenceDigest, resultCode,
		); evidenceErr != nil {
			return evidenceErr
		}
		if attempt.StatusCode == status && attempt.EvidenceDigest == result.EvidenceDigest {
			output = &RefundResultOutcome{RefundAttemptNo: attempt.RefundAttemptNo, StatusCode: status, Reused: true}
			return nil
		}
		if isTerminalRefundStatus(attempt.StatusCode) {
			if attempt.StatusCode == "result_unknown" {
				updated, updateErr := s.database.Exec(ctx, `
UPDATE plugin_idc_payment_refund_attempt
SET status_code='manual_required',result_unknown=FALSE,result_code='late_provider_result',
    result_evidence_digest=?,manual_evidence_digest=?,version=version+1
WHERE refund_attempt_no=? AND status_code='result_unknown'`,
					result.EvidenceDigest, result.EvidenceDigest, attempt.RefundAttemptNo)
				if updateErr != nil {
					return updateErr
				}
				if updateErr = requireOneRefundAttempt(updated); updateErr != nil {
					return updateErr
				}
				output = &RefundResultOutcome{RefundAttemptNo: attempt.RefundAttemptNo, StatusCode: "manual_required"}
				return nil
			}
			return gerror.New("verified refund result conflicts with the stored terminal result")
		}
		updated, updateErr := s.database.Exec(ctx, `
UPDATE plugin_idc_payment_refund_attempt
SET status_code=?,external_refund_reference=CASE WHEN ?='' THEN external_refund_reference ELSE ? END,
    result_unknown=FALSE,result_code=?,result_evidence_digest=?,
    confirmed_at=CASE WHEN ? IN ('succeeded','failed','manual_required') THEN COALESCE(confirmed_at,CURRENT_TIMESTAMP) ELSE confirmed_at END,
    finished_at=CASE WHEN ? IN ('succeeded','failed','manual_required') THEN CURRENT_TIMESTAMP ELSE NULL END,
    version=version+1
WHERE refund_attempt_no=? AND status_code IN ('pending','submitted')`,
			status, result.ExternalTradeReference, result.ExternalTradeReference, resultCode,
			result.EvidenceDigest, status, status, attempt.RefundAttemptNo)
		if updateErr != nil {
			return updateErr
		}
		if updateErr = requireOneRefundAttempt(updated); updateErr != nil {
			return updateErr
		}
		output = &RefundResultOutcome{RefundAttemptNo: attempt.RefundAttemptNo, StatusCode: status}
		return nil
	})
	return output, err
}

func (s *Store) getRefundAttemptByNoForUpdate(ctx context.Context, refundAttemptNo string) (*RefundAttempt, error) {
	row, err := s.database.GetOne(ctx, `
SELECT refund_attempt_no,channel_no,currency_code,amount_minor,status_code,result_code,
       external_refund_reference,
       CASE WHEN result_evidence_digest=repeat('0',64) THEN '' ELSE result_evidence_digest END AS evidence_digest
FROM plugin_idc_payment_refund_attempt
WHERE refund_attempt_no=? FOR UPDATE`, refundAttemptNo)
	if err != nil {
		return nil, err
	}
	if row.IsEmpty() {
		return nil, gerror.New("refund attempt is unavailable")
	}
	return &RefundAttempt{
		RefundAttemptNo: row["refund_attempt_no"].String(), ChannelNo: row["channel_no"].String(),
		CurrencyCode: row["currency_code"].String(), AmountMinor: row["amount_minor"].Int64(),
		StatusCode: row["status_code"].String(), ResultCode: row["result_code"].String(),
		ResultReference: row["external_refund_reference"].String(), EvidenceDigest: row["evidence_digest"].String(),
	}, nil
}

func projectVerifiedRefundStatus(providerStatus string) (string, string, error) {
	switch providerStatus {
	case "succeeded":
		return "succeeded", "refund_succeeded", nil
	case "failed", "closed":
		return "failed", "provider_refund_failed", nil
	case "manual_review":
		return "manual_required", "provider_refund_manual_required", nil
	case "pending":
		return "submitted", "refund_processing", nil
	default:
		return "", "", gerror.New("provider refund status is invalid")
	}
}

func isTerminalRefundStatus(status string) bool {
	return status == "succeeded" || status == "failed" || status == "result_unknown" ||
		status == "manual_required" || status == "manual_confirmed" || status == "disputed"
}

func requireOneRefundAttempt(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return gerror.New("refund attempt changed concurrently")
	}
	return nil
}

func (s *Store) RecordRefundEvidence(
	ctx context.Context,
	subjectNo string,
	evidenceType string,
	evidenceDigest string,
	summaryCode string,
) error {
	evidenceNo := stableNo("pev", subjectNo, evidenceType, evidenceDigest)
	_, err := s.database.Exec(ctx, `
INSERT INTO plugin_idc_payment_evidence
    (evidence_no,subject_type,subject_no,evidence_type,evidence_digest,
     summary_code,observed_at)
VALUES (?,'refund_attempt',?,?,?,?,CURRENT_TIMESTAMP)
ON CONFLICT(subject_type,subject_no,evidence_type,evidence_digest) DO NOTHING`,
		evidenceNo, subjectNo, evidenceType, evidenceDigest, summaryCode)
	return err
}
