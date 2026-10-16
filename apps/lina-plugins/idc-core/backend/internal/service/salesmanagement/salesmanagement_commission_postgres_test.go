// These tests run real PostgreSQL ledger triggers in an isolated schema cloned
// from installed table definitions. Every fixture and write is rolled back.
package salesmanagement

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/util/guid"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

func TestSalesManagementCommissionPayoutPostgreSQL(t *testing.T) {
	managementPostgres(t, func(ctx context.Context, db gdb.DB, s *serviceImpl, audit *managementTestAudit) {
		prepareCommissionPayoutTables(t, ctx, db)
		if _, err := db.Exec(ctx, `INSERT INTO plugin_idc_core_sales_commission_account(tenant_id,account_no,beneficiary_type_code,beneficiary_reference,currency_code) VALUES(0,'test_account','affiliate','test_customer','CNY')`); err != nil {
			t.Fatal(err)
		}
		// Use the actual append-only entry trigger to establish earned and matured money.
		if _, err := db.Exec(ctx, `INSERT INTO plugin_idc_core_sales_commission_account_entry(tenant_id,entry_no,account_no,currency_code,entry_sequence,entry_type_code,source_type_code,source_reference,source_digest,delta_credited_minor,delta_pending_minor,delta_available_minor,delta_frozen_minor,delta_paid_minor,delta_reversed_minor,after_credited_minor,after_pending_minor,after_available_minor,after_frozen_minor,after_paid_minor,after_reversed_minor) VALUES(0,'entry_earn','test_account','CNY',1,'accrual','test','earned',repeat('a',64),1000,1000,0,0,0,0,1000,1000,0,0,0,0); INSERT INTO plugin_idc_core_sales_commission_account_entry(tenant_id,entry_no,account_no,currency_code,entry_sequence,entry_type_code,source_type_code,source_reference,source_digest,delta_credited_minor,delta_pending_minor,delta_available_minor,delta_frozen_minor,delta_paid_minor,delta_reversed_minor,after_credited_minor,after_pending_minor,after_available_minor,after_frozen_minor,after_paid_minor,after_reversed_minor) VALUES(0,'entry_mature','test_account','CNY',2,'maturity','test','matured',repeat('b',64),0,-1000,1000,0,0,0,1000,0,1000,0,0,0)`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(ctx, `INSERT INTO plugin_idc_core_commission_accrual(tenant_id,commission_no,order_no,invoice_no,invoice_item_no,beneficiary_type_code,beneficiary_reference,currency_code,rule_version,basis_amount_minor,invoice_item_amount_minor,original_amount_minor,pending_amount_minor,matured_amount_minor,reversed_amount_minor,status_code,matures_at) VALUES(0,'accrual_test','order_test','invoice_test','item_test','affiliate','test_customer','CNY',1,10000,10000,1000,0,1000,0,'matured',CURRENT_TIMESTAMP)`); err != nil {
			t.Fatal(err)
		}
		input := CommissionPayoutInput{AccountNo: "test_account", AmountMinor: "600", EvidenceReference: "Bank transfer 20261010 test", IdempotencyKey: "test-payout-once-0001", PaidAt: time.Now().UTC().Add(-time.Hour), ExpectedVersion: 3}
		first, err := s.RecordCommissionPayout(ctx, input)
		if err != nil {
			t.Fatal(err)
		}
		second, err := s.RecordCommissionPayout(ctx, input)
		if err != nil || first.PayoutNo != second.PayoutNo {
			t.Fatalf("replay changed payout: %+v %v", second, err)
		}
		assertPayoutBalances(t, ctx, db, 400, 600, 1)
		changed := input
		changed.AmountMinor = "601"
		if _, err = s.RecordCommissionPayout(ctx, changed); !bizerr.Is(err, codeConflict) {
			t.Fatalf("changed replay accepted: %v", err)
		}
		failed := input
		failed.IdempotencyKey = "test-payout-audit-0002"
		failed.AmountMinor = "400"
		failed.ExpectedVersion = 4
		audit.fail = true
		if _, err = s.RecordCommissionPayout(ctx, failed); err == nil {
			t.Fatal("audit failure accepted payout")
		}
		audit.fail = false
		assertPayoutBalances(t, ctx, db, 400, 600, 1)
		failed.AmountMinor = "401"
		if _, err = s.RecordCommissionPayout(ctx, failed); !bizerr.Is(err, codeConflict) {
			t.Fatalf("insufficient commission accepted: %v", err)
		}
		denied := s.authorization
		s.authorization = managementTestAuth{allow: false}
		if _, err = s.RecordCommissionPayout(ctx, input); err == nil {
			t.Fatal("denied payout accepted")
		}
		s.authorization = denied
		assertRecoveryBlocksPayout(t, ctx, db, s, input)
		failed.AmountMinor = "400"
		if _, err = s.RecordCommissionPayout(ctx, failed); err != nil {
			t.Fatal(err)
		}
		assertPayoutBalances(t, ctx, db, 0, 1000, 2)
	})
}
func prepareCommissionPayoutTables(t *testing.T, ctx context.Context, db gdb.DB) {
	t.Helper()
	schema := "commission_payout_" + strings.ToLower(guid.S())
	// Identifiers consist solely of a fixed prefix and generated hexadecimal text.
	if _, err := db.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"sales_commission_account", "sales_commission_account_entry", "commission_accrual", "sales_commission_offline_payout", "sales_commission_offline_payout_item", "sales_commission_settlement", "sales_commission_settlement_item", "sales_commission_recovery", "sales_commission_recovery_offset"} {
		name := "plugin_idc_core_" + table
		if _, err := db.Exec(ctx, `CREATE TABLE "`+schema+`"."`+name+`" (LIKE public."`+name+`" INCLUDING ALL)`); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(ctx, `SET LOCAL search_path TO "`+schema+`",public`); err != nil {
		t.Fatal(err)
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate commission migration")
	}
	content, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../manifest/sql/072-sales-channel-foundation.sql"))
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(string(content), "CREATE OR REPLACE FUNCTION plugin_idc_core_sales_apply_commission_entry()")
	// Only ledger projection functions and triggers are needed in this fixture.
	triggerEnd := strings.Index(string(content)[start:], "FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_sales_guard_commission_projection();")
	if start < 0 || triggerEnd < 0 {
		t.Fatal("ledger trigger migration boundary missing")
	}
	sql := string(content)[start : start+triggerEnd+len("FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_sales_guard_commission_projection();")]
	if _, err = db.Exec(ctx, sql); err != nil {
		t.Fatal(err)
	}
}

// assertRecoveryBlocksPayout tests the payout boundary independently of the
// consumer's refund/offset ledger tests; its temporary debt facts are rolled back.
func assertRecoveryBlocksPayout(t *testing.T, ctx context.Context, db gdb.DB, s *serviceImpl, prior CommissionPayoutInput) {
	t.Helper()
	rollback := bizerr.NewCode(codeUnavailable)
	err := db.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, err := db.Exec(ctx, `INSERT INTO plugin_idc_core_sales_commission_recovery(tenant_id,recovery_no,account_no,commission_no,refund_no,reversal_no,amount_minor) VALUES (0,'debt_test','test_account','accrual_test','refund_test','reversal_test',100),(0,'debt_other','other_account','accrual_other','refund_other','reversal_other',900)`); err != nil {
			t.Fatal(err)
		}
		input := prior
		input.IdempotencyKey = "test-payout-debt-0003"
		input.AmountMinor = "100"
		input.ExpectedVersion = 4
		if _, err := s.RecordCommissionPayout(ctx, input); !bizerr.Is(err, codeRecoveryDue) {
			t.Fatalf("outstanding recovery accepted payout: %v", err)
		}
		// Retrying a historical successful request returns its original evidence.
		if _, err := s.RecordCommissionPayout(ctx, prior); err != nil {
			t.Fatalf("recovery blocked historical replay: %v", err)
		}
		for _, amount := range []int{40, 60} {
			if _, err := db.Exec(ctx, `INSERT INTO plugin_idc_core_sales_commission_recovery_offset(tenant_id,offset_no,recovery_no,commission_no,account_no,amount_minor,entry_no) VALUES(0,?,'debt_test','new_accrual','test_account',?,?)`, guid.S(), amount, guid.S()); err != nil {
				t.Fatal(err)
			}
			due, err := s.commissionRecoveryDue(ctx, []string{"test_account"})
			want := int64(0)
			if amount == 40 {
				want = 60
			}
			if err != nil || due["test_account"] != want || len(due) != 1 {
				t.Fatalf("partial/full or cross-account recovery projection: %+v %v", due, err)
			}
			if amount == 40 {
				if _, err := s.RecordCommissionPayout(ctx, input); !bizerr.Is(err, codeRecoveryDue) {
					t.Fatalf("partial recovery accepted payout: %v", err)
				}
			}
		}
		assertPayoutBalances(t, ctx, db, 400, 600, 1)
		return rollback
	})
	if err != rollback {
		t.Fatalf("recovery fixture rollback: %v", err)
	}
}
func assertPayoutBalances(t *testing.T, ctx context.Context, db gdb.DB, available, paid int64, count int) {
	t.Helper()
	var row entity.SalesCommissionAccount
	if err := db.Model("plugin_idc_core_sales_commission_account").Ctx(ctx).Where("account_no", "test_account").Scan(&row); err != nil {
		t.Fatal(err)
	}
	if row.AvailableAmountMinor != available || row.PaidAmountMinor != paid || row.CreditedAmountMinor != 1000 {
		t.Fatalf("incorrect commission balances: %+v", row)
	}
	value, err := db.GetValue(ctx, "SELECT COUNT(*) FROM plugin_idc_core_sales_commission_offline_payout")
	if err != nil || value.Int() != count {
		t.Fatalf("payout count: %v %v", value, err)
	}
	value, err = db.GetValue(ctx, "SELECT COALESCE(SUM(amount_minor),0) FROM plugin_idc_core_sales_commission_offline_payout_item")
	if err != nil || value.Int64() != paid {
		t.Fatalf("payout allocation: %v %v", value, err)
	}
}
