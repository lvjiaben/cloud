// This file verifies login tokens and interface secrets remain inside one
// bounded host-owned supplier session.

package productautomation

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/supplierinterfacecap"
)

type outboundSessionDoer struct {
	t     *testing.T
	calls int
}

func (d *outboundSessionDoer) Do(request *http.Request) (*http.Response, error) {
	d.t.Helper()
	d.calls++
	body, err := io.ReadAll(request.Body)
	if err != nil {
		d.t.Fatal(err)
	}
	switch d.calls {
	case 1:
		if request.URL.Path != "/api/login" || string(body) != "password=p%40ss&username=merchant" || request.Header.Get("Access-Token") != "" {
			d.t.Fatalf("unexpected login request path=%s body=%s headers=%v", request.URL.Path, body, request.Header)
		}
		return sessionHTTPResponse(http.StatusOK, `"session-token"`), nil
	case 2:
		if request.URL.Path != "/api/clouds/create_page" || request.Header.Get("Access-Token") != "session-token" {
			d.t.Fatalf("unexpected first step path=%s token=%q", request.URL.Path, request.Header.Get("Access-Token"))
		}
		return sessionHTTPResponse(http.StatusOK, `{"data":{"region":"cn"},"ignored":"drop"}`), nil
	case 3:
		if request.URL.Path != "/api/clouds" || request.Header.Get("Access-Token") != "session-token" || string(body) != "plan=small&rootpass=temporary-pass" {
			d.t.Fatalf("unexpected second step path=%s token=%q body=%s", request.URL.Path, request.Header.Get("Access-Token"), body)
		}
		return sessionHTTPResponse(http.StatusAccepted, `{"id":"1001","status":"pending","password":"never-return"}`), nil
	default:
		return nil, gerror.New("unexpected supplier request")
	}
}

func TestSupplierOutboundSessionLogsInOnceAndReturnsOnlyStepProjections(t *testing.T) {
	t.Parallel()
	base, err := url.Parse("https://supplier.example/api")
	if err != nil {
		t.Fatal(err)
	}
	doer := &outboundSessionDoer{t: t}
	secrets := map[string][]byte{"username": []byte("merchant"), "password": []byte("p@ss"), "root_password": []byte("temporary-pass")}
	request := supplierinterfacecap.OutboundSessionRequest{
		EndpointField: "endpoint",
		Authentication: supplierinterfacecap.OutboundSessionAuthentication{
			Method: http.MethodPost, Path: "/login",
			SecretFormFields: map[string]string{"username": "username", "password": "password"},
			TokenSelector:    supplierinterfacecap.OutboundTokenRootString, TokenHeader: "Access-Token",
		},
		Steps: []supplierinterfacecap.OutboundSessionStep{{
			Method: http.MethodGet, Path: "/clouds/create_page", ResponseFields: []string{"data", "region"},
		}, {
			Method: http.MethodPost, Path: "/clouds", Form: map[string]string{"plan": "small"},
			SecretFormFields: map[string]string{"rootpass": "root_password"}, ResponseFields: []string{"id", "status"},
		}},
	}
	result, err := executeSupplierOutboundSession(context.Background(), doer, base, request, func(field string, consume func([]byte) error) error {
		value, ok := secrets[field]
		if !ok {
			return gerror.New("missing test secret")
		}
		return consume(value)
	})
	if err != nil || doer.calls != 3 || len(result.Steps) != 2 {
		t.Fatalf("session result=%#v calls=%d err=%v", result, doer.calls, err)
	}
	if string(result.Steps[0].Data) != `{"data":{"region":"cn"}}` || string(result.Steps[1].Data) != `{"id":"1001","status":"pending"}` {
		t.Fatalf("unsafe or incomplete projections: %#v", result.Steps)
	}
}

func TestSupplierOutboundSessionReadsSingleTokenFieldAndRejectsTokenEcho(t *testing.T) {
	t.Parallel()
	base, _ := url.Parse("https://supplier.example")
	calls := 0
	doer := sessionScriptDoer{do: func(request *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return sessionHTTPResponse(http.StatusOK, `{"access_token":"field-token","expires":300}`), nil
		}
		if request.Header.Get("Authorization") != "field-token" {
			t.Fatalf("field token was not applied")
		}
		return sessionHTTPResponse(http.StatusOK, `{"status":"ok","detail":"field-token"}`), nil
	}}
	request := supplierinterfacecap.OutboundSessionRequest{
		EndpointField: "endpoint",
		Authentication: supplierinterfacecap.OutboundSessionAuthentication{
			Method: http.MethodPost, Path: "/login", SecretFormFields: map[string]string{"password": "password"},
			TokenSelector: supplierinterfacecap.OutboundTokenField, TokenField: "access_token", TokenHeader: "Authorization",
		},
		Steps: []supplierinterfacecap.OutboundSessionStep{{Method: http.MethodGet, Path: "/status", ResponseFields: []string{"status", "detail"}}},
	}
	_, err := executeSupplierOutboundSession(context.Background(), &doer, base, request, func(_ string, consume func([]byte) error) error {
		return consume([]byte("secret-value"))
	})
	if err == nil || !strings.Contains(err.Error(), "credential material") {
		t.Fatalf("token echo was not rejected: %v", err)
	}
}

func sessionHTTPResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

type sessionScriptDoer struct {
	do func(*http.Request) (*http.Response, error)
}

func (s *sessionScriptDoer) Do(request *http.Request) (*http.Response, error) { return s.do(request) }
