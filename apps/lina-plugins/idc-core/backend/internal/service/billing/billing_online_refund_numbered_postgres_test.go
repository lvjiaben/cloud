// This file verifies the refund directory SQL against isolated PostgreSQL read
// projections. It does not exercise refund writes or replace migration tests.

package billing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
)

func TestRefundNumberedPagesPostgreSQL(t *testing.T) {
	ctx := context.Background()
	db := refundNumberedDatabase(t, ctx)
	if _, err := db.Exec(ctx, `CREATE TABLE plugin_idc_core_refund_request (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, tenant_id INT NOT NULL DEFAULT 0,
 refund_no TEXT NOT NULL UNIQUE, customer_no TEXT NOT NULL, order_no TEXT NOT NULL DEFAULT 'order',
 invoice_no TEXT NOT NULL DEFAULT 'invoice', currency_code TEXT NOT NULL DEFAULT 'CNY',
 status_code TEXT NOT NULL, reason_code TEXT NOT NULL DEFAULT 'service_issue', reason_detail TEXT NOT NULL DEFAULT '',
 requested_amount_minor BIGINT NOT NULL DEFAULT 100, approval_mode_code TEXT NOT NULL DEFAULT 'manual',
 refund_destination_code TEXT NOT NULL DEFAULT 'original_channel', requires_entitlement_termination BOOLEAN NOT NULL DEFAULT FALSE,
 has_resource_impact BOOLEAN NOT NULL DEFAULT FALSE, has_dangerous_operation BOOLEAN NOT NULL DEFAULT FALSE,
 has_fund_anomaly BOOLEAN NOT NULL DEFAULT FALSE, high_amount BOOLEAN NOT NULL DEFAULT FALSE,
 version BIGINT NOT NULL DEFAULT 1, impact_digest TEXT NOT NULL DEFAULT '', request_digest TEXT NOT NULL DEFAULT '',
 submitted_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, approved_at TIMESTAMPTZ, completed_at TIMESTAMPTZ
);
INSERT INTO plugin_idc_core_refund_request (refund_no,customer_no,status_code)
 SELECT 'refund-page-'||n,'customer-page','pending_review' FROM generate_series(1,25) n;
INSERT INTO plugin_idc_core_refund_request (refund_no,customer_no,status_code)
 SELECT 'refund-other-'||n,'customer-other','pending_review' FROM generate_series(1,7) n;
INSERT INTO plugin_idc_core_refund_request (refund_no,customer_no,status_code)
 SELECT 'refund-rejected-'||n,'customer-page','rejected' FROM generate_series(1,5) n;
INSERT INTO plugin_idc_core_refund_request (tenant_id,refund_no,customer_no,status_code)
 VALUES (9,'refund-outside','customer-page','pending_review');`); err != nil {
		t.Fatal(err)
	}
	for _, size := range []int{10, 20, 50, 100} {
		seen := map[string]bool{}
		for number := 1; number <= (25+size-1)/size+1; number++ {
			page, err := listRefundDirectory(ctx, db, RefundDirectoryFilter{CustomerNo: "customer-page", Status: "pending_review", Page: number, Limit: size})
			if err != nil {
				t.Fatal(err)
			}
			if page.Total != 25 || len(page.Refunds) != min(size, max(25-(number-1)*size, 0)) {
				t.Fatalf("page %d/%d: %+v", number, size, page)
			}
			for _, row := range page.Refunds {
				if seen[row.RefundNo] || row.CustomerNo != "customer-page" {
					t.Fatalf("overlap or scope leak: %+v", row)
				}
				seen[row.RefundNo] = true
			}
		}
		if len(seen) != 25 {
			t.Fatalf("page size %d omitted records: %d", size, len(seen))
		}
	}
	for _, filter := range []RefundDirectoryFilter{{CustomerNo: "customer-other", Page: 1, Limit: 10}, {CustomerNo: "customer-page", Status: "rejected", Page: 1, Limit: 10}, {CustomerNo: "customer-missing", Page: 1, Limit: 10}} {
		page, err := listRefundDirectory(ctx, db, filter)
		want := 0
		if filter.CustomerNo == "customer-other" {
			want = 7
		}
		if filter.Status == "rejected" {
			want = 5
		}
		if err != nil || page.Total != want || len(page.Refunds) != want {
			t.Fatalf("filter %+v: %+v, %v", filter, page, err)
		}
	}
	denied := &managementRefundService{database: db, authorizer: refundNumberedDenied{}}
	if _, err := denied.ListRefunds(ctx, RefundDirectoryFilter{Page: 1, Limit: 10}); err != context.Canceled {
		t.Fatalf("authorization ignored: %v", err)
	}
}

type refundNumberedDenied struct{ ManagementRefundAuthorizer }

func (refundNumberedDenied) AuthorizeRefundList(context.Context) (int, error) {
	return 0, context.Canceled
}

func refundNumberedDatabase(t *testing.T, ctx context.Context) gdb.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(billingIntegrationDSNEnv))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN for refund pagination PostgreSQL test")
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := admin.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	var nonce [8]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	schema := "linapro_refund_page_" + hex.EncodeToString(nonce[:])
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoteBillingIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(ctx, "DROP SCHEMA "+quoteBillingIdentifier(schema)+" CASCADE"); err != nil {
			t.Error(err)
		}
	})
	db, err := gdb.New(gdb.ConfigNode{Link: appendBillingDSNParameter(dsn, "search_path="+url.QueryEscape(schema))})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	return db
}
