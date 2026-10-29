// Package paymentattempt owns durable payment attempts, callback receipts and asynchronous core funding work.
package paymentattempt

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"lina-plugin-idc-core/backend/cap/billingcap"
	"lina-plugin-idc-payment/backend/cap/paymentcap"
)

type Store struct {
	database gdb.DB
	billing  billingcap.Service
}

// ReconciliationSupplementInput identifies one frozen two-person review. The
// store rechecks every immutable money and ownership field before it reuses the
// ordinary payment confirmation work queue.
type ReconciliationSupplementInput struct {
	RequestNo       string
	ReviewerUserID  int
	ExpectedVersion int64
}

func (s *Store) ApplyReconciliationSupplement(ctx context.Context, input ReconciliationSupplementInput) error {
	if input.RequestNo == "" || input.ReviewerUserID <= 0 || input.ExpectedVersion <= 0 {
		return errors.New("invalid reconciliation supplement review")
	}
	return s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		row, err := tx.GetOne(`SELECT r.request_no,r.line_no,r.attempt_no,r.channel_no,r.customer_ref_digest,r.currency_code,r.amount_minor,r.external_reference,r.evidence_digest,r.content_digest,r.status_code,r.submitter_user_id,r.version,l.difference_code,l.channel_status_code,l.evidence_digest AS line_evidence,b.channel_no AS batch_channel,a.customer_ref_digest AS actual_customer,a.currency_code AS actual_currency,(a.amount_minor-a.balance_locked_minor) AS actual_amount,a.external_trade_reference,a.status_code AS attempt_status FROM plugin_idc_payment_reconciliation_supplement r JOIN plugin_idc_payment_reconciliation_line l ON l.line_no=r.line_no JOIN plugin_idc_payment_reconciliation_batch b ON b.batch_no=l.batch_no JOIN plugin_idc_payment_reconciliation_source src ON src.batch_no=l.batch_no AND src.record_type_code=l.record_type_code AND src.local_subject_no=r.attempt_no AND src.external_reference=r.external_reference AND src.evidence_digest=r.evidence_digest JOIN plugin_idc_payment_attempt a ON a.attempt_no=r.attempt_no WHERE r.request_no=? FOR UPDATE`, input.RequestNo)
		if err != nil {
			return err
		}
		if row.IsEmpty() || row["status_code"].String() != "pending_review" || row["version"].Int64() != input.ExpectedVersion {
			return errors.New("supplement review is not pending or changed")
		}
		if row["submitter_user_id"].Int() == input.ReviewerUserID {
			return errors.New("supplement submitter cannot review the same request")
		}
		contentDigest := sha256Hex(strings.Join([]string{
			row["line_no"].String(), row["attempt_no"].String(), row["channel_no"].String(),
			row["actual_customer"].String(), row["actual_currency"].String(),
			fmt.Sprint(row["actual_amount"].Int64()), row["external_reference"].String(),
			row["evidence_digest"].String(),
		}, "\x00"))
		eligible := row["difference_code"].String() == "local_missing" && row["channel_status_code"].String() == "succeeded" &&
			row["channel_no"].String() == row["batch_channel"].String() &&
			row["customer_ref_digest"].String() == row["actual_customer"].String() &&
			row["currency_code"].String() == row["actual_currency"].String() && row["amount_minor"].Int64() == row["actual_amount"].Int64() &&
			row["evidence_digest"].String() == row["line_evidence"].String() && row["content_digest"].String() == contentDigest &&
			row["external_reference"].String() != "" && row["attempt_status"].String() != "succeeded" &&
			(row["external_trade_reference"].String() == "" || row["external_trade_reference"].String() == row["external_reference"].String())
		if !eligible {
			return errors.New("supplement facts no longer match the original payment")
		}
		conflict, err := tx.GetOne(`SELECT attempt_no FROM plugin_idc_payment_attempt WHERE channel_no=? AND external_trade_reference=? AND attempt_no<>? LIMIT 1`, row["channel_no"].String(), row["external_reference"].String(), row["attempt_no"].String())
		if err != nil {
			return err
		}
		if !conflict.IsEmpty() {
			return errors.New("provider transaction is already bound to another payment")
		}
		if _, err = tx.Exec(`UPDATE plugin_idc_payment_attempt SET status_code='confirming',external_trade_reference=?,funding_result_digest=?,result_unknown=FALSE,updated_at=CURRENT_TIMESTAMP,version=version+1 WHERE attempt_no=? AND status_code<>'succeeded'`, row["external_reference"].String(), row["evidence_digest"].String(), row["attempt_no"].String()); err != nil {
			return err
		}
		if _, err = tx.Exec(`INSERT INTO plugin_idc_payment_work_item(work_no,work_type,subject_no,evidence_digest) VALUES(?,'confirm_payment',?,?) ON CONFLICT(work_type,subject_no) DO NOTHING`, stableNo("pwk", "confirm_payment", row["attempt_no"].String()), row["attempt_no"].String(), row["evidence_digest"].String()); err != nil {
			return err
		}
		if _, err = tx.Exec(`UPDATE plugin_idc_payment_reconciliation_supplement SET status_code='applied',reviewer_user_id=?,reviewed_at=CURRENT_TIMESTAMP,applied_at=CURRENT_TIMESTAMP,version=version+1 WHERE request_no=? AND status_code='pending_review'`, input.ReviewerUserID, input.RequestNo); err != nil {
			return err
		}
		_, err = tx.Exec(`UPDATE plugin_idc_payment_reconciliation_channel_health SET unresolved_count=GREATEST(0,unresolved_count-1),version=version+1,updated_at=CURRENT_TIMESTAMP WHERE channel_no=?`, row["channel_no"].String())
		return err
	})
}

