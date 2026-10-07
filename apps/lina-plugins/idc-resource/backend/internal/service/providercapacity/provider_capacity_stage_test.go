// This file verifies deterministic bounded providerCapacity fingerprints.

package providercapacity

import (
	"testing"
	"time"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func TestPageFingerprintIsOrderIndependentWithinPage(t *testing.T) {
	t.Parallel()
	observedAt := time.Now().UTC()
	left, err := canonicalizeItems([]resourcecap.ProviderCapacityItem{
		{Kind: resourcecap.ProviderCapacityKindImage, ExternalRef: "image-b", Data: []byte(`{"name":"b"}`), ObservedAt: observedAt},
		{Kind: resourcecap.ProviderCapacityKindImage, ExternalRef: "image-a", Data: []byte(`{"name":"a"}`), ObservedAt: observedAt},
	})
	if err != nil {
		t.Fatalf("canonicalize first providerCapacity page failed: %v", err)
	}
	right, err := canonicalizeItems([]resourcecap.ProviderCapacityItem{
		{Kind: resourcecap.ProviderCapacityKindImage, ExternalRef: "image-a", Data: []byte("{\n\t\"name\": \"a\"\n}"), ObservedAt: observedAt},
		{Kind: resourcecap.ProviderCapacityKindImage, ExternalRef: "image-b", Data: []byte(`{"name":"b"}`), ObservedAt: observedAt},
	})
	if err != nil {
		t.Fatalf("canonicalize reordered providerCapacity page failed: %v", err)
	}
	if pageFingerprint("", "channel-test", resourcecap.ProviderCapacityKindImage, 100, 1, "", "next", false, left) !=
		pageFingerprint("", "channel-test", resourcecap.ProviderCapacityKindImage, 100, 1, "", "next", false, right) {
		t.Fatal("canonical providerCapacity page fingerprint changed with provider item order")
	}
}

func TestPageFingerprintCoversContinuationSemantics(t *testing.T) {
	t.Parallel()
	items := []canonicalItem{{externalRef: "image-a", projection: []byte(`{"name":"a"}`)}}
	baseline := pageFingerprint("", "channel-test", resourcecap.ProviderCapacityKindImage, 100, 1, "", "next", false, items)
	cases := []string{
		pageFingerprint("previous", "channel-test", resourcecap.ProviderCapacityKindImage, 100, 1, "", "next", false, items),
		pageFingerprint("", "channel-other", resourcecap.ProviderCapacityKindImage, 100, 1, "", "next", false, items),
		pageFingerprint("", "channel-test", resourcecap.ProviderCapacityKindPlan, 100, 1, "", "next", false, items),
		pageFingerprint("", "channel-test", resourcecap.ProviderCapacityKindImage, 50, 1, "", "next", false, items),
		pageFingerprint("", "channel-test", resourcecap.ProviderCapacityKindImage, 100, 2, "", "next", false, items),
		pageFingerprint("", "channel-test", resourcecap.ProviderCapacityKindImage, 100, 1, "request", "next", false, items),
		pageFingerprint("", "channel-test", resourcecap.ProviderCapacityKindImage, 100, 1, "", "other", false, items),
		pageFingerprint("", "channel-test", resourcecap.ProviderCapacityKindImage, 100, 1, "", "next", true, items),
	}
	for _, candidate := range cases {
		if candidate == baseline {
			t.Fatal("providerCapacity fingerprint omitted continuation semantics")
		}
	}
}
