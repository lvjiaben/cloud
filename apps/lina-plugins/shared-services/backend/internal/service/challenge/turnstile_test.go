package challenge

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func TestVerifyTurnstileRequiresExpectedAction(t *testing.T) {
	service := &serviceImpl{
		config: Config{TurnstileSecret: "secret-value", TurnstileURL: turnstileVerifyURL},
		httpClient: &http.Client{Timeout: time.Second, Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(request.Body)
			if err != nil || !strings.Contains(string(body), "secret=secret-value") || !strings.Contains(string(body), "response=browser-token") {
				t.Fatalf("unexpected Turnstile request body: %q err=%v", body, err)
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"success":true,"action":"shared_action","hostname":"example.test"}`))}, nil
		})},
	}
	verified, err := service.verifyTurnstile(context.Background(), "browser-token", "shared_action")
	if err != nil || !verified {
		t.Fatalf("expected successful Turnstile verification: verified=%v err=%v", verified, err)
	}
	verified, err = service.verifyTurnstile(context.Background(), "browser-token", "other_action")
	if err != nil || verified {
		t.Fatalf("mismatched Turnstile action accepted: verified=%v err=%v", verified, err)
	}
}

func TestVerifyTurnstileRejectsOversizedOrTrailingResponses(t *testing.T) {
	for name, responseBody := range map[string]string{
		"oversized": strings.Repeat(" ", 4097),
		"trailing":  `{"success":true,"action":"shared_action"}{}`,
	} {
		t.Run(name, func(t *testing.T) {
			service := &serviceImpl{
				config: Config{TurnstileSecret: "secret-value", TurnstileURL: turnstileVerifyURL},
				httpClient: &http.Client{Timeout: time.Second, Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(responseBody))}, nil
				})},
			}
			if verified, err := service.verifyTurnstile(context.Background(), "browser-token", "shared_action"); err == nil || verified {
				t.Fatalf("invalid Turnstile response was accepted: verified=%v err=%v", verified, err)
			}
		})
	}
}