type CreateInput struct {
	AttemptNo, ChannelNo, InvoiceNo, OrderNo, CustomerRefDigest, CurrencyCode string
	PurposeCode, PurposeReference                                             string
	AmountMinor, BalanceLockedMinor                                           int64
	BalanceLockReference, ModeCode, RequestDigest                             string
	ExpiresAt                                                                 time.Time
}

type CallbackInput struct {
	ChannelNo     string
	Result        paymentcap.VerifiedResult
	PayloadDigest string
}

type CallbackOutcome struct {
	ReceiptNo, AttemptNo, Status string
	Duplicate                    bool
}

type ClaimedWork struct {
	WorkNo, AttemptNo, InvoiceNo, CurrencyCode, EvidenceDigest, LeaseToken string
	PurposeCode, PurposeReference                                          string
	AmountMinor, TotalAmountMinor, BalanceAmountMinor                      int64
	BalanceHoldNo                                                          string
}

type PrepareInput struct {
	AttemptNo, PaymentContextNo, ChannelNo, ModeCode, RequestDigest string
	BalanceAmountMinor                                              int64
}

type Status struct {
	InvoiceNo, OrderNo                                       string
	AttemptNo, StatusCode, ChannelNo, ModeCode, CurrencyCode string
	PurposeCode, PurposeReference                            string
	TotalAmountMinor, ChannelAmountMinor, BalanceAmountMinor int64
	ExpiresAt, ConfirmedAt                                   time.Time
	QueryQueued                                              bool
}

type CreditPrepareInput struct {
	AttemptNo, StatementNo, ChannelNo, ModeCode, RequestDigest string
	AmountMinor                                                int64
}

type RechargePrepareInput struct {
	AttemptNo, ChannelNo, ModeCode, RequestDigest, CurrencyCode string
	AmountMinor                                                 int64
}

type ClaimedQuery struct {
	WorkNo, AttemptNo, ChannelNo, ExternalTradeReference, LeaseToken string
}

func (s *Store) CurrentActor(ctx context.Context) (*billingcap.PaymentActor, error) {
	return s.billing.CurrentPaymentActor(ctx)
}

func (s *Store) PrepareAndCreate(ctx context.Context, input PrepareInput) (*billingcap.PreparedChannelPayment, error) {
	prepared, err := s.billing.PrepareChannelPayment(ctx, billingcap.PrepareChannelPayment{PaymentAttemptNo: input.AttemptNo, PaymentContextNo: input.PaymentContextNo, BalanceAmountMinor: input.BalanceAmountMinor})
	if err != nil {
		return nil, err
	}
	err = s.Create(ctx, CreateInput{
		AttemptNo: input.AttemptNo, ChannelNo: input.ChannelNo, InvoiceNo: prepared.InvoiceNo,
		OrderNo: prepared.OrderNo, CustomerRefDigest: prepared.CustomerRefDigest,
		CurrencyCode: prepared.CurrencyCode, AmountMinor: prepared.TotalAmountMinor,
		BalanceLockedMinor: prepared.BalanceAmountMinor, BalanceLockReference: prepared.BalanceHoldNo,
		ModeCode: input.ModeCode, RequestDigest: input.RequestDigest, ExpiresAt: prepared.ExpiresAt,
	})
	if err != nil && prepared.BalanceHoldNo != "" {
		_, _ = s.billing.ReleaseChannelPayment(ctx, billingcap.ReleaseChannelPayment{PaymentAttemptNo: input.AttemptNo, BalanceHoldNo: prepared.BalanceHoldNo})
	}
	return prepared, err
}

func (s *Store) PrepareCreditAndCreate(ctx context.Context, input CreditPrepareInput) (*billingcap.PreparedCreditRepayment, error) {
	prepared, err := s.billing.PrepareCreditRepayment(ctx, billingcap.PrepareCreditRepayment{PaymentAttemptNo: input.AttemptNo, StatementNo: input.StatementNo, AmountMinor: input.AmountMinor})
	if err != nil {
		return nil, err
	}
	err = s.Create(ctx, CreateInput{AttemptNo: input.AttemptNo, ChannelNo: input.ChannelNo, PurposeCode: "credit_statement", PurposeReference: prepared.StatementNo, CustomerRefDigest: prepared.CustomerRefDigest, CurrencyCode: prepared.CurrencyCode, AmountMinor: prepared.AmountMinor, ModeCode: input.ModeCode, RequestDigest: input.RequestDigest, ExpiresAt: prepared.ExpiresAt})
	return prepared, err
}

