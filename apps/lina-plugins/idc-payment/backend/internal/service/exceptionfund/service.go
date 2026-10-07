// Package exceptionfund owns late-payment disposition without reviving orders.
package exceptionfund

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/billingcap"
	"lina-plugin-idc-payment/backend/cap/paymentcap"
	"lina-plugin-idc-payment/backend/internal/service/channelconfig"
)

type Item struct {
	ExceptionNo, AttemptNo, OrderNo, ChannelNo, ProviderCode          string
	CurrencyCode, ReasonCode, StatusCode, EvidenceDigest              string
	AmountMinor                                                       int64
	CreatedAt                                                         time.Time
	ActionNo, ActionCode, ActionStatus, TargetOrderNo, ProofReference string
	Version                                                           int64
}

// Page contains one numeric exception-fund page and its exact filtered total.
type Page struct {
	Items []*Item
	Total int
}

func (s *Service) RunOneAutomaticRefund(ctx context.Context, channels *channelconfig.Service) (bool, error) {
	if channels == nil {
		return false, gerror.New("exception refund channel service is required")
	}
	work, err := s.ClaimAutomaticRefund(ctx)
	if err != nil || work == nil {
		return false, err
	}
	var providerResult *paymentcap.VerifiedResult
	err = channels.WithCredentials(ctx, work.ChannelNo, func(adapter paymentcap.Adapter, credentials paymentcap.Credentials) error {
		if !adapter.Capabilities().Operations[paymentcap.OperationRefund] {
			return gerror.New("payment refund capability is unavailable")
		}
		var callErr error
		providerResult, callErr = adapter.Refund(ctx, credentials, paymentcap.RefundRequest{RefundAttemptNo: work.RefundAttemptNo, ExternalTradeReference: work.ExternalTradeReference, Currency: work.CurrencyCode, AmountMinor: work.AmountMinor, TotalAmountMinor: work.AmountMinor})
		return callErr
	})
	if err != nil {
		return true, s.FinishAutomaticRefund(ctx, *work, "unknown", "", hash(work.ExceptionNo, "provider_result_unknown"))
	}
	if providerResult == nil || providerResult.AttemptNo != work.RefundAttemptNo || providerResult.Currency != work.CurrencyCode || providerResult.AmountMinor != work.AmountMinor || len(providerResult.EvidenceDigest) != 64 {
		return true, s.FinishAutomaticRefund(ctx, *work, "unknown", "", hash(work.ExceptionNo, "invalid_provider_result"))
	}
	return true, s.FinishAutomaticRefund(ctx, *work, providerResult.Status, providerResult.ExternalTradeReference, providerResult.EvidenceDigest)
}

type Service struct {
	database gdb.DB
	billing  billingcap.Service
}

type AutomaticRefundWork struct {
	ExceptionNo, AttemptNo, RefundAttemptNo, ChannelNo, CurrencyCode, ExternalTradeReference string
	AmountMinor                                                                              int64
}

func New(database gdb.DB, billing billingcap.Service) (*Service, error) {
	if database == nil || billing == nil {
		return nil, gerror.New("exception fund dependencies are required")
	}
	return &Service{database: database, billing: billing}, nil
}

func (s *Service) Allocate(ctx context.Context, exceptionNo, targetOrderNo string, actorID int) (*Item, error) {
	if !validRef(exceptionNo) || !validRef(targetOrderNo) || actorID <= 0 {
		return nil, gerror.New("exception fund allocation request is invalid")
	}
	row, err := s.lockAndCreateAction(ctx, exceptionNo, "allocate", targetOrderNo, actorID)
	if err != nil {
		return nil, err
	}
	allocated, err := s.billing.AllocateExceptionFund(ctx, billingcap.AllocateExceptionFund{
		ExceptionNo: exceptionNo, PaymentAttemptNo: row["attempt_no"].String(),
		TargetOrderNo: targetOrderNo, CustomerRefDigest: row["customer_ref_digest"].String(),
		CurrencyCode: row["currency_code"].String(), AmountMinor: row["amount_minor"].Int64(),
		EvidenceDigest: row["evidence_digest"].String(),
	})
	if err != nil {
		return nil, err
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		updated, updateErr := tx.Ctx(ctx).Exec(`
UPDATE plugin_idc_payment_exception_fund_action
SET status_code='completed',target_invoice_no=?,allocation_no=?,settlement_no=?,completed_by=?,
    completed_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP,version=version+1
WHERE exception_no=? AND action_code='allocate' AND status_code='pending'`,
			allocated.InvoiceNo, allocated.AllocationNo, allocated.SettlementNo, actorID, exceptionNo)
		if updateErr != nil {
			return updateErr
		}
		affected, _ := updated.RowsAffected()
		if affected == 0 {
			existing, loadErr := tx.Ctx(ctx).GetOne(`SELECT status_code,target_order_no,allocation_no FROM plugin_idc_payment_exception_fund_action WHERE exception_no=?`, exceptionNo)
			if loadErr != nil || existing.IsEmpty() || existing["status_code"].String() != "completed" || existing["target_order_no"].String() != targetOrderNo || existing["allocation_no"].String() != allocated.AllocationNo {
				return gerror.New("exception fund allocation state changed")
			}
		}
		_, updateErr = tx.Ctx(ctx).Exec(`UPDATE plugin_idc_payment_exception_fund SET status_code='allocated',resolved_at=COALESCE(resolved_at,CURRENT_TIMESTAMP) WHERE exception_no=? AND status_code='pending'`, exceptionNo)
		return updateErr
	})
	if err != nil {
		return nil, gerror.Wrap(err, "complete exception fund allocation")
	}
	return s.Get(ctx, exceptionNo)
}

