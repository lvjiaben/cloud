//nolint:cyclop // settlement lifecycle is a single audited transaction state machine.
package saleschannel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
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

// PayoutResultRequest records a provider result. Unknown results never move
// money and remain available for manual investigation.
type PayoutResultRequest struct {
	BatchNo, SettlementNo, StatusCode, FailureReasonCode string
}

func (s *serviceImpl) ReviewSettlement(ctx context.Context, request SettlementReviewRequest) error {
	if s.database == nil || ctx == nil || strings.TrimSpace(request.SettlementNo) == "" ||
		(request.DecisionCode != "approved" && request.DecisionCode != "rejected" && request.DecisionCode != "manual_review") ||
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
		if request.DecisionCode != "rejected" {
			_, err = tx.Exec(`UPDATE plugin_idc_core_sales_commission_settlement SET status_code=?,version=version+1 WHERE settlement_no=? AND version=?`, request.DecisionCode, request.SettlementNo, row["version"].Int64())
			return err
		}
		amount := row["frozen_amount_minor"].Int64()
		accountRows, err := tx.GetAll(`SELECT account_no,credited_amount_minor,pending_amount_minor,available_amount_minor,frozen_amount_minor,paid_amount_minor,reversed_amount_minor,version FROM plugin_idc_core_sales_commission_account WHERE tenant_id=0 AND account_no=? AND currency_code=? FOR UPDATE`, row["account_no"].String(), row["currency_code"].String())
		if err != nil {
			return err
		}
		if len(accountRows) != 1 {
			return gerror.New("commission account is missing")
		}
		account := accountRows[0]
		entryDigest := sha256.Sum256([]byte("settlement-reject:" + request.SettlementNo + ":" + reviewDigest))
		digest := hex.EncodeToString(entryDigest[:])
		if _, err = tx.Exec(`INSERT INTO plugin_idc_core_sales_commission_account_entry (tenant_id,entry_no,account_no,currency_code,entry_sequence,entry_type_code,source_type_code,source_reference,source_digest,delta_credited_minor,delta_pending_minor,delta_available_minor,delta_frozen_minor,delta_paid_minor,delta_reversed_minor,after_credited_minor,after_pending_minor,after_available_minor,after_frozen_minor,after_paid_minor,after_reversed_minor) VALUES (0,?,?,?,?,?,?,?,?,0,0,?, ?,0,0,?,?,?,?,?,?)`, "CENTRY_"+digest[:33], account["account_no"].String(), account["currency_code"].String(), account["version"].Int64()+1, "settlement_release", "commission_settlement", request.SettlementNo, digest, amount, -amount, account["credited_amount_minor"].Int64(), account["pending_amount_minor"].Int64(), account["available_amount_minor"].Int64()+amount, account["frozen_amount_minor"].Int64()-amount, account["paid_amount_minor"].Int64(), account["reversed_amount_minor"].Int64()); err != nil {
			return err
		}
		_, err = tx.Exec(`UPDATE plugin_idc_core_sales_commission_settlement SET status_code='rejected',returned_amount_minor=requested_amount_minor,completed_at=CURRENT_TIMESTAMP,version=version+1 WHERE settlement_no=? AND version=?`, request.SettlementNo, row["version"].Int64())
		return err
	})
	return err
}