func (s *Store) PrepareRechargeAndCreate(ctx context.Context, input RechargePrepareInput) (*billingcap.PreparedBalanceRecharge, error) {
	prepared, err := s.billing.PrepareBalanceRecharge(ctx, billingcap.PrepareBalanceRecharge{PaymentAttemptNo: input.AttemptNo, CurrencyCode: input.CurrencyCode, AmountMinor: input.AmountMinor})
	if err != nil {
		return nil, err
	}
	err = s.Create(ctx, CreateInput{AttemptNo: input.AttemptNo, ChannelNo: input.ChannelNo, PurposeCode: "balance_recharge", PurposeReference: prepared.RechargeContextNo, CustomerRefDigest: prepared.CustomerRefDigest, CurrencyCode: prepared.CurrencyCode, AmountMinor: prepared.AmountMinor, ModeCode: input.ModeCode, RequestDigest: input.RequestDigest, ExpiresAt: prepared.ExpiresAt})
	return prepared, err
}

func (s *Store) MarkSubmitted(ctx context.Context, attemptNo string) error {
	result, err := s.database.Exec(ctx, `UPDATE plugin_idc_payment_attempt SET status_code=CASE WHEN status_code='created' THEN 'submitted' ELSE status_code END,submitted_at=COALESCE(submitted_at,CURRENT_TIMESTAMP),updated_at=CURRENT_TIMESTAMP,version=version+1 WHERE attempt_no=? AND status_code IN ('created','submitted','confirming','succeeded')`, attemptNo)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return gerror.New("payment attempt cannot be submitted")
	}
	return nil
}

func (s *Store) FailAndRelease(ctx context.Context, attemptNo string) error {
	row, err := s.database.GetOne(ctx, `UPDATE plugin_idc_payment_attempt SET status_code='failed',result_unknown=FALSE,updated_at=CURRENT_TIMESTAMP,version=version+1 WHERE attempt_no=? AND (status_code IN ('created','submitted') OR (status_code='manual_review' AND funding_result_digest=repeat('0',64))) RETURNING balance_lock_reference`, attemptNo)
	if err != nil {
		return err
	}
	if row.IsEmpty() {
		row, err = s.database.GetOne(ctx, `SELECT balance_lock_reference FROM plugin_idc_payment_attempt WHERE attempt_no=? AND status_code='failed'`, attemptNo)
		if err != nil || row.IsEmpty() {
			return err
		}
	}
	holdNo := row["balance_lock_reference"].String()
	if holdNo != "" {
		_, err = s.billing.ReleaseChannelPayment(ctx, billingcap.ReleaseChannelPayment{PaymentAttemptNo: attemptNo, BalanceHoldNo: holdNo})
	}
	return err
}

func (s *Store) ObserveReturn(ctx context.Context, attemptNo, customerRefDigest string) (*Status, error) {
	if attemptNo == "" || len(customerRefDigest) != 64 {
		return nil, errors.New("payment attempt is not visible")
	}
	var result *Status
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		row, loadErr := tx.GetOne(`SELECT a.attempt_no,a.invoice_no,a.order_no,a.status_code,a.channel_no,a.mode_code,a.currency_code,a.purpose_code,a.purpose_reference,a.amount_minor,a.balance_locked_minor,a.expires_at,a.confirmed_at,c.capability_snapshot FROM plugin_idc_payment_attempt a JOIN plugin_idc_payment_channel c ON c.channel_no=a.channel_no WHERE a.attempt_no=? AND a.customer_ref_digest=? FOR UPDATE OF a`, attemptNo, customerRefDigest)
		if loadErr != nil {
			return loadErr
		}
		if row.IsEmpty() {
			return errors.New("payment attempt is not visible")
		}
		queryQueued := false
		if status := row["status_code"].String(); status == "created" || status == "submitted" || status == "confirming" {
			var operations map[string]bool
			if json.Unmarshal(row["capability_snapshot"].Bytes(), &operations) == nil && operations[string(paymentcap.OperationQuery)] {
				_, loadErr = tx.Exec(`INSERT INTO plugin_idc_payment_work_item(work_no,work_type,subject_no,evidence_digest) VALUES(?, 'query_payment', ?, repeat('0',64)) ON CONFLICT(work_type,subject_no) DO NOTHING`, stableNo("pwk", "query_payment", attemptNo), attemptNo)
				if loadErr != nil {
					return loadErr
				}
				queryQueued = true
			}
		}
		result = &Status{InvoiceNo: row["invoice_no"].String(), OrderNo: row["order_no"].String(), AttemptNo: row["attempt_no"].String(), StatusCode: row["status_code"].String(), ChannelNo: row["channel_no"].String(), ModeCode: row["mode_code"].String(), CurrencyCode: row["currency_code"].String(), PurposeCode: row["purpose_code"].String(), PurposeReference: row["purpose_reference"].String(), TotalAmountMinor: row["amount_minor"].Int64(), BalanceAmountMinor: row["balance_locked_minor"].Int64(), ExpiresAt: row["expires_at"].Time().UTC(), ConfirmedAt: row["confirmed_at"].Time().UTC(), QueryQueued: queryQueued}
		result.ChannelAmountMinor = result.TotalAmountMinor - result.BalanceAmountMinor
		return nil
	})
	return result, err
}

