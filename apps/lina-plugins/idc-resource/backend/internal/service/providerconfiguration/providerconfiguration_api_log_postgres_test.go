// This file verifies durable body-free logging, rollback isolation and scoped pagination.
package providerconfiguration

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/providerexecution"
	"lina-plugin-idc-resource/backend/internal/service/resourcetask"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type logHost struct {
	pluginhost.BusinessExtensionRegistrar
	evidence *capmodel.BusinessExtensionProviderEvidence
	calls    int
	payload  json.RawMessage
}

func (h *logHost) InvokeProviderMethod(context.Context, pluginhost.BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error) {
	return &capmodel.BusinessExtensionMethodResult{Payload: h.payload}, nil
}

func TestProviderAPILogFlagsFailedAndUnknownBusinessResults(t *testing.T) {
	ctx, db := newProviderConfigurationDatabase(t)
	_, file, _, _ := runtime.Caller(0)
	sql, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../manifest/sql/035-provider-api-log.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, string(sql)); err != nil {
		t.Fatal(err)
	}
	host := &logHost{evidence: &capmodel.BusinessExtensionProviderEvidence{PluginID: "connector", Point: resourcecap.DynamicProviderPoint, ContractVersion: "v1", ReleaseVersion: "v1.0.0", ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("b", 64), AuthorizedMethods: []string{resourcecap.DynamicProviderMethodExecute}}}
	recorder, err := providerexecution.NewRecorded(host, db)
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		status resourcecap.TaskStatus
		code   string
	}{
		{resourcecap.TaskStatusAccepted, ""}, {resourcecap.TaskStatusFailed, "provider_operation_failed"}, {resourcecap.TaskStatusUnknown, "provider_operation_unknown"},
	} {
		result := resourcecap.OperationResult{Status: scenario.status}
		if scenario.status == resourcecap.TaskStatusAccepted {
			result.ExternalTaskRef = "task-71"
		} else {
			result.Failure = &resourcecap.ProviderFailure{Code: "upstream_result_unknown", Message: "Outcome requires review", ResultUnknown: scenario.status == resourcecap.TaskStatusUnknown}
		}
		host.payload, err = json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		input := pluginhost.ConfidentialOutboundContextInput{ProviderPluginID: "connector", Point: resourcecap.DynamicProviderPoint, ContractVersion: "v1", ReleaseVersion: "v1.0.0", ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("b", 64), InterfaceInstanceID: "CHN_log_test", ProviderMethod: resourcecap.DynamicProviderMethodExecute}
		err = recorder.WithConfidentialOutboundContext(ctx, input, func(callCtx context.Context, _ *capmodel.BusinessExtensionRuntimeContext) error {
			_, invokeErr := recorder.InvokeProviderMethod(callCtx, pluginhost.BusinessExtensionMethodInvokeInput{PluginID: input.ProviderPluginID, Point: input.Point, ContractVersion: input.ContractVersion, ReleaseVersion: input.ReleaseVersion, ReleaseDigest: input.ReleaseDigest, DeclarationDigest: input.DeclarationDigest, Method: input.ProviderMethod})
			return invokeErr
		})
		if err != nil {
			t.Fatal(err)
		}
		var row struct {
			Succeeded bool   `orm:"succeeded"`
			Code      string `orm:"error_code"`
		}
		if err = db.GetScan(ctx, &row, `SELECT succeeded,error_code FROM plugin_idc_resource_provider_api_log ORDER BY id DESC LIMIT 1`); err != nil || row.Succeeded != (scenario.code == "") || row.Code != scenario.code {
			t.Fatalf("business result %s was misreported: %+v %v", scenario.status, row, err)
		}
	}
	if host.calls != 3 {
		t.Fatal("logging repeated execution")
	}
}

