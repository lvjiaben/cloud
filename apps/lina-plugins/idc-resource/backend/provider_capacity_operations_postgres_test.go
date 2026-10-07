// This file verifies audited management requests and one-page worker publication.
package backend

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/providercapacity"
	"lina-plugin-idc-resource/backend/internal/service/providerhealth"
)

type providerCapacityManagementAuthorizer struct {
	actor       int
	err         error
	permissions []string
	targets     []string
}

func (a *providerCapacityManagementAuthorizer) Authorize(
	_ context.Context,
	request managementcap.AuthorizationRequest,
) (int, error) {
	a.permissions = append(a.permissions, request.Permission)
	a.targets = append(a.targets, request.TargetNo)
	if a.err != nil {
		return 0, a.err
	}
	return a.actor, nil
}

type providerCapacityAuditWriter struct {
	err         error
	transitions []idccap.BusinessAuditTransition
}

func (w *providerCapacityAuditWriter) AppendBusinessTransition(
	_ context.Context,
	input idccap.BusinessAuditTransition,
) error {
	if w.err != nil {
		return w.err
	}
	w.transitions = append(w.transitions, input)
	return nil
}

func (*providerCapacityAuditWriter) AppendBusinessBatchTransition(
	context.Context,
	idccap.BusinessAuditBatchTransition,
) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, errors.New("unexpected batch audit")
}

type providerCapacityProviderConstructor struct {
	provider resourcecap.Provider
}

func (c *providerCapacityProviderConstructor) Construct(
	context.Context,
	string,
) (resourcecap.Provider, error) {
	return c.provider, nil
}

type providerCapacityPageProvider struct {
	calls int
}

func (*providerCapacityPageProvider) DiscoverCapabilities(context.Context) (*resourcecap.CapabilitySnapshot, error) {
	return nil, errors.New("unexpected capability discovery")
}

func (p *providerCapacityPageProvider) ListProviderCapacity(
	_ context.Context,
	request resourcecap.ProviderCapacityRequest,
) (*resourcecap.ProviderCapacityPage, error) {
	p.calls++
	return &resourcecap.ProviderCapacityPage{Complete: true, Items: []resourcecap.ProviderCapacityItem{{
		Kind: request.Kind, ExternalRef: "stock-operation-1",
		Data: []byte(`{"available":12}`), ObservedAt: time.Now().UTC().Truncate(time.Microsecond),
	}}}, nil
}

func (*providerCapacityPageProvider) Execute(context.Context, resourcecap.OperationRequest) (*resourcecap.OperationResult, error) {
	return nil, errors.New("unexpected execute")
}

func (*providerCapacityPageProvider) QueryTask(context.Context, resourcecap.TaskQuery) (*resourcecap.OperationResult, error) {
	return nil, errors.New("unexpected task query")
}

func (*providerCapacityPageProvider) QueryResource(context.Context, resourcecap.ResourceQuery) (*resourcecap.ResourceSnapshot, error) {
	return nil, errors.New("unexpected resource query")
}

