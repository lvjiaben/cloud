// Package cartcookie owns the browser-only anonymous cart credential boundary.
package cartcookie

import (
	"context"
	"net/http"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

const (
	Name               = "lina_idc_guest_cart"
	credentialLifetime = 30 * 24 * time.Hour
)

// Read returns the raw credential only to the HTTP controller boundary.
func Read(ctx context.Context) (string, error) {
	request := g.RequestFromCtx(ctx)
	if request == nil {
		return "", gerror.New("guest cart credential requires an HTTP request")
	}
	value := ghttp.GetCookie(request).Get(Name)
	if value == nil {
		return "", nil
	}
	return value.String(), nil
}

// Write stores the credential outside JavaScript-visible storage and response
// bodies. Secure is enabled whenever the inbound request is HTTPS.
func Write(ctx context.Context, token string) error {
	request := g.RequestFromCtx(ctx)
	if request == nil {
		return gerror.New("guest cart credential requires an HTTP request")
	}
	ghttp.GetCookie(request).SetHttpCookie(New(token, request.TLS != nil))
	return nil
}

// Clear expires the browser credential after a successful merge or when a
// stale/invalid credential is observed.
func Clear(ctx context.Context) error {
	request := g.RequestFromCtx(ctx)
	if request == nil {
		return gerror.New("guest cart credential requires an HTTP request")
	}
	cookie := New("", request.TLS != nil)
	cookie.Expires = time.Unix(1, 0)
	cookie.MaxAge = -1
	ghttp.GetCookie(request).SetHttpCookie(cookie)
	return nil
}

// New builds the exact cookie policy and is kept pure for security regression
// tests.
func New(token string, secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     Name,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		Expires:  time.Now().Add(credentialLifetime),
		MaxAge:   int(credentialLifetime / time.Second),
	}
}
