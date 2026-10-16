// This file supports channel-scoped leaf certificate trust without weakening
// TLS verification for other suppliers or accepting a changed certificate.
package composition

import (
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/hex"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

// providerTLSConfig keeps system trust as the default. An explicitly configured
// SHA-256 pin authenticates exactly the supplier certificate chosen by the admin.
func providerTLSConfig(host, fingerprint string) (*tls.Config, error) {
	policy := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: host}
	if fingerprint == "" {
		return policy, nil
	}
	expected, err := hex.DecodeString(fingerprint)
	if err != nil || len(expected) != sha256.Size {
		return nil, bizerr.NewCode(resourcecap.CodeProviderTLSUntrusted)
	}
	// The standard chain verifier cannot trust self-signed certificates. The
	// mandatory callback replaces it with exact certificate authentication.
	policy.InsecureSkipVerify = true
	policy.VerifyConnection = func(state tls.ConnectionState) error {
		if len(state.PeerCertificates) == 0 {
			return bizerr.NewCode(resourcecap.CodeProviderTLSUntrusted)
		}
		actual := sha256.Sum256(state.PeerCertificates[0].Raw)
		if subtle.ConstantTimeCompare(actual[:], expected) != 1 {
			return bizerr.NewCode(resourcecap.CodeProviderTLSUntrusted)
		}
		return nil
	}
	return policy, nil
}
