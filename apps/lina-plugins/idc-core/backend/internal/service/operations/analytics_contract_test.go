// File analytics_contract_test.go verifies deterministic risk decisions,
// currency-preserving report aggregation, scope filtering, and export limits.
package operations

import (
	"context"
	"testing"
	"time"
)

func TestEvaluateRiskRulesReturnsAllMatchesInRuleOrder(t *testing.T) {
	limit := int64(100)
	matches, err := EvaluateRiskRules(context.Background(), []RiskRule{
		{RuleNo: "risk-b", EventType: string(RiskEventRefund), MaxEvents: 2, Action: "manual_review"},
		{RuleNo: "risk-a", EventType: string(RiskEventRefund), MaxAmountMinor: &limit, Action: "delay"},
	}, RiskInput{EventType: string(RiskEventRefund), AmountMinor: 101, EventCount: 3})
	if err != nil || len(matches) != 2 {
		t.Fatalf("matches=%+v err=%v", matches, err)
	}
	if matches[0].RuleNo != "risk-a" || matches[0].EvidenceCode != "amount_threshold_exceeded" || matches[1].RuleNo != "risk-b" {
		t.Fatalf("unexpected deterministic matches=%+v", matches)
	}
	if ExplainRiskMatch(matches[0]) == "" {
		t.Fatal("expected operator explanation")
	}
}

func TestEvaluateRiskRulesRejectsUnknownEvent(t *testing.T) {
	if _, err := EvaluateRiskRules(context.Background(), nil, RiskInput{EventType: "unknown", AmountMinor: 1}); err == nil {
		t.Fatal("unknown risk event must fail closed")
	}
}

func TestRiskReviewRequiresReasonAndUsesClosedLifecycle(t *testing.T) {
	if _, err := ValidateRiskReview(RiskReviewInput{CurrentStatus: "open", Decision: "resolve", Reason: "x"}); err == nil {
		t.Fatal("short review reason must fail")
	}
	status, err := ValidateRiskReview(RiskReviewInput{CurrentStatus: "acknowledged", Decision: "dismiss", Reason: "误报，已核对"})
	if err != nil || status != "dismissed" {
		t.Fatalf("status=%q err=%v", status, err)
	}
	if _, err := ValidateRiskReview(RiskReviewInput{CurrentStatus: "resolved", Decision: "dismiss", Reason: "已处理"}); err == nil {
		t.Fatal("terminal risk record must not be reviewed again")
	}
}

func TestAggregateReportKeepsCurrenciesSeparateAndMarksMissingCost(t *testing.T) {
	now := time.Now().UTC()
	rows, err := AggregateReport(context.Background(), []ReportFact{
		{OccurredAt: now.Add(-time.Hour), CurrencyCode: "CNY", RevenueMinor: 1000, PaidMinor: 900, RefundMinor: 100, CostMinor: 300, CostKnown: true, OrderCount: 1},
		{OccurredAt: now.Add(-30 * time.Minute), CurrencyCode: "USD", RevenueMinor: 500, PaidMinor: 500, CostKnown: false, OrderCount: 1},
		{OccurredAt: now.Add(-2 * time.Hour), CurrencyCode: "CNY", RevenueMinor: 300, PaidMinor: 300, CostMinor: 50, CostKnown: true, OrderCount: 1},
	}, now.Add(-2*time.Hour), now, "")
	if err != nil || len(rows) != 2 {
		t.Fatalf("rows=%+v err=%v", rows, err)
	}
	if rows[0].CurrencyCode != "CNY" || rows[0].GrossProfitMinor != 750 || rows[0].OrderCount != 2 {
		t.Fatalf("unexpected CNY row=%+v", rows[0])
	}
	if rows[1].CurrencyCode != "USD" || rows[1].CostKnown || rows[1].GrossProfitMinor != 0 {
		t.Fatalf("missing cost must not become zero cost=%+v", rows[1])
	}
	filtered, err := AggregateReport(context.Background(), []ReportFact{{OccurredAt: now.Add(-time.Hour), CurrencyCode: "CNY", PaidMinor: 1}, {OccurredAt: now.Add(-time.Hour), CurrencyCode: "USD", PaidMinor: 2}}, now.Add(-2*time.Hour), now, "USD")
	if err != nil || len(filtered) != 1 || filtered[0].CurrencyCode != "USD" {
		t.Fatalf("currency filter=%+v err=%v", filtered, err)
	}
}