func (s *serviceImpl) CreatePayoutBatch(ctx context.Context, request PayoutBatchRequest) (PayoutBatchResult, error) {
	if s.database == nil || ctx == nil || !validCurrencyCode(strings.ToUpper(request.CurrencyCode)) || len(request.SettlementNos) == 0 || len(request.SettlementNos) > 100 {
		return PayoutBatchResult{}, gerror.New("payout batch request is invalid")
	}
	if s.bizCtx == nil {
		return PayoutBatchResult{}, gerror.New("payout batch business context is unavailable")
	}
	request.CreatedBy = s.bizCtx.Current(ctx).UserID
	if request.CreatedBy <= 0 {
		return PayoutBatchResult{}, gerror.New("payout batch creator is unavailable")
	}
	seen := make(map[string]struct{}, len(request.SettlementNos))
	ids := append([]string(nil), request.SettlementNos...)
	sort.Strings(ids)
	for _, id := range ids {
		if strings.TrimSpace(id) == "" {
			return PayoutBatchResult{}, gerror.New("settlement number is invalid")
		}
		if _, ok := seen[id]; ok {
			return PayoutBatchResult{}, gerror.New("duplicate settlement number")
		}
		seen[id] = struct{}{}
	}
	code := strings.ToUpper(request.CurrencyCode)
	h := sha256.Sum256([]byte(code + "\x00" + strings.Join(ids, "\x00")))
	digest := hex.EncodeToString(h[:])
	result := PayoutBatchResult{BatchNo: "PBAT_" + digest[:30], CurrencyCode: code, ItemCount: len(ids)}
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		existing, err := tx.GetAll(`SELECT batch_no,currency_code,item_count,total_amount_minor FROM plugin_idc_core_sales_commission_payout_batch WHERE tenant_id=0 AND batch_no=?`, result.BatchNo)
		if err != nil {
			return err
		}
		if len(existing) == 1 {
			result.ItemCount = existing[0]["item_count"].Int()
			result.TotalAmountMinor = existing[0]["total_amount_minor"].Int64()
			return nil
		}
		amounts := make(map[string]int64, len(ids))
		for _, id := range ids {
			rows, err := tx.GetAll(`SELECT settlement_no,requested_amount_minor,status_code FROM plugin_idc_core_sales_commission_settlement WHERE tenant_id=0 AND settlement_no=? AND currency_code=? FOR UPDATE`, id, code)
			if err != nil {
				return err
			}
			if len(rows) != 1 || (rows[0]["status_code"].String() != "approved" && rows[0]["status_code"].String() != "batched") {
				return gerror.New("settlement is not approved")
			}
			amount := rows[0]["requested_amount_minor"].Int64()
			amounts[id] = amount
			result.TotalAmountMinor += amount
			if rows[0]["status_code"].String() == "approved" {
				if _, err = tx.Exec(`UPDATE plugin_idc_core_sales_commission_settlement SET status_code='batched',version=version+1 WHERE settlement_no=? AND status_code='approved'`, id); err != nil {
					return err
				}
			}
		}
		if _, err := tx.Exec(`INSERT INTO plugin_idc_core_sales_commission_payout_batch (tenant_id,batch_no,currency_code,item_count,total_amount_minor,created_by) VALUES (0,?,?,?,?,?)`, result.BatchNo, code, result.ItemCount, result.TotalAmountMinor, request.CreatedBy); err != nil {
			return err
		}
		for i, id := range ids {
			if _, err := tx.Exec(`INSERT INTO plugin_idc_core_sales_commission_payout_item (tenant_id,item_no,batch_no,settlement_no,currency_code,amount_minor) VALUES (0,?,?,?,?,?)`, fmt.Sprintf("PITEM_%s_%02d", digest[:24], i), result.BatchNo, id, code, amounts[id]); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return PayoutBatchResult{}, gerror.Wrap(err, "create payout batch")
	}
	return result, nil
}

