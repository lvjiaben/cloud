package dcim

import (
	"encoding/json"
	"testing"
	"time"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func TestDecodeProtocolRequiresOneExactFrozenValue(t *testing.T) {
	t.Parallel()
	for _, value := range []string{`{}`, `{"protocol":"auto"}`, `{"protocol":"legacy","fallback":"bms"}`, `{"protocol":"BMS"}`} {
		if _, err := decodeProtocol(json.RawMessage(value)); err == nil {
			t.Fatalf("decodeProtocol(%s) accepted an ambiguous configuration", value)
		}
	}
	for _, value := range []string{`{"protocol":"legacy"}`, `{"protocol":"bms"}`} {
		if _, err := decodeProtocol(json.RawMessage(value)); err != nil {
			t.Fatalf("decodeProtocol(%s): %v", value, err)
		}
	}
}

func fixedTime() time.Time { return time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC) }

func TestCandidateRoutesNeverMixProtocols(t *testing.T) {
	t.Parallel()
	request := candidateRequest{SchemaVersion: "v1", Requests: []candidateQuery{{FieldKey: "server_group"}, {FieldKey: "operating_system"}}}
	legacy := candidateCalls(ProtocolLegacy, request)
	if legacy[0].Path != "/index.php" || legacy[0].Query.Get("a") != "getSaleGroup" || legacy[1].Query.Get("a") != "getAllMirrorOs" {
		t.Fatalf("unexpected legacy calls: %#v", legacy)
	}
	bms := candidateCalls(ProtocolBMS, request)
	if bms[0].Path != "/bms/lcGroup" || bms[1].Path != "/bms/os" {
		t.Fatalf("unexpected BMS calls: %#v", bms)
	}
}

func TestProjectCandidatesKeepsOnlyBoundedIdentifiersAndLabels(t *testing.T) {
	t.Parallel()
	items, err := projectCandidates(ProtocolBMS, "server_group", json.RawMessage(`{"code":200,"data":{"list":[{"id":12,"name":"北京组"},{"id":"13","name":"Shanghai"}]}}`))
	if err != nil || len(items) != 2 || items[0].Value != "12" || items[1].LabelEnUS != "Shanghai" {
		t.Fatalf("projectCandidates() = %#v, %v", items, err)
	}
}

func TestBMSOperatingSystemCandidatesExcludeDisabledAndRescueImages(t *testing.T) {
	t.Parallel()
	items, err := projectCandidates(ProtocolBMS, "operating_system", json.RawMessage(`{"data":{"list":[{"id":1,"name":"Ubuntu","os_name":"ubuntu.raw","status":1},{"id":2,"name":"Disabled","os_name":"off.raw","status":0},{"id":3,"name":"Rescue","os_name":"Linux_Rescue.raw","status":1}]}}`))
	if err != nil || len(items) != 1 || items[0].Value != "1" {
		t.Fatalf("unexpected BMS OS candidates: %#v, %v", items, err)
	}
}

func TestCapabilitySnapshotsAreComplete(t *testing.T) {
	t.Parallel()
	for _, protocol := range []Protocol{ProtocolLegacy, ProtocolBMS} {
		snapshot := declaredCapabilitySnapshot(protocol, fixedTime())
		if len(snapshot.Capabilities) != resourcecap.MaximumCapabilityCount || snapshot.ValidateAt(fixedTime()) != nil {
			t.Fatalf("invalid %s capability snapshot: %#v", protocol, snapshot)
		}
	}
}
