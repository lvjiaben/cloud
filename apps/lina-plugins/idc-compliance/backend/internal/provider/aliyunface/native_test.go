// This file verifies the fixed POP contract, callback-scoped credential use,
// authoritative query mapping, and result-unknown replay boundary.

package aliyunface

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/secretcap"
)

type testAccessor struct {
	values   map[string][]byte
	depth    int
	maxDepth int
}

func (a *testAccessor) Access(
	_ context.Context,
	request secretcap.AccessRequest,
	consume secretcap.Consumer,
) error {
	if err := secretcap.ValidateAccess(request, consume); err != nil {
		return err
	}
	value, ok := a.values[request.Handle.OpaqueRef()]
	if !ok {
		return gerror.New("test secret unavailable")
	}
	plaintext := append([]byte(nil), value...)
	a.depth++
	if a.depth > a.maxDepth {
		a.maxDepth = a.depth
	}
	defer func() {
		a.depth--
		clearBytes(plaintext)
	}()
	return consume(plaintext)
}

type testDoer func(*http.Request) (*http.Response, error)

func (f testDoer) Do(request *http.Request) (*http.Response, error) { return f(request) }

type testRoundTripper struct{ doer testDoer }

func (t testRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return t.doer(request)
}

func newTestHTTPClient(doer testDoer) *http.Client {
	return &http.Client{Transport: testRoundTripper{doer: doer}, Timeout: time.Second}
}

func TestNativeClientInitiateUsesFixedFaceContract(t *testing.T) {
	accessor, credential := newTestCredentials(t)
	doer := testDoer(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.URL.String() != Endpoint {
			t.Fatalf("request=%s %s, want POST fixed endpoint", request.Method, request.URL)
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("read signed form failed: %v", err)
		}
		defer clearBytes(body)
		form, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("parse signed form failed: %v", err)
		}
		for key, expected := range map[string]string{
			"AccessKeyId": "test-access-key-id", "Action": aliyunAction,
			"Version": aliyunVersion, "Service": aliyunService,
			"SignatureMethod": "HMAC-SHA1", "Timestamp": "2026-08-12T01:02:03Z",
			"SignatureNonce": "nonce-1",
		} {
			if form.Get(key) != expected {
				t.Errorf("form[%s]=%q want=%q", key, form.Get(key), expected)
			}
		}
		if form.Get("Signature") == "" || strings.Contains(form.Get("Signature"), "test-access-key-secret") {
			t.Fatal("POP signature was missing or exposed the AccessKey secret")
		}
		service := form.Get("ServiceParameters")
		for _, expected := range []string{
			`"method":"init"`, `"bizCode":"FACE"`, `"identityType":"CERT_INFO"`,
			`"certType":"IDENTITY_CARD"`, `"sceneId":"100000001"`,
			`"outerOrderNo":"0123456789abcdef0123456789abcdef"`,
		} {
			if !strings.Contains(service, expected) {
				t.Errorf("service parameters missing %s", expected)
			}
		}
		if strings.Contains(service, "facePhoto") || strings.Contains(service, "faceVideo") {
			t.Fatal("FACE initialization unexpectedly submitted biometric media")
		}
		return jsonResponse(200, `{"code":200,"requestId":"request-1","data":{"certifyId":"certify-1","certifyUrl":"https://picker.antcloudauth.aliyuncs.com/gateway.do?token=opaque"}}`), nil
	})
	client, err := New(
		accessor, credential, newTestHTTPClient(doer),
		func() time.Time { return time.Date(2026, 8, 12, 1, 2, 3, 0, time.UTC) },
		func() (string, error) { return "nonce-1", nil },
	)
	if err != nil {
		t.Fatalf("construct native client failed: %v", err)
	}
	result, err := client.Initiate(context.Background(), InitiateRequest{
		SceneID: "100000001", OuterOrderNo: "0123456789abcdef0123456789abcdef",
		CertType: CertTypeIdentityCard, CertName: "测试用户", CertNo: "110105199001011234",
		ReturnURL: "https://merchant.example.com/account/compliance/return",
	})
	if err != nil {
		t.Fatalf("initiate FACE failed: %v", err)
	}
	if result.CertifyID != "certify-1" || !strings.HasPrefix(result.CertifyURL, "https://") {
		t.Fatalf("unexpected initiation result: %#v", result)
	}
	if accessor.maxDepth != 1 {
		t.Fatalf("credential callback max depth=%d want=1", accessor.maxDepth)
	}
}

