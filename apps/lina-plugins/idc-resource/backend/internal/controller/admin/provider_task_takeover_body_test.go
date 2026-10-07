package admin

import (
	"bytes"
	"testing"

	"lina-plugin-idc-resource/backend/api/admin/v1"
)

func TestProviderTaskTakeoverBodyViolation(t *testing.T) {
	t.Parallel()
	req := &v1.ProviderTaskManualTakeoverReq{
		ExpectedVersion: 3,
		ReasonCode:      "provider_result_unresolved",
		EvidenceNos:     []string{"evidence_11111111111111111111111111111111"},
	}
	valid := []byte(`{"expectedVersion":3,"reasonCode":"provider_result_unresolved","evidenceNos":["evidence_11111111111111111111111111111111"]}`)
	if providerTaskTakeoverBodyViolation(valid, req) {
		t.Fatal("exact Provider task takeover body was rejected")
	}
	invalid := map[string][]byte{
		"force status":    []byte(`{"expectedVersion":3,"reasonCode":"provider_result_unresolved","evidenceNos":["evidence_11111111111111111111111111111111"],"status":"succeeded"}`),
		"retry":           []byte(`{"expectedVersion":3,"reasonCode":"provider_result_unresolved","evidenceNos":["evidence_11111111111111111111111111111111"],"retry":true}`),
		"free text":       []byte(`{"expectedVersion":3,"reasonCode":"provider_result_unresolved","evidenceNos":["evidence_11111111111111111111111111111111"],"instruction":"force it"}`),
		"trailing JSON":   []byte(`{"expectedVersion":3,"reasonCode":"provider_result_unresolved","evidenceNos":["evidence_11111111111111111111111111111111"]}{}`),
		"binder mismatch": []byte(`{"expectedVersion":4,"reasonCode":"provider_result_unresolved","evidenceNos":["evidence_11111111111111111111111111111111"]}`),
		"oversized":       bytes.Repeat([]byte(" "), maximumProviderTaskTakeoverBodySize+1),
	}
	for name, body := range invalid {
		name, body := name, body
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if !providerTaskTakeoverBodyViolation(body, req) {
				t.Fatal("unsafe Provider task takeover body was accepted")
			}
		})
	}
}
