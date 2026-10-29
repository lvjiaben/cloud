// The managed candidate wire retains real node/area relationships without
// forwarding the supplier's other fields or increasing the number of reads.
package zjmfcloud

import (
	"encoding/json"
	"strings"
	"testing"

	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func TestManagedCandidatesPreserveExplicitNodeAreasWithoutExtraReads(t *testing.T) {
	executor := &lifecycleExecutor{t: t, response: `{"data":[{"id":7,"name":"Node Seven","area_id":2,"password":"must-not-leak"},{"id":"8","name":"Node Eight","area_id":"3","token":"must-not-leak"}]}`}
	result, err := ExecuteDynamicResource(executor, "context", resourcecap.DynamicProviderMethodConfigurationCandidates, json.RawMessage(`{"schemaVersion":"v1","requests":[{"fieldKey":"line"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	var value resourceinterfaceprojectioncap.CandidateResult
	if err = json.Unmarshal(result, &value); err != nil || len(value.Relationships) != 1 || len(value.Relationships[0].Links) != 2 {
		t.Fatalf("explicit relationship was lost: result=%s err=%v", result, err)
	}
	relation := value.Relationships[0]
	if relation.ParentFieldKey != "region" || relation.ChildFieldKey != "line" || relation.Links[0].ParentValue != "2" || relation.Links[1].ChildValue != "8" {
		t.Fatal("node identity or area identity was changed")
	}
	if strings.Contains(string(result), "must-not-leak") || strings.Contains(string(result), "password") || strings.Contains(string(result), "token") {
		t.Fatal("supplier fields escaped the whitelisted projection")
	}
	if len(executor.calls) != 2 || executor.calls[1].Method != "GET" || executor.calls[1].Path != "/nodes" {
		t.Fatal("relationship projection added per-region upstream reads")
	}
}

func TestNodeAreaRelationshipNeverGuessesMissingIdentifiers(t *testing.T) {
	for _, payload := range []string{
		`{"data":[{"id":7,"name":"Area Two"}]}`,
		`{"data":[{"id":7,"name":"Area Two","area_id":0}]}`,
		`{"data":[{"id":7,"name":"Area Two","area_id":{"id":2}}]}`,
		`{"data":[{"id":7,"name":"A","area_id":2},{"id":8,"name":"B"}]}`,
	} {
		items, err := projectDynamicCandidatePage(json.RawMessage(payload))
		if err != nil {
			t.Fatal(err)
		}
		if projectNodeAreaRelationship(json.RawMessage(payload), items) != nil {
			t.Fatal("incomplete upstream identifiers became an invented relationship")
		}
	}
}
