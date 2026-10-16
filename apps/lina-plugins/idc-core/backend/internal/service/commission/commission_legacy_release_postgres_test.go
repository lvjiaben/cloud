// This file proves historical settlement releases are idempotent and preserve
// ledger conservation with the real PostgreSQL constraints and triggers.
package commission

import (
	"context"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/internal/service/saleschannel"
)

type legacyReleaseContext struct{ bizctxcap.Service }

func (legacyReleaseContext) Current(context.Context) bizctxcap.CurrentContext {
	return bizctxcap.CurrentContext{UserID: 81, DataScope: 1}
}

func TestCommissionLegacyReleasePostgreSQL(t *testing.T) {
	database, _, _ := newCommissionDatabase(t)
	ctx := context.Background()
	err := database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		_, err := tx.Exec(`SET LOCAL session_replication_role=replica;
INSERT INTO plugin_idc_core_sales_commission_account(account_no,beneficiary_type_code,beneficiary_reference,currency_code,credited_amount_minor,frozen_amount_minor,version)
VALUES('legacy_account','sales','81','CNY',1000,1000,20);
INSERT INTO plugin_idc_core_sales_commission_settlement(settlement_no,account_no,currency_code,payee_profile_no,beneficiary_type_code,beneficiary_reference,requested_amount_minor,frozen_amount_minor,minimum_amount_minor,status_code,request_digest)
VALUES('legacy_pending','legacy_account','CNY','legacy_payee','sales','81',400,400,0,'pending_review',repeat('1',64)),
('legacy_batched','legacy_account','CNY','legacy_payee','sales','81',600,600,0,'batched',repeat('2',64));
INSERT INTO plugin_idc_core_sales_commission_payout_batch(batch_no,currency_code,item_count,total_amount_minor,created_by)
VALUES('legacy_batch','CNY',1,600,81);
INSERT INTO plugin_idc_core_sales_commission_payout_item(item_no,batch_no,settlement_no,currency_code,amount_minor)
VALUES('legacy_payout_item','legacy_batch','legacy_batched','CNY',600);`)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	service, err := saleschannel.NewWithDatabase(database, legacyReleaseContext{})
	if err != nil {
		t.Fatal(err)
	}
	review := saleschannel.SettlementReviewRequest{SettlementNo: "legacy_pending", DecisionCode: "rejected", ReasonCode: "workflow_retired", EvidenceDigest: strings.Repeat("a", 64)}
	for range 2 {
		if err = service.ReviewSettlement(ctx, review); err != nil {
			t.Fatal(err)
		}
	}
	assertCommissionAmount(t, ctx, database, `SELECT available_amount_minor FROM plugin_idc_core_sales_commission_account`, 400)
	assertCommissionAmount(t, ctx, database, `SELECT frozen_amount_minor FROM plugin_idc_core_sales_commission_account`, 600)
	failure := saleschannel.PayoutResultRequest{BatchNo: "legacy_batch", SettlementNo: "legacy_batched", StatusCode: "failed", FailureReasonCode: "confirmed_not_paid"}
	for range 2 {
		if err = service.RecordPayoutResult(ctx, failure); err != nil {
			t.Fatal(err)
		}
	}
	assertCommissionAmount(t, ctx, database, `SELECT available_amount_minor FROM plugin_idc_core_sales_commission_account`, 1000)
	assertCommissionAmount(t, ctx, database, `SELECT frozen_amount_minor FROM plugin_idc_core_sales_commission_account`, 0)
	assertCommissionAmount(t, ctx, database, `SELECT paid_amount_minor FROM plugin_idc_core_sales_commission_account`, 0)
	assertCommissionAmount(t, ctx, database, `SELECT COUNT(*) FROM plugin_idc_core_sales_commission_account_entry`, 2)
	assertCommissionAmount(t, ctx, database, `SELECT MAX(entry_sequence) FROM plugin_idc_core_sales_commission_account_entry`, 2)
	assertCommissionAmount(t, ctx, database, `SELECT SUM(frozen_amount_minor) FROM plugin_idc_core_sales_commission_settlement`, 0)
	assertCommissionAmount(t, ctx, database, `SELECT COUNT(*) FROM plugin_idc_core_sales_commission_payout_batch WHERE status_code='failed'`, 1)
	failure.FailureReasonCode = "changed_evidence"
	if err = service.RecordPayoutResult(ctx, failure); err == nil {
		t.Fatal("terminal payout accepted a different failure result")
	}
	assertCommissionAmount(t, ctx, database, `SELECT COUNT(*) FROM plugin_idc_core_sales_commission_account_entry`, 2)
}
