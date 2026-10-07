// This file verifies credit directory ownership, projection, and cursor boundaries in PostgreSQL.

package billing

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

type managementCreditAuthorizerStub struct{}

func (managementCreditAuthorizerStub) AuthorizeApplicationList(context.Context) (int, error) {
	return 9001, nil
}
func (managementCreditAuthorizerStub) AuthorizeApplicationDetail(context.Context, string) (int, error) {
	return 9001, nil
}
func (managementCreditAuthorizerStub) AuthorizeApplicationReview(context.Context, string) (int, error) {
	return 9001, nil
}
func (managementCreditAuthorizerStub) AuthorizeAccountList(context.Context) (int, error) {
	return 9001, nil
}
func (managementCreditAuthorizerStub) AuthorizeAccountDetail(context.Context, string) (int, error) {
	return 9001, nil
}
func (managementCreditAuthorizerStub) AuthorizeStatementList(context.Context) (int, error) {
	return 9001, nil
}
func (managementCreditAuthorizerStub) AuthorizeStatementDetail(context.Context, string) (int, error) {
	return 9001, nil
}
func (managementCreditAuthorizerStub) AuthorizePolicyWrite(context.Context) (int, error) {
	return 9001, nil
}

func TestCreditDirectoriesPostgreSQLKeepCustomerOwnershipAndManagementScope(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	account := activateCreditAccountForUsageTest(
		t, ctx, database, "customer-ledger", 8_000, "directory-owner",
	)
	foreign := activateCreditAccountForUsageTest(
		t, ctx, database, "customer-ledger-other", 6_000, "directory-foreign",
	)
	balance, err := NewBalanceService(database)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = balance.PostConfirmedRecharge(ctx, ConfirmedRechargeInput{
		CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 2_000,
		SourceReference:      "credit-directory-recharge-0001",
		SourceSnapshotDigest: strings.Repeat("7", 64),
	}); err != nil {
		t.Fatalf("seed credit directory balance: %v", err)
	}
	ownerStatement := seedCreditDirectoryStatement(
		t, ctx, database, account.CreditAccountNo, "customer-ledger", "owner", 1_000,
	)
	foreignStatement := seedCreditDirectoryStatement(
		t, ctx, database, foreign.CreditAccountNo, "customer-ledger-other", "foreign", 500,
	)

	key := []byte(strings.Repeat("c", 32))
	accountReader, err := NewAccountCreditReader(database, financialPrincipalStub{audience: activeCreditCustomer(77, "customer-ledger")}, key)
	if err != nil {
		t.Fatal(err)
	}
	overview, err := accountReader.GetAccountCreditOverview(ctx)
	if err != nil || len(overview.Accounts) != 1 || overview.Accounts[0].CustomerNo != "" ||
		overview.Accounts[0].CreditAccountNo != account.CreditAccountNo || overview.OpenStatementCount != 1 {
		t.Fatalf("customer credit overview=%+v err=%v", overview, err)
	}
	applications, err := accountReader.ListAccountCreditApplications(ctx, CreditApplicationDirectoryFilter{})
	if err != nil || len(applications.Applications) != 1 || applications.Applications[0].CustomerNo != "" ||
		applications.Applications[0].CreditAccountNo != account.CreditAccountNo {
		t.Fatalf("customer credit applications=%+v err=%v", applications, err)
	}
	statements, err := accountReader.ListAccountCreditStatements(ctx, CreditStatementDirectoryFilter{})
	if err != nil || len(statements.Statements) != 1 || statements.Statements[0].CustomerNo != "" ||
		statements.Statements[0].StatementNo != ownerStatement ||
		statements.Statements[0].AvailableBalanceMinor != 2_000 || !statements.Statements[0].CanRepayFromBalance {
		t.Fatalf("customer credit statements=%+v err=%v", statements, err)
	}
	detail, err := accountReader.GetAccountCreditStatement(ctx, ownerStatement, CreditStatementLineDirectoryFilter{})
	if err != nil || len(detail.Lines) != 1 || detail.Statement.CustomerNo != "" ||
		detail.Statement.AvailableBalanceMinor != 2_000 || !detail.Statement.CanRepayFromBalance {
		t.Fatalf("customer credit statement detail=%+v err=%v", detail, err)
	}
	if _, err = accountReader.GetAccountCreditStatement(ctx, foreignStatement, CreditStatementLineDirectoryFilter{}); !bizerr.Is(err, CodeDirectoryNotAccessible) {
		t.Fatalf("foreign credit statement existence was exposed: %v", err)
	}

	managementReader, err := NewManagementCreditReader(database, managementCreditAuthorizerStub{}, key)
	if err != nil {
		t.Fatal(err)
	}
	first, err := managementReader.ListManagementCreditApplications(ctx, CreditApplicationDirectoryFilter{Limit: 1})
	if err != nil || len(first.Applications) != 1 || first.NextCursor == "" || first.Applications[0].CustomerNo == "" {
		t.Fatalf("management application page=%+v err=%v", first, err)
	}
	if _, err = managementReader.ListManagementCreditApplications(ctx, CreditApplicationDirectoryFilter{
		Limit: 1, Cursor: first.NextCursor, CustomerNo: "customer-ledger",
	}); !bizerr.Is(err, CodeDirectoryInvalid) {
		t.Fatalf("management cursor crossed its filter boundary: %v", err)
	}
	managementAccount, err := managementReader.GetManagementCreditAccount(ctx, foreign.CreditAccountNo)
	if err != nil || managementAccount.CustomerNo != "customer-ledger-other" {
		t.Fatalf("management credit account=%+v err=%v", managementAccount, err)
	}
	managementStatement, err := managementReader.GetManagementCreditStatement(
		ctx, ownerStatement, CreditStatementLineDirectoryFilter{},
	)
	if err != nil || managementStatement.Statement.CustomerNo != "customer-ledger" ||
		managementStatement.Statement.AvailableBalanceMinor != 0 || managementStatement.Statement.CanRepayFromBalance {
		t.Fatalf("management statement projection=%+v err=%v", managementStatement, err)
	}
}