func assertProviderCapacityOperations(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	authorizer := &providerCapacityManagementAuthorizer{actor: 4201}
	audit := &providerCapacityAuditWriter{}
	management, err := providercapacity.NewManagement(database, managementcap.Runtime{
		Authorizer: authorizer, Audit: audit,
	})
	if err != nil {
		t.Fatalf("construct providerCapacity management failed: %v", err)
	}
	created, err := management.RequestSynchronization(
		ctx, "channel-test", resourcecap.ProviderCapacityKindRegion,
	)
	if err != nil || created.Status != providercapacity.StatusCollecting ||
		created.PageSize != providercapacity.AutomaticPageSize || len(audit.transitions) != 0 {
		t.Fatalf("request provider capacity observation failed: created=%+v audit=%+v err=%v", created, audit.transitions, err)
	}
	row, err := database.GetOne(ctx, `
SELECT trigger_code, requested_by, attempt_count, next_attempt_at,
       lease_token_digest, lease_expires_at, failure_code
FROM plugin_idc_resource_provider_capacity_observation
WHERE observation_no = ?`, created.ObservationNo)
	if err != nil || row["trigger_code"].String() != providercapacity.TriggerManual ||
		row["requested_by"].Int() != 4201 || row["attempt_count"].Int() != 0 ||
		row["next_attempt_at"].IsNil() || strings.TrimSpace(row["lease_token_digest"].String()) != "" ||
		!row["lease_expires_at"].IsNil() || row["failure_code"].String() != "" {
		t.Fatalf("manual providerCapacity operation persistence is invalid: row=%v err=%v", row, err)
	}
	page, err := management.ListOperations(ctx, providercapacity.OperationListInput{
		ChannelNo: "channel-test", Kind: resourcecap.ProviderCapacityKindRegion, Limit: 1,
	})
	if err != nil || len(page.Items) != 1 || page.Items[0].ObservationNo != created.ObservationNo ||
		page.Items[0].ChannelName != "Test" || page.Items[0].FailureCode != "" ||
		len(page.Channels) != 2 || page.Channels[0].DisplayName != "Other" ||
		page.Channels[1].DisplayName != "Test" {
		t.Fatalf("list collecting providerCapacity operation failed: page=%+v err=%v", page, err)
	}
	if _, err = management.RequestSynchronization(
		ctx, "channel-test", resourcecap.ProviderCapacityKindRegion,
	); !bizerr.Is(err, providercapacity.CodeProviderCapacityConflict) {
		t.Fatalf("duplicate collecting providerCapacity request returned %v", err)
	}

	provider := &providerCapacityPageProvider{}
	worker, err := providercapacity.NewWorker(
		database, &providerCapacityProviderConstructor{provider: provider}, providerhealth.New(),
	)
	if err != nil {
		t.Fatalf("construct providerCapacity worker failed: %v", err)
	}
	worked, err := worker.RunOne(ctx)
	if err != nil || !worked || provider.calls != 1 {
		t.Fatalf("run one providerCapacity page failed: worked=%v calls=%d err=%v", worked, provider.calls, err)
	}
	healthRows, err := database.GetValue(ctx, `
SELECT count(*)
FROM plugin_idc_resource_health_snapshot
WHERE channel_no = 'channel-test' AND operation_kind = 'provider_capacity'
  AND health_code = 'healthy' AND summary_code = 'succeeded'`)
	if err != nil || healthRows.Int() != 1 {
		t.Fatalf("providerCapacity health observation count=%d want=1 err=%v", healthRows.Int(), err)
	}
	completed, err := management.ListOperations(ctx, providercapacity.OperationListInput{
		ChannelNo: "channel-test", Kind: resourcecap.ProviderCapacityKindRegion,
		Status: providercapacity.StatusComplete,
	})
	if err != nil || len(completed.Items) != 1 || completed.Items[0].ItemCount != 1 ||
		completed.Items[0].PageCount != 1 || completed.Items[0].CompletedAt == nil {
		t.Fatalf("completed providerCapacity operation projection is invalid: page=%+v err=%v", completed, err)
	}
	worked, err = worker.RunOne(ctx)
	if err != nil || worked || provider.calls != 1 {
		t.Fatalf("idle providerCapacity worker performed unexpected work: worked=%v calls=%d err=%v", worked, provider.calls, err)
	}

	denied, err := providercapacity.NewManagement(database, managementcap.Runtime{
		Authorizer: &providerCapacityManagementAuthorizer{actor: 4201, err: errors.New("denied")},
		Audit:      audit,
	})
	if err != nil {
		t.Fatalf("construct denied providerCapacity management failed: %v", err)
	}
	if _, err = denied.ListOperations(ctx, providercapacity.OperationListInput{
		Cursor: "not-a-cursor", Limit: -1,
	}); !bizerr.Is(err, providercapacity.CodeProviderCapacityNotAccessible) {
		t.Fatalf("authorization did not precede malformed list validation: %v", err)
	}
}
