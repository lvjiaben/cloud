// This file proves immutable paid terms, corrected recovery commands, replay and
// actual resource-owner successor creation using isolated schemas and no HTTP.
package commerce

import (
	"context"
	"errors"
	"strings"
	"testing"

	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	productfulfillmentservice "lina-plugin-idc-core/backend/internal/service/productfulfillment"
)

// correctionSource keeps candidate reads isolated while delegating actual task
// creation/read/outcome handling to the real resource owner used by the chain.
type correctionSource struct {
	provisionresourcecap.Source
	unavailable bool
	validations int
	recovered   []provisionresourcecap.TaskReadRequest
}

func (s *correctionSource) InspectRecovery(_ context.Context, input provisionresourcecap.RecoverySelectionRequest) (*provisionresourcecap.RecoveryEditor, error) {
	selection := provisionresourcecap.RecoveryConfiguration{InterfaceInstanceNo: input.Request.InterfaceInstanceNo, InterfaceConfigurationRevision: input.Request.InterfaceConfigurationRevision, Region: "1", Line: "5", OperatingSystem: "98", InterfaceLabel: "Test upstream", RegionLabel: "Test region", LineLabel: "Corrected node", OperatingSystemLabel: "Corrected image"}
	if input.Selection != nil {
		selection = *input.Selection
	}
	return &provisionresourcecap.RecoveryEditor{Selection: selection}, nil
}
func (s *correctionSource) ValidateRecovery(ctx context.Context, input provisionresourcecap.RecoverySelectionRequest) (*provisionresourcecap.RecoveryConfiguration, error) {
	s.validations++
	if s.unavailable || input.Selection == nil || input.Selection.Line == "invalid" {
		return nil, errors.New("isolated candidate rejected")
	}
	editor, err := s.InspectRecovery(ctx, input)
	return &editor.Selection, err
}
func (s *correctionSource) RecoverRejected(ctx context.Context, input provisionresourcecap.TaskReadRequest) (*provisionresourcecap.Task, error) {
	s.recovered = append(s.recovered, input)
	return s.Source.RecoverRejected(ctx, input)
}

func TestPaidFulfillmentOperationalCorrectionPostgreSQL(t *testing.T) {
	prepared := prepareCrossPluginServerScenario(t)
	scenario := prepared.checkout
	ctx, db := scenario.ctx, scenario.database
	configureResourceChainDatabase(t, ctx, db)
	extensions, job, source := crossPluginRuntime(t, ctx)
	extensions.rejectFirst = true
	correctedSource := &correctionSource{Source: source}
	sealer, err := productfulfillmentservice.NewProviderEvidenceSealer("test-key-v1", []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	worker, err := productfulfillmentservice.NewWorker(db, extensions, &fulfillmentWorkerContexts{}, sealer, fulfillmentSystemReasons(t), correctedSource, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	service, err := productfulfillmentservice.NewFullConfigured(db, structuredFulfillmentEvidenceValidator{}, fulfillmentGateStub{}, fulfillmentSystemReasons(t), fulfillmentAuthorizerStub{}, fulfillmentAuditStub{}, fulfillmentActorsStub{}, []byte("0123456789abcdef0123456789abcdef"), correctedSource)
	if err != nil {
		t.Fatal(err)
	}
	run := func() {
		t.Helper()
		advanceFulfillmentTaskClock(t, scenario, prepared.order.OrderNo)
		if _, runErr := worker.Run(ctx, 1); runErr != nil {
			t.Fatal(runErr)
		}
	}
	run()
	run()
	if err = job(ctx); err != nil {
		t.Fatal(err)
	}
	run()
	intent, err := db.GetValue(ctx, `SELECT intent_no FROM plugin_idc_core_product_fulfillment_intent WHERE order_no=?`, prepared.order.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	detail, err := service.Detail(ctx, intent.String())
	if err != nil || !detail.CanRecoverPreparation {
		t.Fatalf("failure not recoverable: %+v %v", detail, err)
	}
	if detail.Execution == nil || !strings.Contains(detail.Execution.ResponseExcerpt, "镜像在当前节点未下载") {
		t.Fatal("resource response did not survive persistence and core execution history")
	}
	outcomes, err := productfulfillmentservice.NewResourceOutcomeWorker(db, correctedSource)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = outcomes.Run(ctx, 1); err != nil {
		t.Fatal(err)
	}
	editor, err := service.InspectRecoveryConfiguration(ctx, intent.String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	originalDigest, err := db.GetValue(ctx, `SELECT content_digest FROM plugin_idc_core_commerce_structured_configuration_snapshot WHERE order_no=?`, prepared.order.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	input := productfulfillmentservice.ActionInput{IntentNo: intent.String(), ExpectedVersion: detail.Version, IdempotencyKey: "corrected-recovery-0001", ReasonCode: "preparation_resolved", Note: "Correct operational mapping without changing paid specifications", RecoveryConfiguration: &editor.Selection}
	invalid := editor.Selection
	invalid.Line = "invalid"
	input.RecoveryConfiguration = &invalid
	if _, err = service.RecoverPreparation(ctx, input); err == nil {
		t.Fatal("invalid operational selection accepted")
	}
	input.RecoveryConfiguration = &editor.Selection
	recovered, err := service.RecoverPreparation(ctx, input)
	if err != nil || recovered.StatusCode != "processing" || len(recovered.RecoveryConfigurations) != 1 || recovered.RecoveryConfigurations[0].Configuration.OperatingSystem != "98" {
		t.Fatalf("corrected command not retained: %+v %v", recovered, err)
	}
	validations := correctedSource.validations
	correctedSource.unavailable = true
	replay, err := service.RecoverPreparation(ctx, input)
	if err != nil || replay.Version != recovered.Version || correctedSource.validations != validations {
		t.Fatalf("immutable replay depended on new candidate reads: %+v %v", replay, err)
	}
	run()
	run()
	if err = job(ctx); err != nil {
		t.Fatal(err)
	}
	run()
	completed, err := service.Detail(ctx, intent.String())
	if err != nil || completed.StatusCode != "completed" {
		t.Fatalf("corrected delivery did not complete: %+v %v", completed, err)
	}
	if len(correctedSource.recovered) != 1 || correctedSource.recovered[0].RecoveryConfiguration == nil || correctedSource.recovered[0].RecoveryConfiguration.OperatingSystem != "98" {
		t.Fatalf("correction lost before resource owner: %+v", correctedSource.recovered)
	}
	var proof struct {
		Commands, Settlements, Tasks, Resources int
		ContentDigest                           string
	}
	err = db.GetScan(ctx, &proof, `SELECT
 (SELECT count(*) FROM plugin_idc_core_product_fulfillment_command WHERE intent_no=?) AS commands,
 (SELECT count(*) FROM plugin_idc_core_financial_settlement WHERE order_no=?) AS settlements,
 (SELECT count(*) FROM plugin_idc_resource_task) AS tasks,
 (SELECT count(DISTINCT local_resource_no) FROM plugin_idc_resource_task) AS resources,
 (SELECT content_digest FROM plugin_idc_core_commerce_structured_configuration_snapshot WHERE order_no=?) AS content_digest`, intent.String(), prepared.order.OrderNo, prepared.order.OrderNo)
	if err != nil || proof.Commands != 1 || proof.Settlements != 1 || proof.Tasks != 2 || proof.Resources != 1 || proof.ContentDigest != originalDigest.String() {
		t.Fatalf("paid terms or task identity changed: %+v %v", proof, err)
	}
}
