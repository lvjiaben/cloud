// Package portalroute owns the exact customer-entry bridge retained by idc-portal.
package portalroute

import (
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/net/ghttp"

	"lina-core/pkg/plugin/pluginhost"
)

const (
	// customerPublicOriginEnvironment is the explicit web-customer deployment origin.
	customerPublicOriginEnvironment = "LINAPRO_IDC_CUSTOMER_PUBLIC_ORIGIN"
	// hostResetPasswordPath is the one host-authentication path owned by this bridge.
	hostResetPasswordPath = "/admin/auth/reset-password"
	// customerResetPasswordPath is the matching web-customer route.
	customerResetPasswordPath = "/auth/reset-password"
)

// Register binds only the exact host password-reset path before the management SPA.
func Register(registrar pluginhost.HTTPRegistrar) error {
	if registrar == nil {
		return gerror.New("portal HTTP registrar is unavailable")
	}
	origin, err := parseCustomerPublicOrigin(os.Getenv(customerPublicOriginEnvironment))
	if err != nil {
		return err
	}
	middlewares := registrar.GlobalMiddlewares()
	if middlewares == nil {
		return gerror.New("portal global middleware registrar is unavailable")
	}
	if err = middlewares.Bind(hostResetPasswordPath, redirectHandler(origin)); err != nil {
		return gerror.Wrap(err, "bind customer password-reset bridge")
	}
	return nil
}

// redirectHandler redirects an exact GET while preserving only one opaque token.
func redirectHandler(origin *url.URL) pluginhost.MiddlewareHandler {
	return func(request *ghttp.Request) {
		if request == nil {
			return
		}
		if request.Method != http.MethodGet || request.URL.Path != hostResetPasswordPath {
			request.Middleware.Next()
			return
		}

		request.Response.Header().Set("Location", resetPasswordTarget(origin, request.URL.RawQuery))
		request.Response.Header().Set("Cache-Control", "no-store")
		request.Response.WriteStatus(http.StatusFound)
		request.ExitAll()
	}
}

// resetPasswordTarget builds the fixed customer route and drops every query key except one token.
func resetPasswordTarget(origin *url.URL, rawQuery string) string {
	target := *origin
	target.Path = customerResetPasswordPath
	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		return target.String()
	}
	if tokens := query["token"]; len(tokens) == 1 && strings.TrimSpace(tokens[0]) != "" {
		target.RawQuery = url.Values{"token": []string{tokens[0]}}.Encode()
	}
	return target.String()
}

// parseCustomerPublicOrigin accepts HTTPS origins and explicit loopback HTTP for local development.
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