func TestNativeClientQueryKeepsNotPassedNonTerminal(t *testing.T) {
	accessor, credential := newTestCredentials(t)
	doer := testDoer(func(request *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("read query form failed: %v", err)
		}
		defer clearBytes(body)
		form, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("parse query form failed: %v", err)
		}
		service := form.Get("ServiceParameters")
		if !strings.Contains(service, `"method":"query"`) ||
			strings.Contains(service, "encToken") {
			t.Fatalf("query contract was not minimal: %s", service)
		}
		return jsonResponse(200, `{"code":"200","data":{"passed":"F"}}`), nil
	})
	client, err := New(accessor, credential, newTestHTTPClient(doer), time.Now, func() (string, error) {
		return "nonce-query", nil
	})
	if err != nil {
		t.Fatalf("construct native client failed: %v", err)
	}
	result, err := client.Query(context.Background(), QueryRequest{
		SceneID: "100000001", CertifyID: "certify-1",
	})
	if err != nil || result.State != QueryStateNotPassed {
		t.Fatalf("query result=%#v err=%v, want non-terminal not_passed", result, err)
	}
}

func TestNativeClientQueryMapsProviderExpiry(t *testing.T) {
	accessor, credential := newTestCredentials(t)
	doer := testDoer(func(*http.Request) (*http.Response, error) {
		return jsonResponse(200, `{"code":407,"message":"must not be retained"}`), nil
	})
	client, err := New(accessor, credential, newTestHTTPClient(doer), time.Now, func() (string, error) {
		return "nonce-expiry", nil
	})
	if err != nil {
		t.Fatalf("construct native client failed: %v", err)
	}
	result, err := client.Query(context.Background(), QueryRequest{
		SceneID: "100000001", CertifyID: "certify-1",
	})
	if err != nil || result.State != QueryStateExpired {
		t.Fatalf("query result=%#v err=%v, want expired", result, err)
	}
}

func TestNativeClientInitializationTransportFailureIsUnknown(t *testing.T) {
	accessor, credential := newTestCredentials(t)
	doer := testDoer(func(*http.Request) (*http.Response, error) {
		return nil, gerror.New("synthetic transport failure")
	})
	client, err := New(accessor, credential, newTestHTTPClient(doer), time.Now, func() (string, error) {
		return "nonce-transport", nil
	})
	if err != nil {
		t.Fatalf("construct native client failed: %v", err)
	}
	_, err = client.Initiate(context.Background(), InitiateRequest{
		SceneID: "100000001", OuterOrderNo: "0123456789abcdef0123456789abcdef",
		CertType: CertTypeIdentityCard, CertName: "测试用户", CertNo: "110105199001011234",
		ReturnURL: "https://merchant.example.com/account/compliance/return",
	})
	callError, ok := err.(*CallError)
	if !ok || callError.Code != ErrorResultUnknown || !callError.Unknown {
		t.Fatalf("transport error=%#v, want non-replayable result unknown", err)
	}
}

func TestNativeClientRejectsUnsafeCertificationURL(t *testing.T) {
	accessor, credential := newTestCredentials(t)
	doer := testDoer(func(*http.Request) (*http.Response, error) {
		return jsonResponse(200, `{"code":200,"data":{"certifyId":"certify-1","certifyUrl":"javascript:alert(1)"}}`), nil
	})
	client, err := New(accessor, credential, newTestHTTPClient(doer), time.Now, func() (string, error) {
		return "nonce-url", nil
	})
	if err != nil {
		t.Fatalf("construct native client failed: %v", err)
	}
	_, err = client.Initiate(context.Background(), InitiateRequest{
		SceneID: "100000001", OuterOrderNo: "0123456789abcdef0123456789abcdef",
		CertType: CertTypeIdentityCard, CertName: "测试用户", CertNo: "110105199001011234",
		ReturnURL: "https://merchant.example.com/account/compliance/return",
	})
	callError, ok := err.(*CallError)
	if !ok || callError.Code != ErrorInvalidResponse || !callError.Unknown {
		t.Fatalf("unsafe URL error=%#v, want invalid unknown response", err)
	}
}

