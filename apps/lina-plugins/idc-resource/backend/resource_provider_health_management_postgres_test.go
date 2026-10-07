// This file verifies the bounded Provider health administration contract against PostgreSQL.
package backend

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-resource/backend/internal/service/providerhealth"
)

func assertResourceHealthManagement(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	authorizer := &providerCapacityManagementAuthorizer{actor: 5101}
	audit := &providerCapacityAuditWriter{}
	management, err := providerhealth.NewManagement(database, managementcap.Runtime{
		Authorizer: authorizer, Audit: audit,
	})
	if err != nil {
		t.Fatalf("construct Provider health management failed: %v", err)
	}
	page, err := management.List(ctx, providerhealth.HealthListInput{
		ChannelNo: "channel-test", Limit: 1,
	})
	if err != nil || len(page.Items) != 1 || page.Items[0].ChannelName != "Test" ||
		page.Items[0].ProviderID != "provider-test" || page.Items[0].Policy.Version != 1 ||
		page.Items[0].Authentication.HealthCode != "degraded" ||
		page.Items[0].Query.HealthCode != "degraded" || page.NextCursor != "" {
		t.Fatalf("list Provider health projection failed: page=%+v err=%v", page, err)
	}
	updated, err := management.UpdatePolicy(ctx, providerhealth.HealthPolicyUpdateInput{
		ChannelNo: "channel-test", WindowSampleLimit: 10,
		MinimumSamplesToProtect: 3, FailureLimit: 2, UnknownResultLimit: 1,
		ConsecutiveFailureLimit: 2, ProtectionDurationSeconds: 600, ExpectedVersion: 1,
	})
	if err != nil || updated.Policy.Version != 2 || updated.Policy.WindowSampleLimit != 10 ||
		len(audit.transitions) != 1 || audit.transitions[0].RequiredPermission != "idc:admin:resource:health-policy:write" {
		t.Fatalf("update Provider health policy failed: item=%+v audit=%+v err=%v", updated, audit.transitions, err)
	}
	health := providerhealth.New()
	for index, outcome := range []providerhealth.Outcome{
		providerhealth.OutcomeFailed, providerhealth.OutcomeSucceeded, providerhealth.OutcomeFailed,
	} {
		if err = health.Record(ctx, providerhealth.Observation{
			ChannelNo: "channel-test", Operation: providerhealth.OperationWrite,
			Outcome: outcome, SummaryCode: "policy_management_" + string(rune('a'+index)),
			Latency: time.Millisecond,
		}); err != nil {
			t.Fatalf("record policy-managed health sample %d failed: %v", index, err)
		}
	}
	protected, err := management.List(ctx, providerhealth.HealthListInput{ChannelNo: "channel-test", Limit: 1})
	if err != nil || len(protected.Items) != 1 || !protected.Items[0].CircuitOpen ||
		protected.Items[0].WriteProtectedUntil == nil || protected.Items[0].HealthCode != "unavailable" {
		t.Fatalf("policy-managed protection did not open: page=%+v err=%v", protected, err)
	}
	deadline := *protected.Items[0].WriteProtectedUntil
	preserved, err := management.UpdatePolicy(ctx, providerhealth.HealthPolicyUpdateInput{
		ChannelNo: "channel-test", WindowSampleLimit: 10,
		MinimumSamplesToProtect: 10, FailureLimit: 10, UnknownResultLimit: 10,
		ConsecutiveFailureLimit: 10, ProtectionDurationSeconds: 60, ExpectedVersion: 2,
	})
	if err != nil || !preserved.CircuitOpen || preserved.WriteProtectedUntil == nil ||
		preserved.WriteProtectedUntil.Before(deadline) || len(audit.transitions) != 2 {
		t.Fatalf("active protection was shortened by policy update: item=%+v err=%v", preserved, err)
	}
	if _, err = management.UpdatePolicy(ctx, providerhealth.HealthPolicyUpdateInput{
		ChannelNo: "channel-test", WindowSampleLimit: 10,
		MinimumSamplesToProtect: 3, FailureLimit: 2, UnknownResultLimit: 1,
		ConsecutiveFailureLimit: 2, ProtectionDurationSeconds: 600, ExpectedVersion: 2,
	}); !bizerr.Is(err, providerhealth.CodeHealthManagementConflict) {
		t.Fatalf("stale Provider health policy version returned %v", err)
	}

	failingAudit, err := providerhealth.NewManagement(database, managementcap.Runtime{
		Authorizer: authorizer, Audit: &providerCapacityAuditWriter{err: errors.New("audit unavailable")},
	})
	if err != nil {
		t.Fatalf("construct failing-audit Provider health management failed: %v", err)
	}
	if _, err = failingAudit.UpdatePolicy(ctx, providerhealth.HealthPolicyUpdateInput{
		ChannelNo: "channel-test", WindowSampleLimit: 12,
		MinimumSamplesToProtect: 3, FailureLimit: 2, UnknownResultLimit: 1,
		ConsecutiveFailureLimit: 2, ProtectionDurationSeconds: 600, ExpectedVersion: 3,
	}); !bizerr.Is(err, providerhealth.CodeHealthManagementUnavailable) {
		t.Fatalf("Provider health audit failure returned %v", err)
	}
	policyVersion, err := database.GetValue(ctx, `
SELECT policy_version FROM plugin_idc_resource_channel_health_state
WHERE channel_no = 'channel-test'`)
	if err != nil || policyVersion.Int64() != 3 {
		t.Fatalf("audit failure did not roll back health policy: version=%d err=%v", policyVersion.Int64(), err)
	}

	denied, err := providerhealth.NewManagement(database, managementcap.Runtime{
		Authorizer: &providerCapacityManagementAuthorizer{actor: 5101, err: errors.New("denied")},
		Audit:      audit,
	})
	if err != nil {
		t.Fatalf("construct denied Provider health management failed: %v", err)
	}
	if _, err = denied.List(ctx, providerhealth.HealthListInput{Cursor: "invalid cursor", Limit: -1}); !bizerr.Is(err, providerhealth.CodeHealthManagementNotAccessible) {
		t.Fatalf("Provider health authorization did not precede malformed validation: %v", err)
	}
}
