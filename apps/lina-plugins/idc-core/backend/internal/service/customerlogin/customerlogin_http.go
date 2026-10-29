package customerlogin

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/gogf/gf/v2/net/ghttp"
	"io"
	"lina-core/pkg/bizerr"
	"net"
	"net/http"
	"strconv"
)

type hostProfile struct {
	ID       int    `json:"id"`
	TenantID int    `json:"tenantId"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	RoleIDs  []int  `json:"roleIds"`
	Status   int    `json:"status"`
}
type hostPage struct {
	List  []hostProfile `json:"list"`
	Total int           `json:"total"`
}

func (s *serviceImpl) request(ctx context.Context, path, token string, target any) error {
	return s.exchange(ctx, http.MethodGet, path, token, nil, target)
}
func (s *serviceImpl) exchange(ctx context.Context, method, path, token string, input, target any) error {
	request := ghttp.RequestFromCtx(ctx)
	if request == nil {
		return bizerr.NewCode(CodeInvalid)
	}
	address, ok := request.Context().Value(http.LocalAddrContextKey).(net.Addr)
	if !ok {
		return bizerr.NewCode(CodeUnavailable)
	}
	host, port, err := net.SplitHostPort(address.String())
	if err != nil {
		return bizerr.WrapCode(err, CodeUnavailable)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return bizerr.NewCode(CodeUnavailable)
	}
	if ip.IsUnspecified() {
		host = "127.0.0.1"
	}
	scheme := "http"
	if request.TLS != nil {
		scheme = "https"
	}
	if token == "" {
		token = request.Header.Get("Authorization")
	} else {
		token = "Bearer " + token
	}
	if token == "" {
		return bizerr.NewCode(CodeInvalid)
	}
	body, err := json.Marshal(input)
	if err != nil {
		return bizerr.WrapCode(err, CodeUnavailable)
	}
	defer clear(body)
	call, err := http.NewRequestWithContext(ctx, method, scheme+"://"+net.JoinHostPort(host, port)+"/api/v1"+path, bytes.NewReader(body))
	if err != nil {
		return bizerr.WrapCode(err, CodeUnavailable)
	}
	call.Header.Set("Authorization", token)
	call.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(call)
	if err != nil {
		return bizerr.WrapCode(err, CodeUnavailable)
	}
	raw, readErr := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	closeErr := response.Body.Close()
	if readErr != nil {
		return bizerr.WrapCode(readErr, CodeUnavailable)
	}
	if closeErr != nil {
		return bizerr.WrapCode(closeErr, CodeUnavailable)
	}
	if response.StatusCode != http.StatusOK || len(raw) > 1<<20 {
		return bizerr.NewCode(CodeInvalid)
	}
	var envelope struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err = json.Unmarshal(raw, &envelope); err != nil {
		return bizerr.WrapCode(err, CodeUnavailable)
	}
	if envelope.Code != 0 {
		return bizerr.NewCode(CodeInvalid)
	}
	if target == nil {
		return nil
	}
	if err = json.Unmarshal(envelope.Data, target); err != nil {
		return bizerr.WrapCode(err, CodeUnavailable)
	}
	return nil
}
func (s *serviceImpl) profile(ctx context.Context, userID int, token string) (*hostProfile, error) {
	path := "/user/profile"
	if userID > 0 && userID != s.bizctx.Current(ctx).UserID {
		path = "/user/" + strconv.Itoa(userID)
	}
	var profile hostProfile
	if err := s.request(ctx, path, token, &profile); err != nil {
		return nil, err
	}
	if profile.ID <= 0 || profile.TenantID != 0 || userID > 0 && profile.ID != userID {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	return &profile, nil
}