func (s *Service) RequestOriginalRefund(ctx context.Context, exceptionNo string, actorID int) (*Item, error) {
	if !validRef(exceptionNo) || actorID <= 0 {
		return nil, gerror.New("exception fund refund request is invalid")
	}
	row, err := s.lockAndCreateAction(ctx, exceptionNo, "original_refund", "", actorID)
	if err != nil {
		return nil, err
	}
	var capabilities map[string]bool
	if json.Unmarshal(row["capability_snapshot"].Bytes(), &capabilities) != nil {
		return nil, gerror.New("payment channel capability snapshot is invalid")
	}
	status := "manual_pending"
	if capabilities["refund"] {
		status = "automatic_pending"
	}
	_, err = s.database.Exec(ctx, `UPDATE plugin_idc_payment_exception_fund_action SET status_code=?,updated_at=CURRENT_TIMESTAMP,version=version+1 WHERE exception_no=? AND action_code='original_refund' AND status_code='pending'`, status, exceptionNo)
	if err != nil {
		return nil, gerror.Wrap(err, "queue exception fund original refund")
	}
	return s.Get(ctx, exceptionNo)
}

func (s *Service) ConfirmManualRefund(ctx context.Context, exceptionNo, proofReference, proofDigest string, actorID int, expectedVersion int64) (*Item, error) {
	proofReference = strings.TrimSpace(proofReference)
	if !validRef(exceptionNo) || !validProofReference(proofReference) || len(proofDigest) != 64 || actorID <= 0 || expectedVersion < 1 {
		return nil, gerror.New("manual original refund confirmation is invalid")
	}
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		updated, updateErr := tx.Ctx(ctx).Exec(`
UPDATE plugin_idc_payment_exception_fund_action
SET status_code='completed',proof_reference=?,proof_digest=?,completed_by=?,completed_at=CURRENT_TIMESTAMP,
    updated_at=CURRENT_TIMESTAMP,version=version+1
WHERE exception_no=? AND action_code='original_refund' AND status_code='manual_pending' AND version=?`, proofReference, proofDigest, actorID, exceptionNo, expectedVersion)
		if updateErr != nil {
			return updateErr
		}
		affected, _ := updated.RowsAffected()
		if affected != 1 {
			return gerror.New("manual original refund state changed")
		}
		_, updateErr = tx.Ctx(ctx).Exec(`UPDATE plugin_idc_payment_exception_fund SET status_code='refunded',resolved_at=CURRENT_TIMESTAMP WHERE exception_no=? AND status_code='pending'`, exceptionNo)
		return updateErr
	})
	if err != nil {
		return nil, gerror.Wrap(err, "confirm manual original refund")
	}
	return s.Get(ctx, exceptionNo)
}