func (s *Store) ClaimQuery(ctx context.Context, lease time.Duration) (*ClaimedQuery, error) {
	if lease < time.Second || lease > 10*time.Minute {
		return nil, errors.New("invalid payment work lease")
	}
	token, err := randomToken()
	if err != nil {
		return nil, err
	}
	digest := sha256Hex(token)
	if _, err = s.database.Exec(ctx, `UPDATE plugin_idc_payment_work_item SET status_code='manual_review',lease_token_digest=repeat('0',64),lease_expires_at=NULL,failure_code='QUERY_RETRY_EXHAUSTED',finished_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP WHERE work_type='query_payment' AND status_code='leased' AND lease_expires_at<=CURRENT_TIMESTAMP AND attempt_count>=max_attempts`); err != nil {
		return nil, err
	}
	row, err := s.database.GetOne(ctx, `WITH candidate AS (SELECT id FROM plugin_idc_payment_work_item WHERE work_type='query_payment' AND attempt_count<max_attempts AND ((status_code='pending' AND available_at<=CURRENT_TIMESTAMP) OR (status_code='leased' AND lease_expires_at<=CURRENT_TIMESTAMP)) ORDER BY available_at,id FOR UPDATE SKIP LOCKED LIMIT 1), claimed AS (UPDATE plugin_idc_payment_work_item w SET status_code='leased',attempt_count=attempt_count+1,lease_token_digest=?,lease_expires_at=CURRENT_TIMESTAMP+(?::bigint*INTERVAL '1 millisecond'),updated_at=CURRENT_TIMESTAMP FROM candidate WHERE w.id=candidate.id RETURNING w.work_no,w.subject_no) SELECT claimed.work_no,claimed.subject_no AS attempt_no,a.channel_no,a.external_trade_reference FROM claimed JOIN plugin_idc_payment_attempt a ON a.attempt_no=claimed.subject_no`, digest, lease.Milliseconds())
	if err != nil || row.IsEmpty() {
		return nil, err
	}
	return &ClaimedQuery{WorkNo: row["work_no"].String(), AttemptNo: row["attempt_no"].String(), ChannelNo: row["channel_no"].String(), ExternalTradeReference: row["external_trade_reference"].String(), LeaseToken: token}, nil
}

func (s *Store) RetryQuery(ctx context.Context, work ClaimedQuery) error {
	digest := sha256Hex(work.LeaseToken)
	result, err := s.database.Exec(ctx, `UPDATE plugin_idc_payment_work_item SET status_code=CASE WHEN attempt_count>=max_attempts THEN 'manual_review' ELSE 'pending' END,available_at=CURRENT_TIMESTAMP+(LEAST(300,1<<LEAST(attempt_count,8))*INTERVAL '1 second'),lease_token_digest=repeat('0',64),lease_expires_at=NULL,failure_code='PROVIDER_QUERY_UNAVAILABLE',finished_at=CASE WHEN attempt_count>=max_attempts THEN CURRENT_TIMESTAMP ELSE NULL END,updated_at=CURRENT_TIMESTAMP WHERE work_no=? AND status_code='leased' AND lease_token_digest=?`, work.WorkNo, digest)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return errors.New("payment query work lease is stale")
	}
	return nil
}

