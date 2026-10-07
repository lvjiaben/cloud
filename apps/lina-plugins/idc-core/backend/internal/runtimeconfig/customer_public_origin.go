package runtimeconfig

import (
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
)

const customerPublicOriginEnvironment = "LINAPRO_IDC_CUSTOMER_PUBLIC_ORIGIN"

// CustomerPublicURL returns an absolute web-customer URL from the explicit
// installation origin. Request headers and the administration origin are not trusted.
func CustomerPublicURL(relativePath string) (string, error) {
	origin, err := parseCustomerPublicOrigin(os.Getenv(customerPublicOriginEnvironment))
	if err != nil {
		return "", err
	}
	reference, err := url.Parse(relativePath)
	allowedPath := strings.HasPrefix(reference.Path, "/account/") || reference.Path == "/auth/customer-activation"
	if err != nil || !allowedPath || reference.IsAbs() || reference.Host != "" {
		return "", gerror.New("IDC customer public path is invalid")
	}
	return origin.ResolveReference(reference).String(), nil
}

func parseCustomerPublicOrigin(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return nil, gerror.New("IDC customer public origin is invalid")
	}
	hostname := parsed.Hostname()
	loopback := strings.EqualFold(hostname, "localhost")
	if ip := net.ParseIP(hostname); ip != nil {
		loopback = ip.IsLoopback()
	}
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && loopback) {
		return nil, gerror.New("IDC customer public origin must use HTTPS outside local development")
	}
	parsed.Path = "/"
	return parsed, nil
}
