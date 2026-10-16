// These tests verify the stable public diagnostic status never includes an
// unreviewed supplier message or a raw transport cause.
package providerverification

import (
	"errors"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"testing"
)

func TestProviderConnectionFailureStatus(t *testing.T) {
	for _, tc := range []struct {
		code   *bizerr.Code
		status string
	}{
		{resourcecap.CodeProviderTLSUntrusted, "tls_untrusted"},
		{resourcecap.CodeProviderTimeout, "timeout"},
		{resourcecap.CodeProviderDNS, "dns"},
		{resourcecap.CodeProviderNetwork, "network"},
		{resourcecap.CodeProviderEndpointNotFound, "endpoint_not_found"},
		{resourcecap.CodeProviderInvalidResponse, "invalid_response"},
		{resourcecap.CodeProviderAuthRejected, "auth_rejected"},
		{resourcecap.CodeProviderLocalAuthPolicy, "local_auth_policy"},
	} {
		if got := connectionFailureStatus(bizerr.NewCode(tc.code)); got != tc.status {
			t.Fatalf("got %s, want %s", got, tc.status)
		}
	}
	if got := connectionFailureStatus(errors.New("password=private upstream")); got != "connection_failed" {
		t.Fatal("unreviewed diagnostic leaked", got)
	}
}
