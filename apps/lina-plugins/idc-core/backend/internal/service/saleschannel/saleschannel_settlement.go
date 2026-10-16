// This file retains only audited releases of historical settlement freezes.
// New applications, approvals, batches and payments are retired.
package saleschannel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/bizerr"
)

// SettlementReviewRequest records one append-only operator decision.
type SettlementReviewRequest struct {
	SettlementNo, DecisionCode, ReasonCode, EvidenceDigest string
	ReviewerUserID                                         int
}

// PayoutBatchRequest creates a bounded, single-currency payout batch.
type PayoutBatchRequest struct {
	CurrencyCode  string
	SettlementNos []string
	CreatedBy     int
}

type PayoutBatchResult struct {
	BatchNo, CurrencyCode string
	ItemCount             int
	TotalAmountMinor      int64
}

// PayoutResultRequest confirms a historical payout failure and releases its freeze.
type PayoutResultRequest struct {
	BatchNo, SettlementNo, StatusCode, FailureReasonCode string
}

// ReviewSettlement only rejects an unreviewed historical request and releases its freeze.
func (s *serviceImpl) ReviewSettlement(ctx context.Context, request SettlementReviewRequest) (resultErr error) {
	defer func() { resultErr = localizeSettlementReleaseError(resultErr) }()
	if request.DecisionCode != "rejected" {
		return bizerr.NewCode(codeSettlementRetired)
	}
	if err := s.authorizeLegacySettlementRelease(ctx); err != nil {
		return err
	}
	if s.database == nil || ctx == nil || strings.TrimSpace(request.SettlementNo) == "" ||
		strings.TrimSpace(request.ReasonCode) == "" || len(request.EvidenceDigest) != 64 {
		return gerror.New("settlement review request is invalid")
	}
	if s.bizCtx == nil {
		return gerror.New("settlement review business context is unavailable")
	}
	request.ReviewerUserID = s.bizCtx.Current(ctx).UserID
	if request.ReviewerUserID <= 0 {
		return gerror.New("settlement reviewer is unavailable")
	}
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		rows, err := tx.GetAll(`SELECT settlement_no,account_no,currency_code,beneficiary_type_code,beneficiary_reference,requested_amount_minor,frozen_amount_minor,status_code,version FROM plugin_idc_core_sales_commission_settlement WHERE tenant_id=0 AND settlement_no=? FOR UPDATE`, request.SettlementNo)
		if err != nil {
			return err
		}
		if len(rows) != 1 {
			return gerror.New("settlement not found")
		}
		row := rows[0]
		existingReview, err := tx.GetAll(`SELECT decision_code,reason_code,evidence_digest FROM plugin_idc_core_sales_commission_settlement_review WHERE tenant_id=0 AND settlement_no=?`, request.SettlementNo)
		if err != nil {
			return err
		}
		if len(existingReview) > 0 {
			if existingReview[0]["decision_code"].String() == request.DecisionCode && existingReview[0]["reason_code"].String() == request.ReasonCode && existingReview[0]["evidence_digest"].String() == request.EvidenceDigest {
				return nil
			}
			return gerror.New("settlement has already been reviewed")
		}
		status := row["status_code"].String()
		if status != "pending_review" && status != "manual_review" {
			return gerror.New("settlement is not reviewable")
		}
		reviewDigest := request.EvidenceDigest
		reviewNo := "SREV_" + reviewDigest[:30]
		if _, err = tx.Exec(`INSERT INTO plugin_idc_core_sales_commission_settlement_review (tenant_id,review_no,settlement_no,decision_code,reason_code,reviewer_user_id,evidence_digest) VALUES (0,?,?,?,?,?,?)`, reviewNo, request.SettlementNo, request.DecisionCode, request.ReasonCode, request.ReviewerUserID, reviewDigest); err != nil {
			return err
		}

		amount := row["frozen_amount_minor"].Int64()
		accountRows, err := tx.GetAll(`SELECT account_no,currency_code,credited_amount_minor,pending_amount_minor,available_amount_minor,frozen_amount_minor,paid_amount_minor,reversed_amount_minor,version FROM plugin_idc_core_sales_commission_account WHERE tenant_id=0 AND account_no=? AND currency_code=? FOR UPDATE`, row["account_no"].String(), row["currency_code"].String())
		if err != nil {
			return err
		}
		if len(accountRows) != 1 {
			return gerror.New("commission account is missing")
		}
		account := accountRows[0]
		sequence, err := tx.GetValue(`SELECT COALESCE(MAX(entry_sequence),0)+1 FROM plugin_idc_core_sales_commission_account_entry WHERE account_no=?`, account["account_no"].String())
		if err != nil {
			return err
		}
		entryDigest := sha256.Sum256([]byte("settlement-reject:" + request.SettlementNo + ":" + reviewDigest))
		digest := hex.EncodeToString(entryDigest[:])
		if _, err = tx.Exec(`INSERT INTO plugin_idc_core_sales_commission_account_entry (tenant_id,entry_no,account_no,currency_code,entry_sequence,entry_type_code,source_type_code,source_reference,source_digest,delta_credited_minor,delta_pending_minor,delta_available_minor,delta_frozen_minor,delta_paid_minor,delta_reversed_minor,after_credited_minor,after_pending_minor,after_available_minor,after_frozen_minor,after_paid_minor,after_reversed_minor) VALUES (0,?,?,?,?,?,?,?,?,0,0,?, ?,0,0,?,?,?,?,?,?)`, "CENTRY_"+digest[:33], account["account_no"].String(), account["currency_code"].String(), sequence.Int64(), "settlement_release", "commission_settlement", request.SettlementNo, digest, amount, -amount, account["credited_amount_minor"].Int64(), account["pending_amount_minor"].Int64(), account["available_amount_minor"].Int64()+amount, account["frozen_amount_minor"].Int64()-amount, account["paid_amount_minor"].Int64(), account["reversed_amount_minor"].Int64()); err != nil {
			return err
		}
		_, err = tx.Exec(`UPDATE plugin_idc_core_sales_commission_settlement SET status_code='rejected',returned_amount_minor=requested_amount_minor,frozen_amount_minor=0,completed_at=CURRENT_TIMESTAMP,version=version+1 WHERE settlement_no=? AND version=?`, request.SettlementNo, row["version"].Int64())
		return err
	})
	return err
}

