package admin

import (
	"testing"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/resourcetask"
)

func TestProviderTaskDetailMapsOnlySafeResidualResourceFields(t *testing.T) {
	detail := providerTaskDetail(&resourcetask.TaskDetail{ResidualResources: []resourcecap.ResidualResource{{
		ResourceKind: "volume", ResourceRef: "volume_600100",
		Status: resourcecap.ResidualResourceReleaseFailed, ReasonCode: "provider_rejected",
	}}})
	if detail == nil || len(detail.ResidualResources) != 1 {
		t.Fatalf("residual resource projection=%+v", detail)
	}
	item := detail.ResidualResources[0]
	if item.ResourceKind != "volume" || item.ResourceRef != "volume_600100" ||
		item.Status != "release_failed" || item.ReasonCode != "provider_rejected" {
		t.Fatalf("unsafe or incomplete residual projection=%+v", item)
	}
}
