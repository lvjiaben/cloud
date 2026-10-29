// This file reduces transport and login failures to reviewed owner-side codes.
// Raw transport errors and supplier payloads never cross the plugin boundary.
package composition

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"net"
	"net/http"
)

func confidentialTransportError(err error) error {
	if safe := resourcecap.SafeProviderConnectionError(err); safe != nil {
		return safe
	}
	var cert *tls.CertificateVerificationError
	var authority x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var invalid x509.CertificateInvalidError
	var roots x509.SystemRootsError
	if errors.As(err, &cert) || errors.As(err, &authority) || errors.As(err, &hostname) || errors.As(err, &invalid) || errors.As(err, &roots) {
		return bizerr.NewCode(resourcecap.CodeProviderTLSUntrusted)
	}
	var network net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &network) && network.Timeout()) {
		return bizerr.NewCode(resourcecap.CodeProviderTimeout)
	}
	var dns *net.DNSError
	if errors.As(err, &dns) {
		return bizerr.NewCode(resourcecap.CodeProviderDNS)
	}
	return bizerr.NewCode(resourcecap.CodeProviderNetwork)
}

func confidentialAuthenticationResponseError(content []byte) error {
	var data map[string]json.RawMessage
	if json.Unmarshal(content, &data) == nil {
		for _, key := range []string{"status", "code"} {
			var status int
			if json.Unmarshal(data[key], &status) != nil {
				continue
			}
			switch status {
			case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden:
				return bizerr.NewCode(resourcecap.CodeProviderAuthRejected)
			case http.StatusNotFound:
				return bizerr.NewCode(resourcecap.CodeProviderEndpointNotFound)
			}
		}
	}
	return bizerr.NewCode(resourcecap.CodeProviderInvalidResponse)
}

// safeConfidentialProviderError retains only validated owner classifications,
// including definitive principal rejection, without supplier messages or causes.
func safeConfidentialProviderError(err error) error {
	if safe := resourcecap.SafeProviderConnectionError(err); safe != nil {
		return safe
	}
	var typed *resourcecap.ProviderCallError
	if errors.As(err, &typed) && typed != nil && typed.Validate() == nil {
		return resourcecap.NormalizeProviderError(typed)
	}
	return nil
}
