// Package loginprofile reads the current identity through its published HTTP
// profile endpoint. It never reads host tables or accepts a caller-supplied URL.
package loginprofile

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

// Snapshot contains only the current identity fields needed for verification.
type Snapshot struct {
	Username string `json:"username"`
	ID       int    `json:"id"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
}

// Reader obtains authoritative values under the current request's authentication.
type Reader interface {
	Current(context.Context) (Snapshot, error)
}

// Client reuses one bounded HTTP transport created during startup.
type Client struct{ client *http.Client }

// New creates the startup HTTP adapter; redirects and environment proxies are
// disabled so bearer credentials cannot escape the current listening service.
func New() *Client {
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true}, DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext, TLSHandshakeTimeout: 5 * time.Second, IdleConnTimeout: time.Minute, MaxIdleConns: 8}
	return &Client{client: &http.Client{Transport: transport, Timeout: 5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
