// This file verifies bounded numeric credit pagination and filtered totals.

package billing

import (
	"context"
	"math"
	"os"
	"strings"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

func TestCreditPageOffsetRejectsMixedAndUnboundedRequests(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name                string
		page, limit, offset int
		cursor              string
		invalid             bool
	}{
		{name: "cursor mode", limit: 10},
		{name: "first page", page: 1, limit: 10},
		{name: "arbitrary page", page: 7, limit: 20, offset: 120},
		{name: "maximum size", page: 2, limit: 100, offset: 100},
		{name: "negative page", page: -1, limit: 10, invalid: true},
		{name: "zero size", page: 1, invalid: true},
		{name: "oversized", page: 1, limit: 101, invalid: true},
		{name: "mixed cursor", page: 1, limit: 10, cursor: "cursor", invalid: true},
		{name: "overflow", page: math.MaxInt, limit: 100, invalid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			offset, err := creditPageOffset(test.page, test.limit, test.cursor)
			if test.invalid {
				if !bizerr.Is(err, CodeDirectoryInvalid) {
					t.Fatalf("expected invalid request, got %v", err)
				}
				return
			}
			if err != nil || offset != test.offset {
				t.Fatalf("offset=%d err=%v", offset, err)
			}
		})
	}
}

func TestCreditNumericPagesPostgreSQLCountAuthorizedFilteredRows(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	owner := activateCreditAccountForUsageTest(t, ctx, database, "customer-ledger", 8_000, "page-owner")
	foreign := activateCreditAccountForUsageTest(t, ctx, database, "customer-ledger-other", 6_000, "page-foreign")
	seedCreditDirectoryStatement(t, ctx, database, owner.CreditAccountNo, "customer-ledger", "page-owner", 100)
	seedCreditDirectoryStatement(t, ctx, database, foreign.CreditAccountNo, "customer-ledger-other", "page-foreign", 200)
	// Many valid requests exceed the previous first-50 cap without creating accounts.
	_, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_application (
 application_no, customer_no, currency_code, policy_no, policy_version,
 requested_limit_minor, requested_valid_from, requested_valid_until,
 required_approval_count, risk_evidence_reference, risk_evidence_digest,
 request_digest, status_code, request_actor_type_code, requested_by, created_at
) SELECT 'page-application-' || series.n, source.customer_no, source.currency_code,
 source.policy_no, source.policy_version, source.requested_limit_minor,
 source.requested_valid_from, source.requested_valid_until, source.required_approval_count,
 source.risk_evidence_reference, source.risk_evidence_digest,
 md5('page-' || series.n) || md5('page-second-' || series.n), 'pending_review',
 source.request_actor_type_code, source.requested_by, source.created_at + series.n * INTERVAL '1 second'
 FROM (SELECT * FROM plugin_idc_core_credit_application WHERE customer_no = 'customer-ledger' LIMIT 1) source
 CROSS JOIN generate_series(1, 123) series(n)`)
	if err != nil {
		t.Fatal(err)
	}
	key := []byte(strings.Repeat("n", 32))
	management, err := NewManagementCreditReader(database, managementCreditAuthorizerStub{}, key)
	if err != nil {
		t.Fatal(err)
	}
	customer, err := NewAccountCreditReader(database, financialPrincipalStub{audience: activeCreditCustomer(77, "customer-ledger")}, key)
	if err != nil {
		t.Fatal(err)
	}
	applications, err := management.ListManagementCreditApplications(ctx, CreditApplicationDirectoryFilter{Page: 3, Limit: 50})
	if err != nil || applications.Total != 125 || len(applications.Applications) != 25 {
		t.Fatalf("third page=%+v err=%v", applications, err)
	}
	applications, err = management.ListManagementCreditApplications(ctx, CreditApplicationDirectoryFilter{Page: 2, Limit: 100, StatusCode: "pending_review", CurrencyCode: "CNY"})
	if err != nil || applications.Total != 123 || len(applications.Applications) != 23 {
		t.Fatalf("filtered page=%+v err=%v", applications, err)
	}
	applications, err = customer.ListAccountCreditApplications(ctx, CreditApplicationDirectoryFilter{Page: 2, Limit: 100, CustomerNo: "customer-ledger-other"})
	if err != nil || applications.Total != 124 || len(applications.Applications) != 24 {
		t.Fatalf("customer page=%+v err=%v", applications, err)
	}
	for _, item := range applications.Applications {
		if item.CustomerNo != "" {
			t.Fatal("customer projection exposes owner identity")
		}
	}
	applications, err = management.ListManagementCreditApplications(ctx, CreditApplicationDirectoryFilter{Page: 99, Limit: 100})
	if err != nil || applications.Total != 125 || len(applications.Applications) != 0 {
		t.Fatalf("out of range=%+v err=%v", applications, err)
	}
	applications, err = management.ListManagementCreditApplications(ctx, CreditApplicationDirectoryFilter{Page: 1, Limit: 10, CurrencyCode: "USD"})
	if err != nil || applications.Total != 0 || len(applications.Applications) != 0 {
		t.Fatalf("empty filter=%+v err=%v", applications, err)
	}
	for _, page := range []int{1, 2, 3} {
		want := 1
		if page == 3 {
			want = 0
		}
		accounts, accountErr := management.ListManagementCreditAccounts(ctx, CreditAccountDirectoryFilter{Page: page, Limit: 1})
		if accountErr != nil || accounts.Total != 2 || len(accounts.Accounts) != want {
			t.Fatalf("accounts page %d=%+v err=%v", page, accounts, accountErr)
		}
		statements, statementErr := management.ListManagementCreditStatements(ctx, CreditStatementDirectoryFilter{Page: page, Limit: 1})
		if statementErr != nil || statements.Total != 2 || len(statements.Statements) != want {
			t.Fatalf("statements page %d=%+v err=%v", page, statements, statementErr)
		}
	}
	accounts, err := management.ListManagementCreditAccounts(ctx, CreditAccountDirectoryFilter{Page: 1, Limit: 100, CustomerNo: "customer-ledger", StatusCode: "active", CurrencyCode: "CNY"})
	if err != nil || accounts.Total != 1 || len(accounts.Accounts) != 1 {
		t.Fatalf("filtered accounts=%+v err=%v", accounts, err)
	}
	statements, err := customer.ListAccountCreditStatements(ctx, CreditStatementDirectoryFilter{Page: 1, Limit: 100, CustomerNo: "customer-ledger-other", StatusCode: "open", CurrencyCode: "CNY"})
	if err != nil || statements.Total != 1 || len(statements.Statements) != 1 {
		t.Fatalf("customer statements=%+v err=%v", statements, err)
	}
	denied, err := NewManagementCreditReader(database, newManagementCreditAuthorizerForTest(
		t, bizctxcap.CurrentContext{UserID: 81, TenantID: 0, DataScope: 2},
		&principal.Audience{UserID: 81, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive},
		map[authz.PermissionKey]bool{},
	), key)
	if err != nil {
		t.Fatal(err)
	}
	if page, deniedErr := denied.ListManagementCreditApplications(ctx, CreditApplicationDirectoryFilter{Page: 1, Limit: 100}); page != nil || !bizerr.Is(deniedErr, CodeDirectoryNotAccessible) {
		t.Fatalf("unauthorized application total was exposed: page=%+v err=%v", page, deniedErr)
	}
	if page, deniedErr := denied.ListManagementCreditAccounts(ctx, CreditAccountDirectoryFilter{Page: 1, Limit: 100}); page != nil || !bizerr.Is(deniedErr, CodeDirectoryNotAccessible) {
		t.Fatalf("unauthorized account total was exposed: page=%+v err=%v", page, deniedErr)
	}
	if page, deniedErr := denied.ListManagementCreditStatements(ctx, CreditStatementDirectoryFilter{Page: 1, Limit: 100}); page != nil || !bizerr.Is(deniedErr, CodeDirectoryNotAccessible) {
		t.Fatalf("unauthorized statement total was exposed: page=%+v err=%v", page, deniedErr)
	}
	// Reapply the schema-only index migration: no duplicate index or row changes.
	paths := billingServiceMigrationPaths(t)
	for _, path := range paths {
		if !strings.HasSuffix(path, "108-credit-directory-pagination-indexes.sql") {
			continue
		}
		migration, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, execErr := database.Exec(ctx, string(migration)); execErr != nil {
			t.Fatal(execErr)
		}
	}
	indexCount, err := database.GetValue(ctx, `SELECT COUNT(*) FROM pg_indexes WHERE schemaname = current_schema() AND indexname IN (
 'idx_idc_credit_application_page', 'idx_idc_credit_application_customer_page',
 'idx_idc_credit_account_page', 'idx_idc_credit_account_customer_page',
 'idx_idc_credit_statement_page', 'idx_idc_credit_statement_customer_page')`)
	if err != nil || indexCount.Int() != 6 {
		t.Fatalf("pagination indexes=%v err=%v", indexCount, err)
	}
}
