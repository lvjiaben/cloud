// This file proves the read-only owner contract reuses the complete immutable
// financial evidence verifier and does not create a missing settlement.

package billing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
)

func TestSettlementEvidenceReaderRejectsMissingDependenciesAndInput(t *testing.T) {
	if _, err := NewSettlementEvidenceReader(nil); err == nil {
		t.Fatal("nil evidence database accepted")
	}
	reader := &settlementEvidenceReader{}
	if _, err := reader.GetSettledOrderEvidence(context.Background(), " "); err == nil {
		t.Fatal("invalid order accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := reader.GetSettledOrderEvidence(ctx, "ord_1"); err == nil {
		t.Fatal("cancelled request accepted")
	}
}

func TestSettlementEvidenceReaderVerifiesImmutableSourcesPostgreSQL(t *testing.T) {
	ctx := context.Background()
	database := settlementReaderDatabase(t)
	prepared := preparedFinancialSettlement{
		settlementNo: "fst_reader", orderNo: "ord_reader", invoiceNo: "inv_reader", customerNo: "cus_reader",
		currencyCode: "CNY", basis: SettlementBasisFundedInvoice, totalAmountMinor: 100, paidAmountMinor: 100,
		identityEvidenceDigest: strings.Repeat("a", 64), settledAt: time.Date(2026, time.September, 4, 3, 0, 0, 0, time.UTC),
		sources: []preparedFinancialSettlementSource{{sourceType: FinancialSettlementSourceBalance, sourceReference: "payment_reader",
			paymentAllocationNo: "allocation_reader", amountMinor: 100, evidenceDigest: strings.Repeat("b", 64)}},
	}
	if err := finalizePreparedFinancialSettlement(&prepared); err != nil {
		t.Fatal(err)
	}
	header := do.FinancialSettlement{TenantId: 0, SettlementNo: prepared.settlementNo, OrderNo: prepared.orderNo,
		InvoiceNo: prepared.invoiceNo, CustomerNo: prepared.customerNo, CurrencyCode: prepared.currencyCode, BasisCode: prepared.basis,
		TotalAmountMinor: 100, PaidAmountMinor: 100, CreditAmountMinor: 0, SourceCount: 1,
		IdentityEvidenceDigest: prepared.identityEvidenceDigest, SourceSnapshotDigest: prepared.sourceSnapshotDigest,
		ContentDigest: prepared.contentDigest, SettledAt: &prepared.settledAt}
	if _, err := database.Model(dao.FinancialSettlement.Table()).Ctx(ctx).Data(header).Insert(); err != nil {
		t.Fatal(err)
	}
	source := prepared.sources[0]
	if _, err := database.Model(dao.FinancialSettlementSource.Table()).Ctx(ctx).Data(do.FinancialSettlementSource{
		TenantId: 0, SettlementSourceNo: source.settlementSourceNo, SettlementNo: prepared.settlementNo,
		OrderNo: prepared.orderNo, InvoiceNo: prepared.invoiceNo, CustomerNo: prepared.customerNo, CurrencyCode: prepared.currencyCode,
		SourceTypeCode: source.sourceType, SourceReference: source.sourceReference, PaymentAllocationNo: &source.paymentAllocationNo,
		AmountMinor: 100, EvidenceDigest: source.evidenceDigest, ContentDigest: source.contentDigest,
	}).Insert(); err != nil {
		t.Fatal(err)
	}
	reader, err := NewSettlementEvidenceReader(database)
	if err != nil {
		t.Fatal(err)
	}
	first, err := reader.GetSettledOrderEvidence(ctx, prepared.orderNo)
	if err != nil || first == nil || first.TotalAmountMinor != 100 || !first.Reused || len(first.Sources) != 1 {
		t.Fatalf("reader rejected valid owner evidence: %+v %v", first, err)
	}
	if _, err = reader.GetSettledOrderEvidence(ctx, "ord_missing"); err == nil {
		t.Fatal("missing settlement was created or accepted")
	}
	for _, test := range []struct{ name, change, restore string }{
		{"source amount", `UPDATE plugin_idc_core_financial_settlement_source SET amount_minor=99`, `UPDATE plugin_idc_core_financial_settlement_source SET amount_minor=100`},
		{"header amount", `UPDATE plugin_idc_core_financial_settlement SET total_amount_minor=101`, `UPDATE plugin_idc_core_financial_settlement SET total_amount_minor=100`},
		{"missing source", `UPDATE plugin_idc_core_financial_settlement_source SET settlement_no='fst_other'`, `UPDATE plugin_idc_core_financial_settlement_source SET settlement_no='fst_reader'`},
		{"missing event", `UPDATE plugin_idc_core_outbox_event SET correlation_no='fst_other'`, `UPDATE plugin_idc_core_outbox_event SET correlation_no='fst_reader'`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Cleanup(func() {
				if _, restoreErr := database.Exec(ctx, test.restore); restoreErr != nil {
					t.Error(restoreErr)
				}
			})
			if _, changeErr := database.Exec(ctx, test.change); changeErr != nil {
				t.Fatal(changeErr)
			}
			if _, readErr := reader.GetSettledOrderEvidence(ctx, prepared.orderNo); readErr == nil {
				t.Fatal("invalid owner evidence accepted")
			}
		})
	}
	count, countErr := database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_financial_settlement`)
	if countErr != nil || count.Int() != 1 {
		t.Fatal("read created a settlement")
	}
}

func settlementReaderDatabase(t *testing.T) gdb.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN for isolated settlement reader tests")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	schema := "linapro_settlement_reader_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + schema})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Error(closeErr)
		}
		if !strings.HasPrefix(schema, "linapro_settlement_reader_") {
			t.Error("unexpected reader schema")
			return
		}
		if _, dropErr := admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`); dropErr != nil {
			t.Error(dropErr)
		}
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Error(closeErr)
		}
	})
	if _, err = database.Exec(ctx, settlementReaderFixtureSQL); err != nil {
		t.Fatal(err)
	}
	return database
}

const settlementReaderFixtureSQL = `
CREATE TABLE plugin_idc_core_financial_settlement(id BIGINT GENERATED ALWAYS AS IDENTITY,tenant_id INT,settlement_no TEXT,order_no TEXT,invoice_no TEXT,customer_no TEXT,currency_code TEXT,basis_code TEXT,total_amount_minor BIGINT,paid_amount_minor BIGINT,credit_amount_minor BIGINT,source_count INT,identity_evidence_digest TEXT,source_snapshot_digest TEXT,content_digest TEXT,settled_at TIMESTAMPTZ,created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE plugin_idc_core_financial_settlement_source(id BIGINT GENERATED ALWAYS AS IDENTITY,tenant_id INT,settlement_source_no TEXT,settlement_no TEXT,order_no TEXT,invoice_no TEXT,customer_no TEXT,currency_code TEXT,source_type_code TEXT,source_reference TEXT,payment_allocation_no TEXT,credit_exposure_no TEXT,amount_minor BIGINT,evidence_digest TEXT,content_digest TEXT,created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE plugin_idc_core_outbox_event(tenant_id INT,event_type TEXT,aggregate_type TEXT,aggregate_no TEXT,correlation_no TEXT);
INSERT INTO plugin_idc_core_outbox_event VALUES(0,'commerce.order.financially_settled','commerce_order','ord_reader','fst_reader');`
