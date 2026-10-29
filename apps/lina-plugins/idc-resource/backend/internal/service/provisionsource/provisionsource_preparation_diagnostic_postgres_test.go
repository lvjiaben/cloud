// This file proves local task-validation failures are classified without queuing a supplier write.

package provisionsource

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/resourcetask"
)

type invalidTaskManagedBuilder struct{ provisionManagedBuilder }

func (b invalidTaskManagedBuilder) Build(ctx context.Context, channel, revision string, plan resourcecap.ServerProvisionPlan) (json.RawMessage, error) {
	raw, err := b.provisionManagedBuilder.Build(ctx, channel, revision, plan)
	if err != nil {
		return nil, err
	}
	var values map[string]json.RawMessage
	if err = json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	values["password"] = json.RawMessage(`"must-never-leak"`)
	return json.Marshal(values)
}

func TestProvisionSourcePostgreSQLPreparationDiagnosticBeforeTaskCreation(t *testing.T) {
	ctx, database := newProvisionSourceDatabase(t)
	seedProvisionRoute(t, ctx, database)
	service, err := New(database, newProvisionSecretSource(t), principalReady, nil, nil, invalidTaskManagedBuilder{})
	if err != nil {
		t.Fatal(err)
	}
	task, err := service.CreateTask(ctx, frozenProvisionSourceRequest(t))
	if err == nil || task != nil || !errors.Is(err, resourcetask.ErrInvalidInput) || !errors.Is(err, provisionresourcecap.ErrSourceUnavailable) {
		t.Fatal("invalid local task lost its error identity")
	}
	code, diagnostic := provisionresourcecap.PreparationDiagnostic(err)
	if code != string(provisionresourcecap.PreparationFailureTaskInput) || !strings.Contains(diagnostic, `"businessRequestStarted":false`) || strings.Contains(diagnostic, "must-never-leak") {
		t.Fatal("local task error lost its safe preparation diagnostic")
	}
	count, err := database.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_resource_task`)
	if err != nil || count.Int() != 0 {
		t.Fatal("failed local validation queued a resource task")
	}
}