func (s *Store) CompleteQuery(ctx context.Context, work ClaimedQuery, result paymentcap.VerifiedResult) error {
	if result.AttemptNo != work.AttemptNo || len(result.EvidenceDigest) != 64 ||
		(result.Status != "succeeded" && result.Status != "pending" && result.Status != "closed" && result.Status != "refunding") {
		return errors.New("payment query result is invalid")
	}
	digest := sha256Hex(work.LeaseToken)
	observationNo := stableNo("pqo", work.AttemptNo, result.EvidenceDigest)
	return s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// Fence the entire result application before observations, callbacks or
		// releasing held funds. The lock is retained until all changes commit.
		lease, err := tx.GetOne(`SELECT w.work_no FROM plugin_idc_payment_work_item w JOIN plugin_idc_payment_attempt a ON a.attempt_no=w.subject_no WHERE w.work_no=? AND w.work_type='query_payment' AND w.subject_no=? AND a.channel_no=? AND w.status_code='leased' AND w.lease_token_digest=? AND w.lease_expires_at>CURRENT_TIMESTAMP FOR UPDATE OF w`, work.WorkNo, work.AttemptNo, work.ChannelNo, digest)
		if err != nil {
			return err
		}
		if lease.IsEmpty() {
			return gerror.New("payment query work lease is stale")
		}
		if _, err = tx.Exec(`INSERT INTO plugin_idc_payment_query_observation(observation_no,attempt_no,query_reference,authority_code,observed_status_code,evidence_digest) VALUES(?,?,?,'authoritative',?,?) ON CONFLICT(attempt_no,query_reference) DO NOTHING`, observationNo, work.AttemptNo, observationNo, result.Status, result.EvidenceDigest); err != nil {
			return err
		}
		if result.Status == "succeeded" {
			if _, err = s.RecordVerifiedCallback(ctx, CallbackInput{ChannelNo: work.ChannelNo, Result: result, PayloadDigest: result.EvidenceDigest}); err != nil {
				return err
			}
		}
		if result.Status == "closed" {
			if err = s.FailAndRelease(ctx, work.AttemptNo); err != nil {
				return err
			}
		}
		status := "succeeded"
		if result.Status == "pending" || result.Status == "refunding" {
			status = "pending"
		}
		completed, err := tx.Exec(`UPDATE plugin_idc_payment_work_item SET status_code=?,available_at=CASE WHEN ?='pending' THEN CURRENT_TIMESTAMP+INTERVAL '30 seconds' ELSE available_at END,lease_token_digest=repeat('0',64),lease_expires_at=NULL,finished_at=CASE WHEN ?='succeeded' THEN CURRENT_TIMESTAMP ELSE NULL END,updated_at=CURRENT_TIMESTAMP WHERE work_no=? AND status_code='leased' AND lease_token_digest=?`, status, status, status, work.WorkNo, digest)
		if err != nil {
			return err
		}
		affected, err := completed.RowsAffected()
		if err != nil {
			return err
		}
		if affected != 1 {
			return gerror.New("payment query work lease is stale")
		}
		return nil
	})
}

func New(database gdb.DB, billing billingcap.Service) (*Store, error) {
	if database == nil || billing == nil {
		return nil, errors.New("payment attempt dependencies are required")
	}
	return &Store{database: database, billing: billing}, nil
}

func (s *Store) Create(ctx context.Context, input CreateInput) error {
	if err := normalizeAndValidateCreateInput(&input); err != nil {
		return err
	}
	if err := s.ensureCreateChannel(ctx, input); err != nil {
		return err
	}
	_, err := s.database.Exec(ctx, `
INSERT INTO plugin_idc_payment_attempt
  (attempt_no, channel_no, invoice_no, order_no, purpose_code, purpose_reference, customer_ref_digest, currency_code,
   amount_minor, balance_lock_reference, balance_locked_minor, mode_code, request_digest, expires_at)
SELECT ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
WHERE EXISTS (
  SELECT 1 FROM plugin_idc_payment_channel
  WHERE channel_no = ? AND status_code = 'enabled' AND deleted_at IS NULL
)
ON CONFLICT (attempt_no) DO NOTHING`,
		input.AttemptNo, input.ChannelNo, input.InvoiceNo, input.OrderNo, input.PurposeCode, input.PurposeReference, input.CustomerRefDigest,
		input.CurrencyCode, input.AmountMinor, input.BalanceLockReference, input.BalanceLockedMinor,
		input.ModeCode, input.RequestDigest, input.ExpiresAt.UTC(), input.ChannelNo)
	if err != nil {
		return err
	}
	row, err := s.database.GetOne(ctx, `SELECT channel_no, invoice_no, order_no, purpose_code,purpose_reference,currency_code,
amount_minor, balance_locked_minor, mode_code, request_digest FROM plugin_idc_payment_attempt WHERE attempt_no=?`, input.AttemptNo)
	if err != nil {
		return err
	}
	if row.IsEmpty() {
		return errors.New("payment channel is unavailable")
	}
	if !paymentAttemptMatchesCreate(row, input) {
		return errors.New("payment attempt replay changed immutable content")
	}
	return nil
}

func normalizeAndValidateCreateInput(input *CreateInput) error {
	if input.PurposeCode == "" {
		input.PurposeCode = "invoice"
	}
	validPurpose := (input.PurposeCode == "invoice" && input.PurposeReference == "" && input.InvoiceNo != "" && input.OrderNo != "") ||
		((input.PurposeCode == "credit_statement" || input.PurposeCode == "balance_recharge") && input.PurposeReference != "" && input.InvoiceNo == "" && input.OrderNo == "" && input.BalanceLockedMinor == 0)
	if input.AmountMinor <= 0 || input.BalanceLockedMinor < 0 || input.BalanceLockedMinor >= input.AmountMinor ||
		len(input.CustomerRefDigest) != 64 || len(input.RequestDigest) != 64 || input.ExpiresAt.IsZero() || !validPurpose {
		return errors.New("invalid payment attempt")
	}
	return nil
}

