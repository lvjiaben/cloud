package provisionsource

import (
	"bytes"
	"testing"
)

func TestCapabilityParameterValidationRemainsProviderOwned(t *testing.T) {
	t.Parallel()
	constraints := `[{"name":"hostname","required":true,"pattern":"^[a-z0-9.-]+$"},{"name":"cpu","required":true,"minimum":1,"maximum":64}]`
	if err := validateCapabilityParameters(constraints, []byte(`{"hostname":"server.example.test","cpu":4}`)); err != nil {
		t.Fatalf("validate dynamic provider parameters: %v", err)
	}
	for _, parameters := range [][]byte{
		[]byte(`{"hostname":"server.example.test"}`),
		[]byte(`{"hostname":"INVALID HOST","cpu":4}`),
		[]byte(`{"hostname":"server.example.test","cpu":65}`),
	} {
		if err := validateCapabilityParameters(constraints, parameters); err == nil {
			t.Fatalf("unsafe dynamic provider parameters were accepted: %s", parameters)
		}
	}
}

func TestGovernedCallbackURLRejectsUntrustedOrigins(t *testing.T) {
	for _, origin := range []string{"https://localhost.invalid", "http://example.com", "https://user:pass@example.com", "https://example.com/base", "https://example.com?host=evil"} {
		if _, err := governedCallbackURL(origin); err == nil {
			t.Fatalf("unsafe public origin was accepted: %q", origin)
		}
	}
	for _, origin := range []string{"https://platform.example.com", "http://127.0.0.1:19120", "http://localhost:19120"} {
		value, err := governedCallbackURL(origin)
		if err != nil || value != origin+"/api/plugins/idc-resource/provider-callback" {
			t.Fatalf("governed public origin rejected: value=%q err=%v", value, err)
		}
	}
}

func TestProviderCorrelationTokenIsExactly32OpaqueCharacters(t *testing.T) {
	var observed []byte
	err := providerCorrelationTokenProducer(bytes.NewReader(bytes.Repeat([]byte{0x5a}, 24)))(func(value []byte) error {
		observed = append([]byte(nil), value...)
		return nil
	})
	if err != nil || len(observed) != 32 || bytes.ContainsAny(observed, "+/=") {
		t.Fatalf("unsafe callback token: length=%d token=%q err=%v", len(observed), observed, err)
	}
}
