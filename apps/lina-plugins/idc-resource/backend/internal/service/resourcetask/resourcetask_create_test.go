// This file verifies resource task input validation and exact idempotent replay semantics.

package resourcetask

import (
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/model/entity"
)

func TestCreateInputRequiresExecutableSecretFreeIntent(t *testing.T) {
	valid := CreateInput{
		RequestID:       "request-1",
		Capability:      resourcecap.CapabilityStart,
		LocalResourceNo: "resource-1",
		ChannelNo:       "channel-1",
		PoolNo:          "pool-1",
		Parameters:      []byte(`{"mode":"safe"}`),
		Priority:        100,
		MaxAttempts:     8,
	}
	handle, err := secretcap.NewHandle("secret-ref-1")
	if err != nil {
		t.Fatalf("construct test secret handle failed: %v", err)
	}
	valid.SecretBindings = []resourcecap.SecretBinding{{Name: "password", Handle: handle}}
	canonical, ownerMetadata, err := validateCreateInput(valid)
	if err != nil || string(canonical) != `{"mode":"safe"}` || string(ownerMetadata) != `{}` {
		t.Fatalf("valid create input rejected: canonical=%s owner=%s err=%v",
			canonical, ownerMetadata, err)
	}
	tests := []struct {
		name   string
		mutate func(*CreateInput)
	}{
		{name: "missing resource", mutate: func(input *CreateInput) { input.LocalResourceNo = "" }},
		{name: "non executable capability", mutate: func(input *CreateInput) {
			input.Capability = resourcecap.CapabilityCode("provider-capacity.list")
		}},
		{name: "secret parameter", mutate: func(input *CreateInput) {
			input.Parameters = []byte(`{"password":"must-not-pass"}`)
		}},
		{name: "secret owner metadata", mutate: func(input *CreateInput) {
			input.OwnerMetadata = []byte(`{"accessToken":"must-not-pass"}`)
		}},
		{name: "invalid channel", mutate: func(input *CreateInput) { input.ChannelNo = "channel/1" }},
		{name: "invalid pool", mutate: func(input *CreateInput) { input.PoolNo = "pool/1" }},
		{name: "negative priority", mutate: func(input *CreateInput) { input.Priority = -1 }},
		{name: "attempt overflow", mutate: func(input *CreateInput) { input.MaxAttempts = maximumTaskAttempts + 1 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := valid
			test.mutate(&input)
			if _, _, err := validateCreateInput(input); err == nil {
				t.Fatal("invalid create input was accepted")
			}
		})
	}
}

func TestCanonicalJSONObjectHasStableObjectSemantics(t *testing.T) {
	first, err := canonicalJSONObject([]byte(`{"b":2,"a":1}`))
	if err != nil {
		t.Fatalf("canonicalize first object failed: %v", err)
	}
	second, err := canonicalJSONObject([]byte(" { \"a\" : 1, \"b\" : 2 } \n"))
	if err != nil || string(first) != string(second) || string(first) != `{"a":1,"b":2}` {
		t.Fatalf("objects were not canonicalized consistently: first=%s second=%s err=%v", first, second, err)
	}
	for _, invalid := range [][]byte{[]byte(`[]`), []byte(`null`), []byte(`{} {}`)} {
		if _, err = canonicalJSONObject(invalid); err == nil {
			t.Fatalf("invalid object payload was accepted: %s", invalid)
		}
	}
}

func TestSameCreateIntentRejectsIdempotencySemanticDrift(t *testing.T) {
	availableAt := time.Now().UTC().Add(time.Hour)
	input := CreateInput{
		Capability: resourcecap.CapabilityStart, LocalResourceNo: "resource-1",
		ChannelNo: "channel-1", PoolNo: "pool-1", Priority: 100, MaxAttempts: 8,
		AvailableAt: &availableAt,
	}
	row := &entity.Task{
		CapabilityCode: string(input.Capability), LocalResourceNo: input.LocalResourceNo,
		ChannelNo: input.ChannelNo, PoolNo: input.PoolNo, Priority: input.Priority,
		MaxAttempts: input.MaxAttempts, SanitizedParameters: `{"a": 1}`,
		OwnerMetadata: `{}`, AvailableAt: &availableAt,
	}
	if !sameCreateIntent(row, input, []byte(`{"a":1}`), []byte(`{}`)) {
		t.Fatal("equivalent idempotent task intent was rejected")
	}
	row.CapabilityCode = string(resourcecap.CapabilityStop)
	if sameCreateIntent(row, input, []byte(`{"a":1}`), []byte(`{}`)) {
		t.Fatal("changed idempotent task capability was accepted")
	}
	row.CapabilityCode = string(input.Capability)
	if sameCreateIntent(row, input, []byte(`{"a":1}`), []byte(`{"changed":true}`)) {
		t.Fatal("changed idempotent owner metadata was accepted")
	}
}

func TestTaskNumberUsesOpaqueRandomMaterial(t *testing.T) {
	number, err := newTaskNumber(zeroReader{})
	if err != nil || number != "task_00000000000000000000000000000000" {
		t.Fatalf("unexpected deterministic task number %q err=%v", number, err)
	}
	binding, err := newBindingNumber(zeroReader{})
	if err != nil || binding != "binding_00000000000000000000000000000000" {
		t.Fatalf("unexpected deterministic binding number %q err=%v", binding, err)
	}
}