// CreatePayoutBatch rejects the retired approval/batch workflow; administrators record offline payments instead.
func (s *serviceImpl) CreatePayoutBatch(context.Context, PayoutBatchRequest) (PayoutBatchResult, error) {
	return PayoutBatchResult{}, bizerr.NewCode(codeSettlementRetired)
}

// RecordPayoutResult records failure of a historical batch item without issuing payments.
func (s *serviceImpl) RecordPayoutResult(ctx context.Context, request PayoutResultRequest) (resultErr error) {
	defer func() { resultErr = localizeSettlementReleaseError(resultErr) }()
	if request.StatusCode != "failed" {
		return bizerr.NewCode(codeSettlementRetired)
	}
	if err := s.authorizeLegacySettlementRelease(ctx); err != nil {
		return err
	}
	if s.database == nil || ctx == nil || strings.TrimSpace(request.BatchNo) == "" || strings.TrimSpace(request.SettlementNo) == "" || strings.TrimSpace(request.FailureReasonCode) == "" || len(request.FailureReasonCode) > 64 {
		return gerror.New("payout result request is invalid")
	}
	return s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		rows, err := tx.GetAll(`SELECT i.item_no,i.amount_minor,i.currency_code,i.status_code,i.failure_reason_code,i.attempt_no,s.account_no,s.requested_amount_minor,s.frozen_amount_minor,s.version FROM plugin_idc_core_sales_commission_payout_item i JOIN plugin_idc_core_sales_commission_settlement s ON s.settlement_no=i.settlement_no WHERE i.batch_no=? AND i.settlement_no=? FOR UPDATE`, request.BatchNo, request.SettlementNo)
		if err != nil {
			return err
		}
		if len(rows) != 1 {
			return gerror.New("payout item not found")
		}
		row := rows[0]
		if row["status_code"].String() == "failed" && row["failure_reason_code"].String() == request.FailureReasonCode {
			return nil
		}
		if row["status_code"].String() == "succeeded" || row["status_code"].String() == "failed" {
			return gerror.New("historical payout already has a terminal result")
		}

		amount := row["amount_minor"].Int64()
		accountRows, err := tx.GetAll(`SELECT account_no,currency_code,credited_amount_minor,pending_amount_minor,available_amount_minor,frozen_amount_minor,paid_amount_minor,reversed_amount_minor,version FROM plugin_idc_core_sales_commission_account WHERE tenant_id=0 AND account_no=? AND currency_code=? FOR UPDATE`, row["account_no"].String(), row["currency_code"].String())
		if err != nil {
			return err
		}
		if len(accountRows) != 1 {
			return gerror.New("commission account is missing")
		}
		account := accountRows[0]
		sequence, err := tx.GetValue(`SELECT COALESCE(MAX(entry_sequence),0)+1 FROM plugin_idc_core_sales_commission_account_entry WHERE account_no=?`, account["account_no"].String())
		if err != nil {
			return err
		}
		if row["frozen_amount_minor"].Int64() != amount || account["frozen_amount_minor"].Int64() < amount {
			return gerror.New("historical payout freeze does not match its item")
		}
		h := sha256.Sum256([]byte(request.BatchNo + ":" + request.SettlementNo + ":" + request.StatusCode))
		digest := hex.EncodeToString(h[:])
		if _, err = tx.Exec(`INSERT INTO plugin_idc_core_sales_commission_payout_evidence (tenant_id,evidence_no,payout_item_no,evidence_type_code,evidence_digest,provider_reference_digest) VALUES (0,?,?,?,?,?) ON CONFLICT (evidence_no) DO NOTHING`, "PEVD_"+digest[:30], row["item_no"].String(), "provider_result", digest, digest); err != nil {
			return err
		}
		if _, err = tx.Exec(`INSERT INTO plugin_idc_core_sales_commission_account_entry (tenant_id,entry_no,account_no,currency_code,entry_sequence,entry_type_code,source_type_code,source_reference,source_digest,delta_credited_minor,delta_pending_minor,delta_available_minor,delta_frozen_minor,delta_paid_minor,delta_reversed_minor,after_credited_minor,after_pending_minor,after_available_minor,after_frozen_minor,after_paid_minor,after_reversed_minor) VALUES (0,?,?,?,?,?,?,?,?,0,0,?,?,?,0,?,?,?,?,?,?)`, "CENTRY_"+digest[:33], account["account_no"].String(), account["currency_code"].String(), sequence.Int64(), "settlement_release", "commission_payout", request.SettlementNo, digest, amount, -amount, int64(0), account["credited_amount_minor"].Int64(), account["pending_amount_minor"].Int64(), account["available_amount_minor"].Int64()+amount, account["frozen_amount_minor"].Int64()-amount, account["paid_amount_minor"].Int64(), account["reversed_amount_minor"].Int64()); err != nil {
			return err
		}
		if _, err = tx.Exec(`UPDATE plugin_idc_core_sales_commission_payout_item SET status_code=?,failure_reason_code=?,processed_at=CURRENT_TIMESTAMP,attempt_no=attempt_no+1 WHERE item_no=?`, request.StatusCode, request.FailureReasonCode, row["item_no"].String()); err != nil {
			return err
		}
		if _, err = tx.Exec(`INSERT INTO plugin_idc_core_sales_commission_payout_recovery (tenant_id,recovery_no,payout_item_no,attempt_no,action_code,result_code,reason_code,evidence_digest) VALUES (0,?,?,?,?,?,?,?) ON CONFLICT (payout_item_no,attempt_no) DO NOTHING`, "PREC_"+digest[:30], row["item_no"].String(), row["attempt_no"].Int()+1, "release", "recovered", request.FailureReasonCode, digest); err != nil {
			return err
		}
		if _, err = tx.Exec(`UPDATE plugin_idc_core_sales_commission_settlement SET status_code='failed',paid_amount_minor=0,returned_amount_minor=?,frozen_amount_minor=0,completed_at=CURRENT_TIMESTAMP,version=version+1 WHERE settlement_no=?`, amount, request.SettlementNo); err != nil {
			return err
		}
		summary, err := tx.GetAll(`SELECT COUNT(*) FILTER (WHERE status_code='succeeded') AS succeeded_count,COALESCE(SUM(amount_minor) FILTER (WHERE status_code='succeeded'),0) AS succeeded_amount,COUNT(*) FILTER (WHERE status_code='failed') AS failed_count,COALESCE(SUM(amount_minor) FILTER (WHERE status_code='failed'),0) AS failed_amount,COUNT(*) FILTER (WHERE status_code IN ('succeeded','failed')) AS terminal_count,COUNT(*) AS total_count FROM plugin_idc_core_sales_commission_payout_item WHERE batch_no=?`, request.BatchNo)
		if err != nil {
			return gerror.Wrap(err, "summarize payout batch")
		}
		if len(summary) != 1 {
			return gerror.New("payout batch summary is unavailable")
		}
		item := summary[0]
		batchStatus := "processing"
		completedAt := "NULL"
		if item["terminal_count"].Int64() == item["total_count"].Int64() {
			completedAt = "CURRENT_TIMESTAMP"
			switch {
			case item["failed_count"].Int64() == 0:
				batchStatus = "succeeded"
			case item["succeeded_count"].Int64() == 0:
				batchStatus = "failed"
			default:
				batchStatus = "partially_failed"
			}
		}
		_, err = tx.Exec(`UPDATE plugin_idc_core_sales_commission_payout_batch SET status_code=?,succeeded_count=?,succeeded_amount_minor=?,failed_count=?,failed_amount_minor=?,completed_at=`+completedAt+` WHERE batch_no=?`, batchStatus, item["succeeded_count"].Int64(), item["succeeded_amount"].Int64(), item["failed_count"].Int64(), item["failed_amount"].Int64(), request.BatchNo)
		return err
	})
}

// authorizeLegacySettlementRelease allows only ordinary staff with the supported global data scope.
// Exact action permission remains enforced by the existing administrator route.
func (s *serviceImpl) authorizeLegacySettlementRelease(ctx context.Context) error {
	if ctx == nil || s.bizCtx == nil {
		return bizerr.NewCode(codeSettlementDenied)
	}
	current := s.bizCtx.Current(ctx)
	if current.UserID <= 0 || current.TenantID != 0 || current.ActingUserID != 0 || current.ActingAsTenant || current.IsImpersonation || current.DataScopeUnsupported || (current.DataScope != 1 && current.DataScope != 2) {
		return bizerr.NewCode(codeSettlementDenied)
	}
	return nil
}

// localizeSettlementReleaseError preserves explicit denials and hides storage details.
func localizeSettlementReleaseError(err error) error {
	if err == nil || bizerr.Is(err, codeSettlementRetired) || bizerr.Is(err, codeSettlementDenied) {
		return err
	}
	return bizerr.WrapCode(err, codeSettlementReleaseFailed)
}
