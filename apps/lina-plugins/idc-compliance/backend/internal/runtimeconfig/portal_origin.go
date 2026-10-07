// Package runtimeconfig loads fail-closed compliance deployment settings.
package runtimeconfig

import (
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
)

const customerPublicOriginEnvironment = "LINAPRO_IDC_CUSTOMER_PUBLIC_ORIGIN"

// LoadPersonalVerificationReturnURL builds the only Aliyun browser return URL.
// Request payloads, Referer, and Host headers never participate in this value.
func LoadPersonalVerificationReturnURL() (string, error) {
	parsed, err := loadCustomerPublicOrigin()
	if err != nil || !validCustomerPublicOrigin(parsed) {
		return "", gerror.New("personal verification customer public origin is invalid")
	}
	return (&url.URL{
		Scheme: parsed.Scheme, Host: parsed.Host, Path: "/account/identity",
		RawQuery: "verification_return=1",
	}).String(), nil
}

// LoadContractSigningReturnURL builds the only Tencent signing browser return URL.
func LoadContractSigningReturnURL() (string, error) {
	parsed, err := loadCustomerPublicOrigin()
	if err != nil || !validCustomerPublicOrigin(parsed) {
		return "", gerror.New("contract signing customer public origin is invalid")
	}
	return (&url.URL{
		Scheme: parsed.Scheme, Host: parsed.Host, Path: "/account/contracts",
		RawQuery: "signing_return=1",
	}).String(), nil
}

func loadCustomerPublicOrigin() (*url.URL, error) {
	return url.Parse(strings.TrimSpace(os.Getenv(customerPublicOriginEnvironment)))
}

func validCustomerPublicOrigin(parsed *url.URL) bool {
	if parsed == nil || parsed.Host == "" || parsed.Hostname() == "" || parsed.User != nil ||
		(parsed.Path != "" && parsed.Path != "/") || parsed.RawPath != "" || parsed.RawQuery != "" ||
		parsed.Fragment != "" || parsed.Opaque != "" || parsed.ForceQuery {
		return false
	}
	if parsed.Scheme == "https" {
		return true
	}
	hostname := parsed.Hostname()
	loopback := strings.EqualFold(hostname, "localhost")
	if ip := net.ParseIP(hostname); ip != nil {
		loopback = ip.IsLoopback()
	}
	return parsed.Scheme == "http" && loopback
}
