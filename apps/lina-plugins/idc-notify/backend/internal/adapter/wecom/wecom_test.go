package wecom

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-notify/backend/cap/channelcap"
)

type accessorStub struct{ payload string }

func (s accessorStub) Access(_ context.Context, request secretcap.AccessRequest, consume secretcap.Consumer) error {
	if request.Handle.IsZero() || consume == nil {
		return errors.New("invalid access")
	}
	value := []byte(s.payload)
	defer clear(value)
	return consume(value)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestDeliverUsesOfficialTokenAndSingleRecipientMessage(t *testing.T) {
	requests := 0
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		switch requests {
		case 1:
			if request.URL.Host != "qyapi.weixin.qq.com" || request.URL.Path != "/cgi-bin/gettoken" || request.URL.Query().Get("corpid") != "ww12345678" {
				t.Fatalf("unexpected token request: %s", request.URL.String())
			}
			return jsonResponse(`{"errcode":0,"access_token":"token-value"}`), nil
		case 2:
			body, _ := io.ReadAll(request.Body)
			if request.URL.Path != "/cgi-bin/message/send" || !strings.Contains(string(body), `"touser":"Employee.1"`) ||
				!strings.Contains(string(body), `"agentid":1000001`) || !strings.Contains(string(body), `"content":"hello"`) {
				t.Fatalf("unexpected send request: url=%s body=%s", request.URL.String(), body)
			}
			return jsonResponse(`{"errcode":0,"msgid":"msg-1","invaliduser":"","unlicenseduser":""}`), nil
		default:
			t.Fatal("unexpected extra provider request")
			return nil, nil
		}
	})}
	adapter, err := New(accessorStub{payload: `{"app_secret":"MTIzNDU2Nzg5MGFiY2RlZg=="}`}, client)
	if err != nil {
		t.Fatalf("construct WeCom adapter: %v", err)
	}
	handle, _ := secretcap.NewHandle("sec-wecom-channel")
	got, err := adapter.Deliver(t.Context(), channelcap.Delivery{
		TaskNo: "ntask_1", Credential: handle, Destination: []byte("Employee.1"), Body: "hello",
		NonSecretConfig: map[string]string{corpIDField: "ww12345678", agentIDField: "1000001"},
	})
	if err != nil || got.Class != channelcap.ResultAccepted || !got.CrossedExternalBorder || got.ProviderMessageRef != "msg-1" || len(got.EvidenceDigest) != 64 || requests != 2 {
		t.Fatalf("unexpected delivery result: result=%+v requests=%d err=%v", got, requests, err)
	}
}

func TestDeliverTreatsPartialRecipientRejectionAsInvalid(t *testing.T) {
	client := sequenceClient(
		`{"errcode":0,"access_token":"token-value"}`,
		`{"errcode":0,"msgid":"msg-2","invaliduser":"Employee.1"}`,
	)
	adapter, _ := New(accessorStub{payload: `{"app_secret":"MTIzNDU2Nzg5MGFiY2RlZg=="}`}, client)
	handle, _ := secretcap.NewHandle("sec-wecom-channel")
	got, err := adapter.Deliver(t.Context(), channelcap.Delivery{TaskNo: "ntask_2", Credential: handle,
		Destination: []byte("Employee.1"), Body: "hello",
		NonSecretConfig: map[string]string{corpIDField: "ww12345678", agentIDField: "1000001"}})
	if err != nil || got.Class != channelcap.ResultInvalidRecipient || !got.CrossedExternalBorder {
		t.Fatalf("partial recipient rejection was not fail-closed: result=%+v err=%v", got, err)
	}
}

func TestResolveOAuthUserRequiresEnterpriseMember(t *testing.T) {
	handle, _ := secretcap.NewHandle("sec-wecom-channel")
	configuration := map[string]string{corpIDField: "ww12345678", agentIDField: "1000001"}
	t.Run("member", func(t *testing.T) {
		adapter, _ := New(accessorStub{payload: `{"app_secret":"MTIzNDU2Nzg5MGFiY2RlZg=="}`}, sequenceClient(
			`{"errcode":0,"access_token":"token-value"}`,
			`{"errcode":0,"userid":"Employee.1"}`,
		))
		userID, err := adapter.ResolveOAuthUser(t.Context(), handle, configuration, "single-use-code")
		defer clear(userID)
		if err != nil || string(userID) != "Employee.1" {
			t.Fatalf("resolve enterprise member: user=%q err=%v", userID, err)
		}
	})
	t.Run("external openid rejected", func(t *testing.T) {
		adapter, _ := New(accessorStub{payload: `{"app_secret":"MTIzNDU2Nzg5MGFiY2RlZg=="}`}, sequenceClient(
			`{"errcode":0,"access_token":"token-value"}`,
			`{"errcode":0,"openid":"external-openid"}`,
		))
		if userID, err := adapter.ResolveOAuthUser(t.Context(), handle, configuration, "single-use-code"); err == nil || len(userID) != 0 {
			clear(userID)
			t.Fatalf("external identity was accepted: user=%q err=%v", userID, err)
		}
	})
}

func TestSendTransportFailureIsResultUnknownAndNeverRetried(t *testing.T) {
	requests := 0
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			return jsonResponse(`{"errcode":0,"access_token":"token-value"}`), nil
		}
		return nil, errors.New("connection reset")
	})}
	adapter, _ := New(accessorStub{payload: `{"app_secret":"MTIzNDU2Nzg5MGFiY2RlZg=="}`}, client)
	handle, _ := secretcap.NewHandle("sec-wecom-channel")
	got, err := adapter.Deliver(t.Context(), channelcap.Delivery{TaskNo: "ntask_3", Credential: handle,
		Destination: []byte("Employee.1"), Body: "hello",
		NonSecretConfig: map[string]string{corpIDField: "ww12345678", agentIDField: "1000001"}})
	if err != nil || got.Class != channelcap.ResultUnknown || !got.CrossedExternalBorder || requests != 2 {
		t.Fatalf("ambiguous send was retried or misclassified: result=%+v requests=%d err=%v", got, requests, err)
	}
}

func sequenceClient(bodies ...string) *http.Client {
	index := 0
	return &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		if index >= len(bodies) {
			return nil, errors.New("unexpected request")
		}
		response := jsonResponse(bodies[index])
		index++
		return response, nil
	})}
}

func jsonResponse(body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}