func (s *Store) ensureCreateChannel(ctx context.Context, input CreateInput) error {
	channel, err := s.database.GetOne(ctx, `SELECT supported_modes,supported_currencies,capability_snapshot
FROM plugin_idc_payment_channel WHERE channel_no=? AND status_code='enabled' AND deleted_at IS NULL`, input.ChannelNo)
	if err != nil {
		return err
	}
	if channel.IsEmpty() {
		return errors.New("payment channel is unavailable")
	}
	var (
		modes      []string
		currencies []string
		operations map[string]bool
	)
	if json.Unmarshal(channel["supported_modes"].Bytes(), &modes) != nil ||
		json.Unmarshal(channel["supported_currencies"].Bytes(), &currencies) != nil ||
		json.Unmarshal(channel["capability_snapshot"].Bytes(), &operations) != nil ||
		!contains(modes, input.ModeCode) || !contains(currencies, input.CurrencyCode) ||
		!operations[string(paymentcap.OperationCreate)] {
		return errors.New("payment mode or currency is unavailable")
	}
	return nil
}

func paymentAttemptMatchesCreate(row gdb.Record, input CreateInput) bool {
	return row["channel_no"].String() == input.ChannelNo && row["invoice_no"].String() == input.InvoiceNo &&
		row["order_no"].String() == input.OrderNo && row["currency_code"].String() == input.CurrencyCode &&
		row["purpose_code"].String() == input.PurposeCode && row["purpose_reference"].String() == input.PurposeReference &&
		row["amount_minor"].Int64() == input.AmountMinor && row["balance_locked_minor"].Int64() == input.BalanceLockedMinor &&
		row["mode_code"].String() == input.ModeCode && row["request_digest"].String() == input.RequestDigest
}

func (s *Store) RecordVerifiedCallback(ctx context.Context, input CallbackInput) (*CallbackOutcome, error) {
	if input.ChannelNo == "" || len(input.PayloadDigest) != 64 || input.Result.Status != "succeeded" || len(input.Result.EvidenceDigest) != 64 {
		return nil, errors.New("invalid verified payment callback")
	}
	var outcome *CallbackOutcome
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		attempt, err := tx.GetOne(`SELECT attempt_no, channel_no, invoice_no, currency_code, amount_minor, balance_locked_minor,
status_code, expires_at, external_trade_reference FROM plugin_idc_payment_attempt WHERE attempt_no=? FOR UPDATE`, input.Result.AttemptNo)
		if err != nil {
			return err
		}
		if attempt.IsEmpty() {
			return errors.New("payment attempt is not visible")
		}
		receiptNo := stableNo("pcb", input.ChannelNo, input.Result.ProviderEventReference, input.PayloadDigest)
		existing, err := tx.GetOne(`SELECT receipt_no, processing_code FROM plugin_idc_payment_callback_receipt
WHERE channel_no=? AND provider_event_reference=? AND payload_digest=?`, input.ChannelNo, input.Result.ProviderEventReference, input.PayloadDigest)
		if err != nil {
			return err
		}
		if !existing.IsEmpty() {
			outcome = &CallbackOutcome{ReceiptNo: existing["receipt_no"].String(), AttemptNo: input.Result.AttemptNo, Status: existing["processing_code"].String(), Duplicate: true}
			return nil
		}
		expectedChannelMinor := attempt["amount_minor"].Int64() - attempt["balance_locked_minor"].Int64()
		matches := attempt["channel_no"].String() == input.ChannelNo && attempt["currency_code"].String() == input.Result.Currency && expectedChannelMinor == input.Result.AmountMinor
		status := "queued"
		attemptStatus := attempt["status_code"].String()
		if !matches {
			status = "manual_review"
		} else if attemptStatus == "succeeded" {
			status = "duplicate"
		} else if attemptStatus == "closed" || attemptStatus == "expired" || attemptStatus == "failed" {
			status = "late_success"
		}
		_, err = tx.Exec(`INSERT INTO plugin_idc_payment_callback_receipt
 (receipt_no, channel_no, attempt_no, provider_event_reference, payload_digest, signature_valid,
  merchant_match, amount_match, currency_match, processing_code, evidence_digest, processed_at)
VALUES (?, ?, ?, ?, ?, TRUE, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`, receiptNo, input.ChannelNo,
			input.Result.AttemptNo, input.Result.ProviderEventReference, input.PayloadDigest,
			attempt["channel_no"].String() == input.ChannelNo, expectedChannelMinor == input.Result.AmountMinor,
			attempt["currency_code"].String() == input.Result.Currency, status, input.Result.EvidenceDigest)
		if err != nil {
			return err
		}
		switch status {
		case "queued":
			_, err = tx.Exec(`UPDATE plugin_idc_payment_attempt SET status_code='confirming',
external_trade_reference=?, funding_result_digest=?, updated_at=CURRENT_TIMESTAMP, version=version+1
WHERE attempt_no=?`, input.Result.ExternalTradeReference, input.Result.EvidenceDigest, input.Result.AttemptNo)
			if err == nil {
				_, err = tx.Exec(`INSERT INTO plugin_idc_payment_work_item
 (work_no, work_type, subject_no, evidence_digest) VALUES (?, 'confirm_payment', ?, ?)
ON CONFLICT (work_type, subject_no) DO NOTHING`, stableNo("pwk", "confirm_payment", input.Result.AttemptNo), input.Result.AttemptNo, input.Result.EvidenceDigest)
			}
		case "manual_review":
			_, err = tx.Exec(`UPDATE plugin_idc_payment_attempt SET status_code='manual_review', result_unknown=TRUE,
updated_at=CURRENT_TIMESTAMP, version=version+1 WHERE attempt_no=?`, input.Result.AttemptNo)
		case "late_success":
			_, err = tx.Exec(`UPDATE plugin_idc_payment_attempt SET status_code='late_success',
external_trade_reference=?, funding_result_digest=?, confirmed_at=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP,
version=version+1 WHERE attempt_no=?`, input.Result.ExternalTradeReference, input.Result.EvidenceDigest, input.Result.AttemptNo)
			if err == nil {
				_, err = tx.Exec(`INSERT INTO plugin_idc_payment_exception_fund
 (exception_no, attempt_no, channel_no, currency_code, amount_minor, reason_code, evidence_digest)
VALUES (?, ?, ?, ?, ?, 'late_success', ?) ON CONFLICT (attempt_no) DO NOTHING`, stableNo("pex", input.Result.AttemptNo), input.Result.AttemptNo, input.ChannelNo, input.Result.Currency, input.Result.AmountMinor, input.Result.EvidenceDigest)
			}
		}
		if err != nil {
			return err
		}
		outcome = &CallbackOutcome{ReceiptNo: receiptNo, AttemptNo: input.Result.AttemptNo, Status: status}
		return nil
	})
	return outcome, err
}

