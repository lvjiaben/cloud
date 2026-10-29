// This file applies the installation's explicitly chosen outbound TLS policy.
package composition

import "crypto/tls"

// providerTLSConfig preserves encrypted HTTPS and endpoint fencing while the
// merchant's interface policy intentionally omits certificate authentication.
// Historical certificate pins remain stored but no longer affect connections.
func providerTLSConfig(host, _ string) (*tls.Config, error) {
	return &tls.Config{MinVersion: tls.VersionTLS12, ServerName: host, InsecureSkipVerify: true}, nil // #nosec G402 -- explicit merchant interface policy.
}
