// Staff verification must work through persisted payment contexts and funded
// settlement without inventing a supplier identity case or repeating a debit.
package commerce

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	billingservice "lina-plugin-idc-core/backend/internal/service/billing"
)

func TestManualIdentityPaymentAndSettlementPostgreSQL(t *testing.T) {
	for _, kind := range []string{"individual", "enterprise"} {
		t.Run(kind, func(t *testing.T) {
			scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
			const customer = "customer-commerce-service-1"
			table, caseColumn, statusColumn := "plugin_idc_core_customer_individual_profile", "identity_case_no", "identity_status_code"
			if kind == "enterprise" {
				if _, err := scenario.database.Exec(scenario.ctx, `UPDATE plugin_idc_core_customer_subject SET customer_type_code='enterprise',version=version+1 WHERE customer_no=?`, customer); err != nil {
					t.Fatal(err)
				}
				insertVerifiedEnterprisePaymentIdentity(t, scenario, customer)
				table, caseColumn, statusColumn = "plugin_idc_core_customer_enterprise_profile", "verification_case_no", "verification_status_code"
			} else {
				insertVerifiedIndividualPaymentIdentity(t, scenario, customer)
			}
			// Both identifiers come from the fixed cases above, never request input.
			if _, err := scenario.database.Exec(scenario.ctx, "UPDATE "+table+" SET "+caseColumn+"='',manual_verification=TRUE,version=version+1 WHERE customer_no=?", customer); err != nil {
				t.Fatal(err)
			}
			migration, err := os.ReadFile(filepath.Join("../../../../manifest/sql", "135-manual-identity-payment.sql"))
			if err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if _, err = scenario.database.Exec(scenario.ctx, string(migration)); err != nil {
					t.Fatal("migration replay", err)
				}
			}
			scenario.quote, err = scenario.service.QuoteAccountCart(scenario.ctx, QuoteRequest{ExpectedCartVersion: scenario.quote.CartVersion})
			if err != nil {
				t.Fatal(err)
			}
			checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "manual-identity-checkout-0001"})
			if err != nil {
				t.Fatal(err)
			}
			preparation, err := NewPaymentPreparationService(scenario.database, scenario.service.principal)
			if err != nil {
				t.Fatal(err)
			}
			ready, err := preparation.PrepareAccountOrderPayment(scenario.ctx, checkout.OrderNo)
			if err != nil || ready == nil || !ready.Ready {
				t.Fatalf("manual ready=%+v err=%v", ready, err)
			}
			resumed, err := preparation.ResumeAccountOrderPayment(scenario.ctx, ready.ContextNo)
			if err != nil || resumed == nil || !resumed.Ready || resumed.ContextNo != ready.ContextNo {
				t.Fatalf("manual resume=%+v err=%v", resumed, err)
			}
			if _, err = scenario.database.Exec(scenario.ctx, "UPDATE "+table+" SET "+statusColumn+"='unverified',manual_verification=FALSE,version=version+1 WHERE customer_no=?", customer); err != nil {
				t.Fatal(err)
			}
			blocked, err := preparation.ResumeAccountOrderPayment(scenario.ctx, ready.ContextNo)
			if err != nil || blocked == nil || blocked.Ready || blocked.StatusCode != PaymentPreparationAwaitingIdentity {
				t.Fatalf("no case and no staff verification accepted: %+v %v", blocked, err)
			}
			seedFinancialSettlementMerchant(t, scenario)
			balances, err := billingservice.NewBalanceService(scenario.database)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = balances.PostConfirmedRecharge(scenario.ctx, billingservice.ConfirmedRechargeInput{CustomerNo: customer, CurrencyCode: "CNY", AmountMinor: checkout.TotalAmountMinor + 500, SourceReference: "manual-identity-recharge-0001", SourceSnapshotDigest: strings.Repeat("a", 64)}); err != nil {
				t.Fatal(err)
			}
			settlement := newFinancialSettlementTestService(t, scenario)
			commands, err := billingservice.NewAccountFinanceCommandService(scenario.database, scenario.service.principal, settlement)
			if err != nil {
				t.Fatal(err)
			}
			input := billingservice.AccountBalancePaymentInput{InvoiceNo: checkout.InvoiceNo, IdempotencyKey: "manual-identity-balance-0001"}
			if _, err = commands.PayAccountInvoiceFromBalance(scenario.ctx, input); err == nil {
				t.Fatal("unverified customer settled")
			}
			if _, err = scenario.database.Exec(scenario.ctx, "UPDATE "+table+" SET "+statusColumn+"='verified',manual_verification=TRUE,version=version+1 WHERE customer_no=?", customer); err != nil {
				t.Fatal(err)
			}
			paid, err := commands.PayAccountInvoiceFromBalance(scenario.ctx, input)
			if err != nil || paid == nil || !paid.Reused || paid.BalanceMinor != 500 {
				t.Fatalf("manual funded recovery=%+v err=%v", paid, err)
			}
			again, err := commands.PayAccountInvoiceFromBalance(scenario.ctx, input)
			if err != nil || again == nil || again.TransactionNo != paid.TransactionNo || again.BalanceMinor != 500 {
				t.Fatalf("repeat debit: %+v %v", again, err)
			}
			fact, err := settlement.SettleOrder(scenario.ctx, checkout.OrderNo)
			if err != nil {
				t.Fatal(err)
			}
			assertFinancialSettlementState(t, scenario, checkout.OrderNo, fact.SettlementNo, 1)
		})
	}
}
