// This file defines owner-reviewed connection failures without retaining URLs,
// supplier messages, credentials or raw transport causes.
package resourcecap

import (
	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
)

// Safe connection diagnostic codes may cross the confidential runtime boundary.
var (
	CodeProviderTLSUntrusted     = bizerr.MustDefine("IDC_RESOURCE_PROVIDER_TLS_UNTRUSTED", "The upstream TLS certificate could not be verified", gcode.CodeInvalidOperation)
	CodeProviderTimeout          = bizerr.MustDefine("IDC_RESOURCE_PROVIDER_TIMEOUT", "The upstream connection timed out", gcode.CodeInvalidOperation)
	CodeProviderDNS              = bizerr.MustDefine("IDC_RESOURCE_PROVIDER_DNS", "The upstream hostname could not be resolved", gcode.CodeInvalidOperation)
	CodeProviderNetwork          = bizerr.MustDefine("IDC_RESOURCE_PROVIDER_NETWORK", "The upstream network connection failed", gcode.CodeInvalidOperation)
	CodeProviderEndpointNotFound = bizerr.MustDefine("IDC_RESOURCE_PROVIDER_ENDPOINT_NOT_FOUND", "The upstream API endpoint was not found", gcode.CodeInvalidOperation)
	CodeProviderInvalidResponse  = bizerr.MustDefine("IDC_RESOURCE_PROVIDER_INVALID_RESPONSE", "The upstream API returned an invalid response", gcode.CodeInvalidOperation)
	CodeProviderAuthRejected     = bizerr.MustDefine("IDC_RESOURCE_PROVIDER_AUTH_REJECTED", "The upstream API rejected authentication", gcode.CodeInvalidOperation)
	CodeProviderLocalAuthPolicy  = bizerr.MustDefine("IDC_RESOURCE_PROVIDER_LOCAL_AUTH_POLICY", "The connector authentication configuration does not match its request", gcode.CodeInvalidOperation)
)

// SafeProviderConnectionError copies only a known classification. It never
// returns the original error or its cause across a confidential boundary.
func SafeProviderConnectionError(err error) error {
	for _, code := range []*bizerr.Code{CodeProviderTLSUntrusted, CodeProviderTimeout, CodeProviderDNS, CodeProviderNetwork, CodeProviderEndpointNotFound, CodeProviderInvalidResponse, CodeProviderAuthRejected, CodeProviderLocalAuthPolicy} {
		if bizerr.Is(err, code) {
			return bizerr.NewCode(code)
		}
	}
	return nil
}