func (s *Store) ClaimConfirmation(ctx context.Context, lease time.Duration) (*ClaimedWork, error) {
	if lease < time.Second || lease > 10*time.Minute {
		return nil, errors.New("invalid payment work lease")
	}
	token, err := randomToken()
	if err != nil {
		return nil, err
	}
	digest := sha256Hex(token)
	if _, err = s.database.Exec(ctx, `UPDATE plugin_idc_payment_work_item SET status_code='manual_review',lease_token_digest=repeat('0',64),lease_expires_at=NULL,failure_code='CORE_CONFIRMATION_RETRY_EXHAUSTED',finished_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP WHERE work_type='confirm_payment' AND status_code='leased' AND lease_expires_at<=CURRENT_TIMESTAMP AND attempt_count>=max_attempts`); err != nil {
		return nil, err
	}
	row, err := s.database.GetOne(ctx, `WITH candidate AS (
 SELECT id FROM plugin_idc_payment_work_item
 WHERE work_type='confirm_payment' AND attempt_count<max_attempts AND ((status_code='pending' AND available_at<=CURRENT_TIMESTAMP) OR (status_code='leased' AND lease_expires_at<=CURRENT_TIMESTAMP))
 ORDER BY available_at,id FOR UPDATE SKIP LOCKED LIMIT 1
), claimed AS (
 UPDATE plugin_idc_payment_work_item w SET status_code='leased', attempt_count=attempt_count+1,
 lease_token_digest=?, lease_expires_at=CURRENT_TIMESTAMP+(?::bigint*INTERVAL '1 millisecond'), updated_at=CURRENT_TIMESTAMP
 FROM candidate WHERE w.id=candidate.id RETURNING w.work_no,w.subject_no,w.evidence_digest
)
SELECT claimed.work_no,claimed.subject_no AS attempt_no,claimed.evidence_digest,
	attempt.invoice_no,attempt.currency_code,attempt.purpose_code,attempt.purpose_reference,
	(attempt.amount_minor-attempt.balance_locked_minor) AS channel_amount_minor,
	attempt.amount_minor AS total_amount_minor,attempt.balance_locked_minor,
	attempt.balance_lock_reference
FROM claimed JOIN plugin_idc_payment_attempt attempt ON attempt.attempt_no=claimed.subject_no`, digest, lease.Milliseconds())
	if err != nil || row.IsEmpty() {
		return nil, err
	}
	return &ClaimedWork{WorkNo: row["work_no"].String(), AttemptNo: row["attempt_no"].String(), EvidenceDigest: row["evidence_digest"].String(), InvoiceNo: row["invoice_no"].String(), CurrencyCode: row["currency_code"].String(), PurposeCode: row["purpose_code"].String(), PurposeReference: row["purpose_reference"].String(), AmountMinor: row["channel_amount_minor"].Int64(), TotalAmountMinor: row["total_amount_minor"].Int64(), BalanceAmountMinor: row["balance_locked_minor"].Int64(), BalanceHoldNo: row["balance_lock_reference"].String(), LeaseToken: token}, nil
}

