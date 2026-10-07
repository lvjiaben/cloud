package resourcecap

import (
	"testing"
	"time"
)

func TestAccessorySnapshotValidation(t *testing.T) {
	now := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	valid := AccessorySnapshot{
		ObservedAt: now.Add(-time.Minute), ExpiresAt: now.Add(14 * time.Minute),
		Items: []AccessoryItem{{
			Kind: AccessoryKindDisk, ExternalRef: "disk-41", DisplayName: "System disk",
			State: AccessoryStateAttached, Data: []byte(`{"sizeGiB":100,"system":true}`),
		}},
	}
	if err := valid.ValidateAt(now); err != nil {
		t.Fatal(err)
	}
	credential := valid
	credential.Items = append([]AccessoryItem(nil), valid.Items...)
	credential.Items[0].Data = []byte(`{"access_token":"secret-value"}`)
	if err := credential.ValidateAt(now); err == nil {
		t.Fatal("expected credential-bearing accessory projection to fail")
	}
	duplicate := valid
	duplicate.Items = append(append([]AccessoryItem(nil), valid.Items...), valid.Items[0])
	if err := duplicate.ValidateAt(now); err == nil {
		t.Fatal("expected duplicate accessory item to fail")
	}
}
