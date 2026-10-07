package admin

import "testing"

func TestOperationsAnalyticsCapabilitiesContractIsStable(t *testing.T) {
	t.Parallel()
	controller := &ControllerV1{}
	res, err := controller.OperationsAnalyticsCapabilitiesGet(t.Context(), nil)
	if err != nil {
		t.Fatalf("capabilities request failed: %v", err)
	}
	if len(res.RiskEvents) != 8 || len(res.RiskActions) != 5 {
		t.Fatalf("unexpected risk contract sizes: events=%d actions=%d", len(res.RiskEvents), len(res.RiskActions))
	}
	if res.ManualReview == nil || !res.ManualReview.ReasonRequired {
		t.Fatal("manual review must require a reason")
	}
	if res.Report == nil || !res.Report.OriginalCurrencyGrouped || !res.Report.ExchangeRateOptional || !res.Report.ConvertedValueLedgerSafe {
		t.Fatal("report currency safety contract is incomplete")
	}
	if res.AuditExport == nil || res.AuditExport.Format != "xlsx" || !res.AuditExport.Asynchronous || !res.AuditExport.ShortLivedLink || !res.AuditExport.RechecksScope {
		t.Fatal("audit export contract is incomplete")
	}
}