func TestNativeClientProbeTreatsInvalidSyntheticCertifyIDAsReachable(t *testing.T) {
	accessor, credential := newTestCredentials(t)
	doer := testDoer(func(request *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatal(err)
		}
		form, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatal(err)
		}
		service := form.Get("ServiceParameters")
		if !strings.Contains(service, `"method":"query"`) ||
			strings.Contains(service, `"method":"init"`) {
			t.Fatalf("probe performed an unsafe operation: %s", service)
		}
		return jsonResponse(200, `{"code":406,"message":"must not be retained"}`), nil
	})
	client, err := New(accessor, credential, newTestHTTPClient(doer), time.Now, func() (string, error) {
		return "nonce-probe-reachable", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome := client.Probe(context.Background(), "100000001"); outcome != ProbeOutcomeReachable {
		t.Fatalf("probe outcome=%q want reachable", outcome)
	}
}

func TestNativeClientProbeClassifiesCredentialAndTransportFailures(t *testing.T) {
	tests := []struct {
		name    string
		doer    testDoer
		outcome ProbeOutcome
	}{
		{
			name: "credential rejected",
			doer: func(*http.Request) (*http.Response, error) {
				return jsonResponse(403, `{"Code":"SignatureDoesNotMatch","Message":"secret"}`), nil
			},
			outcome: ProbeOutcomeRejected,
		},
		{
			name: "transport unavailable",
			doer: func(*http.Request) (*http.Response, error) {
				return nil, gerror.New("synthetic transport failure")
			},
			outcome: ProbeOutcomeUnavailable,
		},
		{
			name: "provider unavailable",
			doer: func(*http.Request) (*http.Response, error) {
				return jsonResponse(503, `{"code":503,"message":"must not be retained"}`), nil
			},
			outcome: ProbeOutcomeUnavailable,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			accessor, credential := newTestCredentials(t)
			client, err := New(accessor, credential, newTestHTTPClient(testCase.doer), time.Now, func() (string, error) {
				return "nonce-probe-failure", nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if outcome := client.Probe(context.Background(), "100000001"); outcome != testCase.outcome {
				t.Fatalf("probe outcome=%q want %q", outcome, testCase.outcome)
			}
		})
	}
}

func TestNewDisablesRedirectsAndRequiresBoundedTimeout(t *testing.T) {
	accessor, credential := newTestCredentials(t)
	if _, err := New(accessor, credential, &http.Client{}, time.Now, func() (string, error) {
		return "nonce-timeout", nil
	}); err == nil {
		t.Fatal("client without a timeout was accepted")
	}
	httpClient := &http.Client{
		Timeout: time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			t.Fatal("caller redirect policy was retained")
			return nil
		},
	}
	client, err := New(accessor, credential, httpClient, time.Now, func() (string, error) {
		return "nonce-redirect", nil
	})
	if err != nil {
		t.Fatalf("construct production client failed: %v", err)
	}
	native, ok := client.(*nativeClient)
	if !ok {
		t.Fatalf("client type=%T want native client", client)
	}
	bounded, ok := native.httpClient.(*http.Client)
	if !ok || bounded == httpClient || bounded.CheckRedirect == nil {
		t.Fatal("production client did not install an isolated redirect policy")
	}
	request, err := http.NewRequest(http.MethodGet, "https://example.com/next", nil)
	if err != nil {
		t.Fatal(err)
	}
	if redirectErr := bounded.CheckRedirect(request, nil); redirectErr != http.ErrUseLastResponse {
		t.Fatalf("redirect error=%v want ErrUseLastResponse", redirectErr)
	}
}

func newTestCredentials(t *testing.T) (*testAccessor, secretcap.Handle) {
	t.Helper()
	credential, err := secretcap.NewHandle("test-access-key-credential-handle")
	if err != nil {
		t.Fatalf("construct AccessKey credential handle failed: %v", err)
	}
	return &testAccessor{values: map[string][]byte{
		credential.OpaqueRef(): []byte(`{"accessKeyId":"dGVzdC1hY2Nlc3Mta2V5LWlk","accessKeySecret":"dGVzdC1hY2Nlc3Mta2V5LXNlY3JldA=="}`),
	}}, credential
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
