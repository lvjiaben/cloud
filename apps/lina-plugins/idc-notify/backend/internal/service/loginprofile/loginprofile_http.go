// This file reads the authoritative self-profile through the local public route.
package loginprofile

import (
	"context"
	"encoding/json"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/net/ghttp"
	"io"
	"net"
	"net/http"
)

// Current calls only the local listener's published self-profile route and
// rejects missing authentication, malformed envelopes and oversized responses.
func (c *Client) Current(ctx context.Context) (Snapshot, error) {
	var result Snapshot
	request := ghttp.RequestFromCtx(ctx)
	if request == nil || request.Header.Get("Authorization") == "" {
		return result, gerror.New("authenticated profile request is unavailable")
	}
	address, ok := request.Context().Value(http.LocalAddrContextKey).(net.Addr)
	if !ok {
		return result, gerror.New("local HTTP listener is unavailable")
	}
	host, port, err := net.SplitHostPort(address.String())
	if err != nil {
		return result, err
	}
	if ip := net.ParseIP(host); ip == nil {
		return result, gerror.New("local HTTP listener must be an IP address")
	} else if ip.IsUnspecified() {
		host = "127.0.0.1"
	}
	scheme := "http"
	if request.TLS != nil {
		scheme = "https"
	}
	call, err := http.NewRequestWithContext(ctx, http.MethodGet, scheme+"://"+net.JoinHostPort(host, port)+"/api/v1/user/profile", nil)
	if err != nil {
		return result, err
	}
	call.Header.Set("Authorization", request.Header.Get("Authorization"))
	response, err := c.client.Do(call)
	if err != nil {
		return result, err
	}
	body, readErr := io.ReadAll(io.LimitReader(response.Body, 32769))
	closeErr := response.Body.Close()
	if readErr != nil {
		return result, readErr
	}
	if closeErr != nil {
		return result, gerror.Wrap(closeErr, "close profile response")
	}
	if len(body) > 32768 {
		return result, gerror.New("profile response exceeds size limit")
	}
	if response.StatusCode != http.StatusOK {
		return result, gerror.New("profile lookup failed")
	}
	var envelope struct {
		Code int      `json:"code"`
		Data Snapshot `json:"data"`
	}
	if err = json.Unmarshal(body, &envelope); err != nil {
		return result, err
	}
	if envelope.Code != 0 || envelope.Data.ID <= 0 {
		return result, gerror.New("invalid profile response")
	}
	return envelope.Data, nil
}
