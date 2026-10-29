// The native billing bridge admits only the frozen published product and
// batches provider evidence even when one cart repeats the same configuration.
package productautomation

import (
	"context"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
	"testing"
)

type nativeBillingCandidateStub struct {
	resourceinterfaceprojectioncap.CandidateSource
	calls    int
	revision string
}

func (s *nativeBillingCandidateStub) ReadImportedBillingTerms(_ context.Context, request resourceinterfaceprojectioncap.ImportedBillingRequest) (*channelcommercecap.BillingTerms, error) {
	s.calls++
	return &channelcommercecap.BillingTerms{ConfigurationRevision: request.Input.ConfigurationRevision, CurrencyCode: request.Input.CurrencyCode, Terms: []channelcommercecap.BillingTerm{{BillingTermToken: "term-token", ChargeType: "one_time", PeriodUnit: "once", PeriodCount: 1, MinimumPurchaseCycles: 1, Status: "available", SetupAmount: "0", SubtotalAmount: "731", TaxAmount: "0", TotalAmount: "731"}}}, nil
}
func verifyNativeBillingBatch(t *testing.T, ctx context.Context, db gdb.DB, service *serviceImpl, extensions *automationTestExtensions, version string, snapshot *channelcommercecap.ImportSnapshot) {
	t.Helper()
	source := &nativeBillingCandidateStub{}
	service.resourceCandidates = source
	text := "test-region"
	inputs := []catalog.ImportedBillingInput{{VersionNo: version, CurrencyCode: "USD", Values: []configurationcontract.ConfigurationValue{{FieldKey: "region", TextValue: &text}}}, {VersionNo: version, CurrencyCode: "USD", Values: []configurationcontract.ConfigurationValue{{FieldKey: "region", TextValue: &text}}}}
	before := extensions.exactBatchCalls
	result, err := service.ReadImportedPurchaseBillingTerms(ctx, inputs)
	if err != nil || len(result) != 2 || result[0].Terms[0].TotalAmount != "731" || source.calls != 1 || extensions.exactBatchCalls != before+1 {
		t.Fatalf("native owner batch result=%+v source=%d batch=%d err=%v", result, source.calls, extensions.exactBatchCalls-before, err)
	}
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_catalog_product SET current_version_no='replaced'`); err != nil {
		t.Fatal(err)
	}
	if _, err = service.ReadImportedPurchaseBillingTerms(ctx, inputs); err == nil || source.calls != 1 {
		t.Fatal("replaced product reached native supplier")
	}
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_catalog_product SET current_version_no=?`, version); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_catalog_import_contract SET snapshot_digest=repeat('0',64)`); err != nil {
		t.Fatal(err)
	}
	if _, err = service.ReadImportedPurchaseBillingTerms(ctx, inputs); err == nil || source.calls != 1 {
		t.Fatal("tampered snapshot reached native supplier")
	}
}