func (s *serviceImpl) RecordPayoutResult(ctx context.Context, request PayoutResultRequest) error {
	if s.database == nil || ctx == nil || strings.TrimSpace(request.BatchNo) == "" || strings.TrimSpace(request.SettlementNo) == "" || (request.StatusCode != "succeeded" && request.StatusCode != "failed" && request.StatusCode != "result_unknown" && request.StatusCode != "manual_review") {
		return gerror.New("payout result request is invalid")
	}
	return s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		rows, err := tx.GetAll(`SELECT i.item_no,i.amount_minor,i.currency_code,i.status_code,i.attempt_no,s.account_no,s.requested_amount_minor,s.frozen_amount_minor,s.version FROM plugin_idc_core_sales_commission_payout_item i JOIN plugin_idc_core_sales_commission_settlement s ON s.settlement_no=i.settlement_no WHERE i.batch_no=? AND i.settlement_no=? FOR UPDATE`, request.BatchNo, request.SettlementNo)
		if err != nil {
			return err
		}
		if len(rows) != 1 {
			return gerror.New("payout item not found")
		}
		row := rows[0]
		if row["status_code"].String() == "succeeded" {
			return nil
		}
		if request.StatusCode == "result_unknown" || request.StatusCode == "manual_review" {
			h := sha256.Sum256([]byte(request.BatchNo + ":" + request.SettlementNo + ":" + request.StatusCode))
			digest := hex.EncodeToString(h[:])
			_, evidenceErr := tx.Exec(`INSERT INTO plugin_idc_core_sales_commission_payout_evidence (tenant_id,evidence_no,payout_item_no,evidence_type_code,evidence_digest,provider_reference_digest) VALUES (0,?,?,?,?,?) ON CONFLICT (evidence_no) DO NOTHING`, "PEVD_"+digest[:30], row["item_no"].String(), "provider_result", digest, digest)
			if evidenceErr != nil {
				return evidenceErr
			}
			_, err = tx.Exec(`UPDATE plugin_idc_core_sales_commission_payout_item SET status_code=?,failure_reason_code=?,attempt_no=attempt_no+1 WHERE item_no=?`, request.StatusCode, "result_unknown", row["item_no"].String())
			if err == nil {
				_, err = tx.Exec(`UPDATE plugin_idc_core_sales_commission_payout_batch SET status_code='manual_review' WHERE batch_no=? AND status_code IN ('pending','processing','manual_review')`, request.BatchNo)
			}
			return err
		}
		amount := row["amount_minor"].Int64()
		accountRows, err := tx.GetAll(`SELECT account_no,credited_amount_minor,pending_amount_minor,available_amount_minor,frozen_amount_minor,paid_amount_minor,reversed_amount_minor,version FROM plugin_idc_core_sales_commission_account WHERE tenant_id=0 AND account_no=? AND currency_code=? FOR UPDATE`, row["account_no"].String(), row["currency_code"].String())
		if err != nil {
			return err
		}
		if len(accountRows) != 1 {
			return gerror.New("commission account is missing")
		}
		account := accountRows[0]
		entryType, deltaFrozen, deltaPaid, deltaAvailable := "payout", -amount, amount, int64(0)
		settlementStatus := "paid"
		returned := int64(0)
		if request.StatusCode == "failed" {
			entryType, deltaFrozen, deltaPaid, deltaAvailable, settlementStatus, returned = "settlement_release", -amount, 0, amount, "failed", amount
		}
		h := sha256.Sum256([]byte(request.BatchNo + ":" + request.SettlementNo + ":" + request.StatusCode))
		digest := hex.EncodeToString(h[:])
		if _, err = tx.Exec(`INSERT INTO plugin_idc_core_sales_commission_payout_evidence (tenant_id,evidence_no,payout_item_no,evidence_type_code,evidence_digest,provider_reference_digest) VALUES (0,?,?,?,?,?) ON CONFLICT (evidence_no) DO NOTHING`, "PEVD_"+digest[:30], row["item_no"].String(), "provider_result", digest, digest); err != nil {
			return err
		}
		if _, err = tx.Exec(`INSERT INTO plugin_idc_core_sales_commission_account_entry (tenant_id,entry_no,account_no,currency_code,entry_sequence,entry_type_code,source_type_code,source_reference,source_digest,delta_credited_minor,delta_pending_minor,delta_available_minor,delta_frozen_minor,delta_paid_minor,delta_reversed_minor,after_credited_minor,after_pending_minor,after_available_minor,after_frozen_minor,after_paid_minor,after_reversed_minor) VALUES (0,?,?,?,?,?,?,?,?,0,0,?,?,?,0,?,?,?,?,?,?)`, "CENTRY_"+digest[:33], account["account_no"].String(), account["currency_code"].String(), account["version"].Int64()+1, entryType, "commission_payout", request.SettlementNo, digest, deltaAvailable, deltaFrozen, deltaPaid, account["credited_amount_minor"].Int64(), account["pending_amount_minor"].Int64(), account["available_amount_minor"].Int64()+deltaAvailable, account["frozen_amount_minor"].Int64()+deltaFrozen, account["paid_amount_minor"].Int64()+deltaPaid, account["reversed_amount_minor"].Int64()); err != nil {
			return err
		}
		if _, err = tx.Exec(`UPDATE plugin_idc_core_sales_commission_payout_item SET status_code=?,failure_reason_code=?,processed_at=CURRENT_TIMESTAMP,attempt_no=attempt_no+1 WHERE item_no=?`, request.StatusCode, request.FailureReasonCode, row["item_no"].String()); err != nil {
			return err
		}
		if request.StatusCode == "failed" {
			if _, err = tx.Exec(`INSERT INTO plugin_idc_core_sales_commission_payout_recovery (tenant_id,recovery_no,payout_item_no,attempt_no,action_code,result_code,reason_code,evidence_digest) VALUES (0,?,?,?,?,?,?,?) ON CONFLICT (payout_item_no,attempt_no) DO NOTHING`, "PREC_"+digest[:30], row["item_no"].String(), row["attempt_no"].Int()+1, "release", "recovered", request.FailureReasonCode, digest); err != nil {
				return err
			}
		}
		if _, err = tx.Exec(`UPDATE plugin_idc_core_sales_commission_settlement SET status_code=?,paid_amount_minor=?,returned_amount_minor=?,completed_at=CURRENT_TIMESTAMP,version=version+1 WHERE settlement_no=?`, settlementStatus, func() int64 {
			if request.StatusCode == "succeeded" {
				return amount
			}
			return 0
		}(), returned, request.SettlementNo); err != nil {
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