func (s *Store) CompleteConfirmation(ctx context.Context, work ClaimedWork) error {
	var callErr error
	if work.PurposeCode == "credit_statement" {
		var credit *billingcap.CreditRepaymentResult
		credit, callErr = s.billing.ConfirmCreditRepayment(ctx, billingcap.ConfirmedCreditRepayment{PaymentAttemptNo: work.AttemptNo, StatementNo: work.PurposeReference, CurrencyCode: work.CurrencyCode, AmountMinor: work.AmountMinor, EvidenceDigest: work.EvidenceDigest})
		if callErr == nil && (credit == nil || credit.PaymentAttemptNo != work.AttemptNo || credit.StatementNo != work.PurposeReference || credit.CurrencyCode != work.CurrencyCode || credit.AmountMinor != work.AmountMinor) {
			callErr = errors.New("core returned an invalid credit repayment result")
		}
	} else if work.PurposeCode == "balance_recharge" {
		var recharge *billingcap.BalanceRechargeResult
		recharge, callErr = s.billing.ConfirmBalanceRecharge(ctx, billingcap.ConfirmedBalanceRecharge{PaymentAttemptNo: work.AttemptNo, RechargeContextNo: work.PurposeReference, CurrencyCode: work.CurrencyCode, AmountMinor: work.AmountMinor, EvidenceDigest: work.EvidenceDigest})
		if callErr == nil && (recharge == nil || recharge.PaymentAttemptNo != work.AttemptNo || recharge.RechargeContextNo != work.PurposeReference || recharge.CurrencyCode != work.CurrencyCode || recharge.AmountMinor != work.AmountMinor) {
			callErr = errors.New("core returned an invalid balance recharge result")
		}
	} else {
		var result *billingcap.ChannelPaymentResult
		result, callErr = s.billing.ConfirmChannelPayment(ctx, billingcap.ConfirmedChannelPayment{PaymentAttemptNo: work.AttemptNo, InvoiceNo: work.InvoiceNo, CurrencyCode: work.CurrencyCode, AmountMinor: work.AmountMinor, TotalAmountMinor: work.TotalAmountMinor, BalanceAmountMinor: work.BalanceAmountMinor, BalanceHoldNo: work.BalanceHoldNo, EvidenceDigest: work.EvidenceDigest})
		if callErr == nil && (result == nil || result.PaymentAttemptNo != work.AttemptNo || result.InvoiceNo != work.InvoiceNo || result.CurrencyCode != work.CurrencyCode || result.AmountMinor != work.AmountMinor || result.TotalAmountMinor != work.TotalAmountMinor || result.BalanceAmountMinor != work.BalanceAmountMinor) {
			callErr = errors.New("core returned an invalid payment result")
		}
	}
	digest := sha256Hex(work.LeaseToken)
	if callErr != nil {
		g.Log().Warningf(ctx, "payment confirmation failed: attempt=%s purpose=%s reference=%s error=%v", work.AttemptNo, work.PurposeCode, work.PurposeReference, callErr)
		_, err := s.database.Exec(ctx, `UPDATE plugin_idc_payment_work_item SET status_code=CASE WHEN attempt_count>=max_attempts THEN 'manual_review' ELSE 'pending' END,
available_at=CURRENT_TIMESTAMP+(LEAST(300,1<<LEAST(attempt_count,8))*INTERVAL '1 second'), lease_token_digest=repeat('0',64), lease_expires_at=NULL,
failure_code='CORE_CONFIRMATION_UNAVAILABLE', updated_at=CURRENT_TIMESTAMP WHERE work_no=? AND status_code='leased' AND lease_token_digest=?`, work.WorkNo, digest)
		if err != nil {
			return err
		}
		return callErr
	}
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		res, err := tx.Exec(`UPDATE plugin_idc_payment_work_item SET status_code='succeeded', lease_token_digest=repeat('0',64), lease_expires_at=NULL,
failure_code='', finished_at=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP WHERE work_no=? AND status_code='leased' AND lease_token_digest=?`, work.WorkNo, digest)
		if err != nil {
			return err
		}
		affected, _ := res.RowsAffected()
		if affected != 1 {
			return errors.New("payment work lease is stale")
		}
		confirmed, err := tx.Exec(`UPDATE plugin_idc_payment_attempt SET status_code='succeeded', result_unknown=FALSE,
confirmed_at=COALESCE(confirmed_at,CURRENT_TIMESTAMP), updated_at=CURRENT_TIMESTAMP, version=version+1 WHERE attempt_no=? AND status_code IN ('confirming','succeeded')`, work.AttemptNo)
		if err != nil {
			return err
		}
		affected, err = confirmed.RowsAffected()
		if err != nil {
			return err
		}
		if affected != 1 {
			return gerror.New("payment confirmation state changed")
		}
		return nil
	})
	return err
}

func stableNo(prefix string, values ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(values, "\x00")))
	return prefix + "_" + hex.EncodeToString(sum[:16])
}
func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func randomToken() (string, error) {
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate lease token: %w", err)
	}
	return hex.EncodeToString(value[:]), nil
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