func seedCreditDirectoryStatement(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	creditAccountNo string,
	customerNo string,
	suffix string,
	amountMinor int64,
) string {
	t.Helper()
	statementNo := "credit-directory-statement-" + suffix
	lineNo := "credit-directory-line-" + suffix
	issuedAt := time.Now().UTC().Add(-48 * time.Hour).Truncate(time.Microsecond)
	periodEnd := issuedAt.Add(-time.Hour)
	periodStart := periodEnd.AddDate(0, -1, 0)
	err := database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, execErr := tx.Exec(`
INSERT INTO plugin_idc_core_credit_statement (
    statement_no, credit_account_no, customer_no, currency_code, terms_no,
    terms_version, period_started_at, period_ended_at, line_count,
    principal_minor, adjustment_minor, total_amount_minor, outstanding_amount_minor,
    status_code, content_digest, issued_at, payment_due_at
) SELECT ?, account.credit_account_no, account.customer_no, account.currency_code,
	         account.current_terms_no, account.current_terms_version, ?, ?, 1,
	         0, ?, ?, ?, 'open', ?, ?, ?
  FROM plugin_idc_core_credit_account account
 WHERE account.tenant_id = 0 AND account.credit_account_no = ? AND account.customer_no = ?`,
			statementNo, periodStart, periodEnd, amountMinor, amountMinor, amountMinor,
			strings.Repeat("8", 64), issuedAt, issuedAt.AddDate(0, 0, 10),
			creditAccountNo, customerNo); execErr != nil {
			return execErr
		}
		_, execErr := tx.Exec(`
INSERT INTO plugin_idc_core_credit_statement_line (
    statement_line_no, statement_no, credit_account_no, customer_no,
	    currency_code, line_number, line_type_code, source_type_code,
    source_reference, amount_minor, content_digest
) VALUES (?, ?, ?, ?, 'CNY', 1, 'adjustment', 'manual', ?, ?, ?)`,
			lineNo, statementNo, creditAccountNo, customerNo, "source-"+suffix,
			amountMinor, strings.Repeat("9", 64))
		return execErr
	})
	if err != nil {
		t.Fatalf("seed credit directory statement: %v", err)
	}
	return statementNo
}

func activeCreditCustomer(userID int, customerNo string) *principal.Audience {
	return &principal.Audience{
		UserID: userID, Code: principal.AudienceCustomer,
		Status: principal.AudienceStatusActive, CustomerNo: customerNo,
	}
}
