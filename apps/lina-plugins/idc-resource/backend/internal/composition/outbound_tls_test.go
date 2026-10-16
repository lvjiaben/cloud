// This file proves certificate trust is scoped to one exact leaf certificate.
package composition

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProviderTLSConfigRequiresSystemTrustOrExactCertificatePin(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	digest := sha256.Sum256(server.Certificate().Raw)
	for _, test := range []struct {
		name, fingerprint string
		wantSuccess       bool
	}{
		{name: "system trust rejects self signed"},
		{name: "explicit exact pin", fingerprint: hex.EncodeToString(digest[:]), wantSuccess: true},
		{name: "changed leaf rejected", fingerprint: strings.Repeat("0", 64)},
	} {
		t.Run(test.name, func(t *testing.T) {
			policy, err := providerTLSConfig("127.0.0.1", test.fingerprint)
			if err != nil {
				t.Fatal(err)
			}
			transport := &http.Transport{TLSClientConfig: policy}
			defer transport.CloseIdleConnections()
			client := &http.Client{Transport: transport}
			response, requestErr := client.Get(server.URL)
			if response != nil {
				if closeErr := response.Body.Close(); closeErr != nil {
					t.Fatal(closeErr)
				}
			}
			if (requestErr == nil) != test.wantSuccess {
				t.Fatalf("success=%v want=%v", requestErr == nil, test.wantSuccess)
			}
		})
	}
	if _, err := providerTLSConfig("supplier.example", "not-a-sha256"); err == nil {
		t.Fatal("invalid fingerprint accepted")
	}
	// A pin on one channel must not change the defaults of a later channel.
	policy, err := providerTLSConfig("other.example", "")
	if err != nil || policy.InsecureSkipVerify || policy.VerifyConnection != nil {
		t.Fatal("explicit trust escaped its channel")
	}
}
