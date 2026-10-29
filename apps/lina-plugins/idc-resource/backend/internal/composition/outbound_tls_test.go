// This file verifies the merchant's certificate-independent interface policy.
package composition

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProviderTLSConfigUsesEncryptedConnectionWithoutCertificateVerification(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	for _, fingerprint := range []string{"", "historical-pin-no-longer-used"} {
		policy, err := providerTLSConfig("different-certificate-host.example", fingerprint)
		if err != nil || !policy.InsecureSkipVerify || policy.VerifyConnection != nil || policy.MinVersion < tls.VersionTLS12 {
			t.Fatal("configured interface certificate policy was not applied")
		}
		transport := &http.Transport{TLSClientConfig: policy}
		client := &http.Client{Transport: transport}
		response, requestErr := client.Get(server.URL)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		if closeErr := response.Body.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
		transport.CloseIdleConnections()
	}
}