func (s *Service) ClaimAutomaticRefund(ctx context.Context) (*AutomaticRefundWork, error) {
	// A provider call that outlived its process has an unknowable result. Never
	// replay it automatically; expose it for manual reconciliation instead.
	if _, err := s.database.Exec(ctx, `UPDATE plugin_idc_payment_exception_fund_action SET status_code='result_unknown',failure_code='PROVIDER_RESULT_UNKNOWN',updated_at=CURRENT_TIMESTAMP,version=version+1 WHERE status_code='automatic_running' AND updated_at<CURRENT_TIMESTAMP-INTERVAL '2 minutes'`); err != nil {
		return nil, err
	}
	row, err := s.database.GetOne(ctx, `
WITH candidate AS (
 SELECT x.id FROM plugin_idc_payment_exception_fund_action x
 WHERE x.status_code='automatic_pending' ORDER BY x.requested_at,x.id FOR UPDATE SKIP LOCKED LIMIT 1
), claimed AS (
 UPDATE plugin_idc_payment_exception_fund_action x SET status_code='automatic_running',updated_at=CURRENT_TIMESTAMP,version=version+1
 FROM candidate WHERE x.id=candidate.id RETURNING x.exception_no
)
SELECT f.exception_no,f.attempt_no,f.channel_no,f.currency_code,f.amount_minor,a.external_trade_reference
FROM claimed JOIN plugin_idc_payment_exception_fund f ON f.exception_no=claimed.exception_no
JOIN plugin_idc_payment_attempt a ON a.attempt_no=f.attempt_no`)
	if err != nil || row.IsEmpty() {
		return nil, err
	}
	refundNo := "prf_" + hash(row["exception_no"].String(), "original_refund")[:32]
	if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_payment_refund_attempt(refund_attempt_no,refund_request_no,payment_attempt_no,channel_no,currency_code,amount_minor,request_digest,capability_code,status_code) VALUES(?,?,?,?,?,?,?,'automatic','pending') ON CONFLICT(refund_request_no,payment_attempt_no) DO NOTHING`, refundNo, row["exception_no"].String(), row["attempt_no"].String(), row["channel_no"].String(), row["currency_code"].String(), row["amount_minor"].Int64(), hash(row["exception_no"].String(), refundNo)); err != nil {
		return nil, err
	}
	return &AutomaticRefundWork{ExceptionNo: row["exception_no"].String(), AttemptNo: row["attempt_no"].String(), RefundAttemptNo: refundNo, ChannelNo: row["channel_no"].String(), CurrencyCode: row["currency_code"].String(), AmountMinor: row["amount_minor"].Int64(), ExternalTradeReference: row["external_trade_reference"].String()}, nil
}

func (s *Service) FinishAutomaticRefund(ctx context.Context, work AutomaticRefundWork, providerStatus, externalReference, evidenceDigest string) error {
	status, actionStatus, fundStatus := "result_unknown", "result_unknown", "pending"
	switch providerStatus {
	case "succeeded":
		status, actionStatus, fundStatus = "succeeded", "completed", "refunded"
	case "failed", "closed":
		status, actionStatus = "manual_required", "manual_pending"
	case "pending":
		status = "submitted"
	}
	return s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		refundResult, err := tx.Ctx(ctx).Exec(`UPDATE plugin_idc_payment_refund_attempt SET status_code=?,external_refund_reference=?,result_unknown=?,result_code=?,result_evidence_digest=?,confirmed_at=CURRENT_TIMESTAMP,finished_at=CASE WHEN ? IN ('succeeded','manual_required','result_unknown') THEN CURRENT_TIMESTAMP ELSE NULL END,version=version+1 WHERE refund_attempt_no=? AND status_code='pending'`, status, externalReference, status == "result_unknown", "exception_fund_original_refund", evidenceDigest, status, work.RefundAttemptNo)
		if err != nil {
			return err
		}
		if affected, _ := refundResult.RowsAffected(); affected != 1 {
			return gerror.New("automatic refund attempt state changed")
		}
		actionResult, err := tx.Ctx(ctx).Exec(`UPDATE plugin_idc_payment_exception_fund_action SET status_code=?,refund_attempt_no=?,failure_code=CASE WHEN ?='result_unknown' THEN 'PROVIDER_RESULT_UNKNOWN' ELSE '' END,completed_at=CASE WHEN ?='completed' THEN CURRENT_TIMESTAMP ELSE NULL END,updated_at=CURRENT_TIMESTAMP,version=version+1 WHERE exception_no=? AND status_code='automatic_running'`, actionStatus, work.RefundAttemptNo, actionStatus, actionStatus, work.ExceptionNo)
		if err != nil {
			return err
		}
		if affected, _ := actionResult.RowsAffected(); affected != 1 {
			return gerror.New("automatic refund action state changed")
		}
		if fundStatus == "refunded" {
			_, err := tx.Ctx(ctx).Exec(`UPDATE plugin_idc_payment_exception_fund SET status_code='refunded',resolved_at=CURRENT_TIMESTAMP WHERE exception_no=? AND status_code='pending'`, work.ExceptionNo)
			return err
		}
		return nil
	})
}

func (s *Service) Get(ctx context.Context, exceptionNo string) (*Item, error) {
	items, err := s.database.GetAll(ctx, `
SELECT f.exception_no,f.attempt_no,a.order_no,f.channel_no,c.provider_code,f.currency_code,f.amount_minor,
 f.reason_code,f.status_code,f.evidence_digest,f.created_at,COALESCE(x.action_no,'') action_no,
 COALESCE(x.action_code,'') action_code,COALESCE(x.status_code,'') action_status,
 COALESCE(x.target_order_no,'') target_order_no,COALESCE(x.proof_reference,'') proof_reference,
 COALESCE(x.version,0) version
FROM plugin_idc_payment_exception_fund f
JOIN plugin_idc_payment_attempt a ON a.attempt_no=f.attempt_no
JOIN plugin_idc_payment_channel c ON c.channel_no=f.channel_no
LEFT JOIN plugin_idc_payment_exception_fund_action x ON x.exception_no=f.exception_no
WHERE f.exception_no=? LIMIT 1`, exceptionNo)
	if err != nil || len(items) == 0 {
		if err != nil {
			return nil, err
		}
		return nil, gerror.New("exception fund is not visible")
	}
	return mapItem(items[0]), nil
}

func (s *Service) lockAndCreateAction(ctx context.Context, exceptionNo, actionCode, targetOrderNo string, actorID int) (gdb.Record, error) {
	var result gdb.Record
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		row, err := tx.Ctx(ctx).GetOne(`SELECT f.exception_no,f.attempt_no,f.currency_code,f.amount_minor,f.evidence_digest,f.status_code,a.customer_ref_digest,c.capability_snapshot FROM plugin_idc_payment_exception_fund f JOIN plugin_idc_payment_attempt a ON a.attempt_no=f.attempt_no JOIN plugin_idc_payment_channel c ON c.channel_no=f.channel_no WHERE f.exception_no=? FOR UPDATE OF f`, exceptionNo)
		if err != nil {
			return err
		}
		if row.IsEmpty() {
			return gerror.New("exception fund is not visible")
		}
		digest := hash(exceptionNo, actionCode, targetOrderNo)
		actionNo := "pfa_" + digest[:32]
		if row["status_code"].String() == "pending" {
			_, err = tx.Ctx(ctx).Exec(`INSERT INTO plugin_idc_payment_exception_fund_action(action_no,exception_no,action_code,target_order_no,requested_by,request_digest) VALUES(?,?,?,?,?,?) ON CONFLICT(exception_no) DO NOTHING`, actionNo, exceptionNo, actionCode, targetOrderNo, actorID, digest)
			if err != nil {
				return err
			}
		}
		action, err := tx.Ctx(ctx).GetOne(`SELECT action_code,target_order_no,request_digest,status_code FROM plugin_idc_payment_exception_fund_action WHERE exception_no=?`, exceptionNo)
		if err != nil {
			return err
		}
		if action.IsEmpty() || action["action_code"].String() != actionCode || action["target_order_no"].String() != targetOrderNo || action["request_digest"].String() != digest ||
			(row["status_code"].String() != "pending" && action["status_code"].String() != "completed") {
			return gerror.New("exception fund already has a different action")
		}
		result = row
		return nil
	})
	return result, err
}

func mapItem(row gdb.Record) *Item {
	return &Item{ExceptionNo: row["exception_no"].String(), AttemptNo: row["attempt_no"].String(), OrderNo: row["order_no"].String(), ChannelNo: row["channel_no"].String(), ProviderCode: row["provider_code"].String(), CurrencyCode: row["currency_code"].String(), AmountMinor: row["amount_minor"].Int64(), ReasonCode: row["reason_code"].String(), StatusCode: row["status_code"].String(), EvidenceDigest: row["evidence_digest"].String(), CreatedAt: row["created_at"].Time(), ActionNo: row["action_no"].String(), ActionCode: row["action_code"].String(), ActionStatus: row["action_status"].String(), TargetOrderNo: row["target_order_no"].String(), ProofReference: row["proof_reference"].String(), Version: row["version"].Int64()}
}
func validRef(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && len(value) <= 64
}
func validProofReference(value string) bool {
	const prefix = "host-file:"
	if !strings.HasPrefix(value, prefix) {
		return false
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(value, prefix), 10, 64)
	return err == nil && id > 0
}
func hash(values ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(values, "\x00")))
	return hex.EncodeToString(sum[:])
}