func (h *logHost) ListProviders(context.Context, plugincap.BusinessExtensionProviderListInput) (*capmodel.PageResult[*capmodel.BusinessExtensionProviderEvidence], error) {
	return &capmodel.PageResult[*capmodel.BusinessExtensionProviderEvidence]{Items: []*capmodel.BusinessExtensionProviderEvidence{h.evidence}}, nil
}
func (h *logHost) WithConfidentialOutboundContext(ctx context.Context, _ pluginhost.ConfidentialOutboundContextInput, action pluginhost.ConfidentialOutboundAction) error {
	h.calls++
	return action(ctx, nil)
}
func TestProviderAPILogSurvivesRollbackAndUsesScopedPages(t *testing.T) {
	ctx, db := newProviderConfigurationDatabase(t)
	_, file, _, _ := runtime.Caller(0)
	sql, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../manifest/sql/035-provider-api-log.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err = db.Exec(ctx, string(sql)); err != nil {
			t.Fatal(err)
		}
	}
	host := &logHost{evidence: &capmodel.BusinessExtensionProviderEvidence{PluginID: "connector", Point: "resource-provider", ContractVersion: "v1", ReleaseVersion: "v1.0.0", ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("b", 64), AuthorizedMethods: []string{"connection.verify"}}}
	recorder, err := providerexecution.NewRecorded(host, db)
	if err != nil {
		t.Fatal(err)
	}
	input := pluginhost.ConfidentialOutboundContextInput{ProviderPluginID: "connector", Point: "resource-provider", ContractVersion: "v1", ReleaseVersion: "v1.0.0", ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("b", 64), InterfaceInstanceID: "CHN_log_test", ProviderMethod: "connection.verify"}
	rollback := errors.New("rollback business, keep log")
	err = db.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		_err := recorder.WithConfidentialOutboundContext(txCtx, input, func(context.Context, *capmodel.BusinessExtensionRuntimeContext) error {
			return errors.New("private-secret-marker")
		})
		if _err == nil {
			return errors.New("failed call lost")
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	if host.calls != 1 {
		t.Fatal("logging repeated provider call")
	}
	value, err := db.GetValue(ctx, `SELECT count(*) FROM plugin_idc_resource_provider_api_log WHERE succeeded=FALSE AND error_code='provider_call_failed'`)
	if err != nil || value.Int() != 1 {
		t.Fatalf("failed history rolled back: %v", err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_resource_provider_api_log(tenant_id,channel_no,provider_plugin_id,method_code,succeeded,duration_ms,started_at) SELECT 0,'CHN_log_test','connector','connection.verify',TRUE,10,CURRENT_TIMESTAMP FROM generate_series(1,30)`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_resource_provider_api_log(tenant_id,channel_no,provider_plugin_id,method_code,succeeded,duration_ms,started_at) VALUES(99,'CHN_hidden','connector','connection.verify',FALSE,10,CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	actor := &directoryAuthorizer{allowed: true}
	store, err := resourcetask.NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	management, err := resourcetask.NewManagement(store, managementcap.Runtime{Authorizer: actor, Audit: providerConfigurationAudit{}})
	if err != nil {
		t.Fatal(err)
	}
	page, err := management.ListAPILogs(ctx, resourcetask.APILogListInput{Page: 2, Limit: 25})
	if err != nil || page.Total != 31 || len(page.Items) != 6 {
		t.Fatalf("numeric page: %#v %v", page, err)
	}
	filtered, err := management.ListAPILogs(ctx, resourcetask.APILogListInput{Page: 1, Limit: 25, Result: "failed"})
	if err != nil || filtered.Total != 1 || len(filtered.Items) != 1 {
		t.Fatalf("result scope: %#v %v", filtered, err)
	}
	if actor.permissions[len(actor.permissions)-1] != "idc:admin:resource:api-log:list" {
		t.Fatal("wrong permission")
	}
	actor.allowed = false
	if _, err = management.ListAPILogs(ctx, resourcetask.APILogListInput{Page: 1, Limit: 25}); err == nil {
		t.Fatal("unauthorized log read")
	}
	actor.allowed = true
	if _, err = management.ListAPILogs(ctx, resourcetask.APILogListInput{Page: 1, Limit: 101}); err == nil {
		t.Fatal("unbounded log page")
	}
}
