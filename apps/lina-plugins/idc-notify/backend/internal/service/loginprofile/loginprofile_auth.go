// This file calls fixed public host authentication routes through the local
// listener. Passwords and temporary recovery sessions never enter plugin storage.
package loginprofile

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/gogf/gf/v2/net/ghttp"
	"lina-core/pkg/bizerr"
)

// Session is the ordinary host password-login result. Tokens remain host owned.
type Session struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

// AuthClient is the bounded public-route adapter used by customer authentication.
type AuthClient interface {
	PasswordLogin(context.Context, string, string) (*Session, error)
	SetPassword(context.Context, string, string, string) error
	Logout(context.Context, string) error
	RecoveryEnabled(context.Context) error
	CleanupReset(context.Context, string, string) (bool, error)
}

func (c *Client) call(ctx context.Context, method, path, token, proof string, input, output any) error {
	request := ghttp.RequestFromCtx(ctx)
	if request == nil {
		return bizerr.NewCode(codeAuthInvalid)
	}
	address, ok := request.Context().Value(http.LocalAddrContextKey).(net.Addr)
	if !ok {
		return bizerr.NewCode(codeAuthInvalid)
	}
	host, port, err := net.SplitHostPort(address.String())
	if err != nil {
		return bizerr.WrapCode(err, codeAuthInvalid)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return bizerr.NewCode(codeAuthInvalid)
	}
	if ip.IsUnspecified() {
		host = "127.0.0.1"
	}
	scheme := "http"
	if request.TLS != nil {
		scheme = "https"
	}
	body, err := json.Marshal(input)
	if err != nil {
		return bizerr.WrapCode(err, codeAuthInvalid)
	}
	defer clear(body)
	prefix := "/api/v1"
	if strings.HasPrefix(path, "/x/") {
		prefix = ""
	}
	call, err := http.NewRequestWithContext(ctx, method, scheme+"://"+net.JoinHostPort(host, port)+prefix+path, bytes.NewReader(body))
	if err != nil {
		return bizerr.WrapCode(err, codeAuthInvalid)
	}
	call.Header.Set("Content-Type", "application/json")
	call.Header.Set("User-Agent", request.UserAgent())
	if token != "" {
		call.Header.Set("Authorization", "Bearer "+token)
	}
	if proof != "" {
		call.Header.Set("X-Password-Recovery", proof)
	}
	response, err := c.client.Do(call)
	if err != nil {
		return bizerr.WrapCode(err, codeAuthInvalid)
	}
	raw, readErr := io.ReadAll(io.LimitReader(response.Body, 65537))
	closeErr := response.Body.Close()
	defer clear(raw)
	if readErr != nil {
		return bizerr.WrapCode(readErr, codeAuthInvalid)
	}
	if closeErr != nil {
		return bizerr.WrapCode(closeErr, codeAuthInvalid)
	}
	if response.StatusCode != http.StatusOK || len(raw) > 65536 {
		return bizerr.NewCode(codeAuthInvalid)
	}
	var envelope struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err = json.Unmarshal(raw, &envelope); err != nil {
		return bizerr.WrapCode(err, codeAuthInvalid)
	}
	if envelope.Code != 0 {
		return bizerr.NewCode(codeAuthInvalid)
	}
	if output != nil {
		if err = json.Unmarshal(envelope.Data, output); err != nil {
			return bizerr.WrapCode(err, codeAuthInvalid)
		}
	}
	return nil
}

// PasswordLogin retains the host's password, disabled-account and tenant checks.
func (c *Client) PasswordLogin(ctx context.Context, username, password string) (*Session, error) {
	var result Session
	err := c.call(ctx, http.MethodPost, "/auth/login", "", "", struct {
		Username   string `json:"username"`
		Password   string `json:"password"`
		ClientType string `json:"clientType"`
	}{username, password, "web"}, &result)
	if err != nil {
		return nil, err
	}
	if result.AccessToken == "" {
		return nil, bizerr.NewCode(codeAuthInvalid)
	}
	return &result, nil
}

// SetPassword invokes the original profile route with a recovery-only proof.
func (c *Client) SetPassword(ctx context.Context, token, password, proof string) error {
	return c.call(ctx, http.MethodPut, "/user/profile", token, proof, struct {
		Password string `json:"password"`
	}{password}, nil)
}

// Logout revokes a server-held temporary session through the public host route.
func (c *Client) Logout(ctx context.Context, token string) error {
	return c.call(ctx, http.MethodPost, "/auth/logout", token, "", struct{}{}, nil)
}

// RecoveryEnabled respects the host's published password-recovery switch.
func (c *Client) RecoveryEnabled(ctx context.Context) error {
	var result struct {
		Auth struct {
			Enabled bool `json:"forgetPasswordEnabled"`
		} `json:"auth"`
	}
	if err := c.call(ctx, http.MethodGet, "/config/public/frontend", "", "", nil, &result); err != nil {
		return err
	}
	if !result.Auth.Enabled {
		return bizerr.NewCode(codeAuthInvalid)
	}
	return nil
}

// CleanupReset runs with a real bearer-authenticated self context, never a forged
// administrator or user context. Pending cleanup remains observable to callers.
func (c *Client) CleanupReset(ctx context.Context, token string, recoveryNo string) (bool, error) {
	var result struct {
		Pending bool `json:"pending"`
	}
	err := c.call(ctx, http.MethodPost, "/x/idc-notify/api/v1/account/password-reset/session-cleanups", token, "", struct {
		RecoveryNo string `json:"recoveryNo"`
	}{recoveryNo}, &result)
	return result.Pending, err
}