func TestConvertReportAmountUsesHalfUpAndNeverTouchesLedger(t *testing.T) {
	converted, err := ConvertReportAmount(125, ExchangeRate{FromCurrency: "USD", ToCurrency: "CNY", Numerator: 2, Denominator: 1})
	if err != nil || converted != 250 {
		t.Fatalf("converted=%d err=%v", converted, err)
	}
	converted, err = ConvertReportAmount(1, ExchangeRate{FromCurrency: "USD", ToCurrency: "CNY", Numerator: 1, Denominator: 2})
	if err != nil || converted != 1 {
		t.Fatalf("half-up converted=%d err=%v", converted, err)
	}
	if _, err := ConvertReportAmount(1, ExchangeRate{FromCurrency: "USD", ToCurrency: "CNY", Numerator: 0, Denominator: 1}); err == nil {
		t.Fatal("invalid rate must fail")
	}
}

func TestConvertReportMetricForDisplayPreservesOriginalAndHandlesSignedValues(t *testing.T) {
	original := ReportMetric{
		CurrencyCode:     "USD",
		RevenueMinor:     125,
		ReceivableMinor:  25,
		PaidMinor:        100,
		RefundMinor:      10,
		CashFlowMinor:    -5,
		BalanceMinor:     -7,
		CostMinor:        20,
		CostKnown:        true,
		GrossProfitMinor: 70,
		OrderCount:       3,
	}
	converted, err := ConvertReportMetricForDisplay(context.Background(), original, ExchangeRate{FromCurrency: "USD", ToCurrency: "CNY", Numerator: 2, Denominator: 1})
	if err != nil {
		t.Fatalf("convert metric: %v", err)
	}
	if original.CurrencyCode != "USD" || original.PaidMinor != 100 || original.CashFlowMinor != -5 {
		t.Fatalf("conversion mutated original metric: %+v", original)
	}
	if converted.CurrencyCode != "CNY" || converted.RevenueMinor != 250 || converted.PaidMinor != 200 || converted.RefundMinor != 20 || converted.CashFlowMinor != -10 || converted.BalanceMinor != -14 || converted.GrossProfitMinor != 140 {
		t.Fatalf("unexpected converted metric: %+v", converted)
	}
	if converted.OrderCount != original.OrderCount {
		t.Fatalf("non-monetary counts must remain unchanged: %+v", converted)
	}
}

func TestConvertReportMetricForDisplayFailsClosedOnCurrencyMismatchAndCancellation(t *testing.T) {
	metric := ReportMetric{CurrencyCode: "USD", PaidMinor: 1}
	if _, err := ConvertReportMetricForDisplay(context.Background(), metric, ExchangeRate{FromCurrency: "EUR", ToCurrency: "CNY", Numerator: 1, Denominator: 1}); err == nil {
		t.Fatal("currency mismatch must fail closed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ConvertReportMetricForDisplay(ctx, metric, ExchangeRate{FromCurrency: "USD", ToCurrency: "CNY", Numerator: 1, Denominator: 1}); err == nil {
		t.Fatal("cancelled conversion must stop")
	}
}

func TestFilterReportFactsAndExportPlanAreBounded(t *testing.T) {
	facts, err := FilterReportFacts([]ReportFact{{CustomerNo: "cus-1", CurrencyCode: "CNY"}, {CustomerNo: "cus-2", CurrencyCode: "USD"}}, Scope{CustomerNos: []string{"cus-1"}}, func(f ReportFact) string { return f.CustomerNo })
	if err != nil || len(facts) != 1 || facts[0].CustomerNo != "cus-1" {
		t.Fatalf("scoped facts=%+v err=%v", facts, err)
	}
	if err := ValidateExportPlan(ExportPlan{Format: "csv", MaxRows: 1, PermissionCode: "idc:admin:operations:report:view", Scope: Scope{AllCustomers: true}}); err == nil {
		t.Fatal("CSV export must fail closed")
	}
	if err := ValidateExportPlan(ExportPlan{Format: "xlsx", MaxRows: 100000, PermissionCode: "idc:admin:operations:report:view", Scope: Scope{CustomerNos: []string{"cus-1"}}}); err != nil {
		t.Fatalf("valid bounded export rejected: %v", err)
	}
	predicate, args, err := BuildCustomerScopePredicate("invoice.customer_no", Scope{CustomerNos: []string{"cus-1", "cus-1", "cus-2"}})
	if err != nil || predicate != "invoice.customer_no IN (?,?)" || len(args) != 2 {
		t.Fatalf("parameterized scope predicate=%q args=%v err=%v", predicate, args, err)
	}
	if _, _, err := BuildCustomerScopePredicate("invoice.customer_no;DROP", Scope{AllCustomers: true}); err == nil {
		t.Fatal("untrusted SQL column must fail")
	}
}

func TestAnalyticsContractsHonorCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := EvaluateRiskRules(ctx, nil, RiskInput{EventType: "refund"}); err == nil {
		t.Fatal("cancelled risk evaluation must stop")
	}
	if _, err := AggregateReport(ctx, nil, time.Now().Add(-time.Hour), time.Now(), ""); err == nil {
		t.Fatal("cancelled report aggregation must stop")
	}
}
