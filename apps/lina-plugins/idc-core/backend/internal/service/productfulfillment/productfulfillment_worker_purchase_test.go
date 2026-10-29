// These tests verify that downstream configuration is translated only through
// authenticated import mappings, without losing exact scalar values or leaking
// local upload tokens to the upstream installation.

package productfulfillment

import (
	"encoding/json"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

func TestPeerPurchaseMapsFrozenStaticAndDynamicSelections(t *testing.T) {
	snapshot := &channelcommercecap.ImportSnapshot{
		Schema: &capmodel.ConfigurationSchemaDeclaration{Fields: []*capmodel.ConfigurationFieldDeclaration{
			{Key: "cpu", Type: capmodel.ConfigurationFieldSingleSelect},
			{Key: "zone", Type: capmodel.ConfigurationFieldTreeSelect},
			{Key: "count", Type: capmodel.ConfigurationFieldInteger},
			{Key: "name", Type: capmodel.ConfigurationFieldSingleLine},
		}},
		Mappings: []channelcommercecap.ImportFieldMapping{
			{FieldKey: "cpu", FieldToken: "field-cpu", Options: []channelcommercecap.ImportOptionMapping{{Value: "opaque-cpu", OptionToken: "upstream-cpu"}}},
			{FieldKey: "zone", FieldToken: "field-zone"},
			{FieldKey: "count", FieldToken: "field-count"},
			{FieldKey: "name", FieldToken: "field-name"},
		},
	}
	values, err := configurationcontract.DecodeCompiledValues(snapshot.Schema, json.RawMessage(`{"cpu":"opaque-cpu","zone":"opaque-zone","count":9007199254740993,"name":"example"}`))
	if err != nil {
		t.Fatal(err)
	}
	candidates := []peerCandidateMapping{{FieldKey: "zone", CandidateValue: "opaque-zone", OptionToken: "upstream-zone"}}
	result, err := mapPeerPurchaseValues(snapshot, values, candidates)
	if err != nil {
		t.Fatal(err)
	}
	byField := make(map[string]channelcommercecap.PurchaseValue)
	for _, value := range result {
		byField[value.FieldToken] = value
	}
	if got := byField["field-count"].IntegerValue; got == nil || *got != "9007199254740993" {
		t.Fatalf("integer precision lost: %#v", got)
	}
	if byField["field-cpu"].OptionTokens[0] != "upstream-cpu" || byField["field-zone"].TreeOptionTokens[0] != "upstream-zone" {
		t.Fatalf("opaque mappings not used: %#v", result)
	}
	if _, err = mapPeerPurchaseValues(snapshot, values, nil); err == nil {
		t.Fatal("missing dynamic mapping accepted")
	}
	candidates = append(candidates, peerCandidateMapping{FieldKey: "cpu", CandidateValue: "opaque-cpu", OptionToken: "changed"})
	if _, err = mapPeerPurchaseValues(snapshot, values, candidates); err == nil {
		t.Fatal("conflicting static mapping accepted")
	}
}

func TestPeerPurchaseRejectsLocalFileTokensAndUnknownFields(t *testing.T) {
	if _, err := mapPeerPurchaseValue(configurationcontract.ConfigurationValue{FieldKey: "upload", FileTokens: []string{"local-private-upload"}}, "upstream-field", string(capmodel.ConfigurationFieldFile), nil); err == nil {
		t.Fatal("local upload token leaked upstream")
	}
	if _, err := mapPeerPurchaseValue(configurationcontract.ConfigurationValue{FieldKey: "unknown"}, "", "text", nil); err == nil {
		t.Fatal("unknown field accepted")
	}
}

func TestPeerUnknownRecoveryNeverAuthorizesResubmit(t *testing.T) {
	claim := &claimedFulfillment{PluginID: "idc-peer", Action: productfulfillmentcap.ActionSubmit, QueryAttempts: 1}
	unknown := workerDecision{TaskStatus: "result_unknown", IntentStatus: "processing", PhaseCode: "frozen_unknown"}
	recovery := peerRecoveryDecision(claim, unknown, false, false)
	if recovery.PhaseCode != "post_submit_query" || recovery.TaskStatus != "retry_wait" || !recovery.ScheduleNext {
		t.Fatalf("lost response not recoverable: %+v", recovery)
	}
	absent := decideWorkerResult(recovery.TaskStatus, recovery.PhaseCode, productfulfillmentcap.ActionQuery, &productfulfillmentcap.Result{Status: productfulfillmentcap.StatusNotFound}, false, true, false, false, 2, 1)
	recovery = peerRecoveryDecision(claim, absent, false, false)
	if recovery.PhaseCode != "post_submit_query" {
		t.Fatalf("not found authorized resubmit: %+v", recovery)
	}
	claim.QueryAttempts = maximumQueryAttempts
	if recovery = peerRecoveryDecision(claim, unknown, false, false); recovery.ScheduleNext || recovery.IntentStatus != "manual_pending" {
		t.Fatalf("recovery unbounded: %+v", recovery)
	}
	if recovery = peerRecoveryDecision(claim, unknown, true, false); recovery != unknown {
		t.Fatal("takeover was overridden")
	}
}

func TestPeerRecoveryPreparationFailureKeepsWriteFence(t *testing.T) {
	claim := &claimedFulfillment{PluginID: "idc-peer", Action: productfulfillmentcap.ActionQuery, PreviousPhase: "post_submit_query", QueryAttempts: 2, SubmitAttempts: 1}
	preparationFailed := decideWorkerResult("retry_wait", claim.PreviousPhase, claim.Action, nil, true, false, false, false, claim.QueryAttempts, claim.SubmitAttempts)
	recovery := peerRecoveryDecision(claim, preparationFailed, false, false)
	if recovery.PhaseCode != "post_submit_query" || !recovery.ScheduleNext {
		t.Fatalf("preparation failure erased write fence: %+v", recovery)
	}
	absent := decideWorkerResult(recovery.TaskStatus, recovery.PhaseCode, claim.Action, &productfulfillmentcap.Result{Status: productfulfillmentcap.StatusNotFound}, false, true, false, false, 3, 1)
	if absent.PhaseCode == "submit_allowed" || absent.TaskStatus == "pending_submit" {
		t.Fatalf("recovery authorized another write: %+v", absent)
	}
	claim.QueryAttempts = maximumQueryAttempts
	exhausted := decideWorkerResult("retry_wait", claim.PreviousPhase, claim.Action, nil, true, false, false, false, claim.QueryAttempts, claim.SubmitAttempts)
	exhausted = peerRecoveryDecision(claim, exhausted, false, false)
	if exhausted.PhaseCode != "post_submit_query" || exhausted.ScheduleNext || exhausted.IntentStatus != "manual_pending" {
		t.Fatalf("exhausted recovery lost fence or remained automatic: %+v", exhausted)
	}
}

func TestPeerPurchaseOmitsSourceHiddenDynamicDefault(t *testing.T) {
	hidden := false
	snapshot := &channelcommercecap.ImportSnapshot{Mappings: []channelcommercecap.ImportFieldMapping{{FieldKey: "hidden", FieldToken: "upstream-field", CustomerVisible: &hidden, DefaultValue: json.RawMessage(`"opaque-default"`)}}, Schema: &capmodel.ConfigurationSchemaDeclaration{Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "hidden", Type: capmodel.ConfigurationFieldSingleSelect}}}}
	mapped, err := mapPeerPurchaseValues(snapshot, []configurationcontract.ConfigurationValue{{FieldKey: "hidden", OptionTokens: []string{"opaque-default"}}}, nil)
	if err != nil || len(mapped) != 0 {
		t.Fatalf("source default should need no cached token: %+v %v", mapped, err)
	}
}
