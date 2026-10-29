// These self-contained HTTP tests verify diagnostic facts, permissions, bounded
// requests, and secret-free output without reaching a database or a supplier.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// serveData wraps fixture data using the normal local API envelope.
func serveData(t *testing.T, w http.ResponseWriter, data any) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": data}); err != nil {
		t.Error(err)
	}
}

func TestDiagnoseUnknownResultAndSecrets(t *testing.T) {
	const task = "task_0123456789abcdef0123456789abcdef"
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer PRIVATE_TOKEN" {
			t.Error("write or missing credential")
		}
		switch r.URL.Path {
		case coreAPI + "commerce/orders/ORD_1":
			serveData(t, w, map[string]any{"order": map[string]any{"summary": map[string]any{"orderNo": "ORD_1", "statusCode": "paid", "customerPrimaryPhone": "PRIVATE_PHONE"}, "invoice": map[string]any{"statusCode": "settled", "currencyCode": "CNY", "totalAmountMinor": "1100", "paidAmountMinor": "1100"}, "password": "PRIVATE_PASSWORD"}})
		case coreAPI + "product-fulfillment/tasks":
			if r.URL.Query().Get("orderNo") != "ORD_1" || r.URL.Query().Get("limit") != "3" {
				t.Error("unbounded order filter")
			}
			serveData(t, w, map[string]any{"total": 1, "tasks": []any{map[string]any{"orderNo": "ORD_1", "intentNo": "pft_1", "statusCode": "processing"}}})
		case coreAPI + "product-fulfillment/tasks/pft_1":
			serveData(t, w, map[string]any{"task": map[string]any{"orderNo": "ORD_1", "intentNo": "pft_1", "execution": map[string]any{"resourceTaskNo": task, "statusCode": "manual_pending", "phaseCode": "query", "lastResultCode": "upstream_result_unknown", "attemptCount": 2}, "events": []any{map[string]any{"safeNote": "PRIVATE_NOTE"}}}})
		case resourceAPI + task:
			serveData(t, w, map[string]any{"item": map[string]any{"task": map[string]any{"taskNo": task, "status": "result_unknown", "resultUnknown": true}, "upstream": map[string]any{"externalTaskRef": "9001", "externalResourceRef": "", "statusCode": "unknown"}, "password": "PRIVATE_ROOT_PASSWORD"}})
		default:
			t.Errorf("unexpected API %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(server.Close)
	client, err := newDiagnosticClient(server.URL, "PRIVATE_TOKEN", "")
	if err != nil {
		t.Fatal(err)
	}
	result := client.diagnose(context.Background(), "ORD_1")
	if !result.Complete || requests != 4 || result.Requests != 4 || result.Invoice.PaidAmountMinor != "1100" || len(result.Tasks) != 1 || result.Tasks[0].Resource.Upstream.ExternalTaskRef != "9001" || !strings.Contains(result.Conclusion, "The upstream result is unknown") {
		t.Fatalf("incorrect facts: %+v", result)
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("PRIVATE_")) {
		t.Fatalf("leaked private fields: %s", data)
	}
}

func TestDiagnosePartialAndRequestBudget(t *testing.T) {
	for _, denied := range []bool{false, true} {
		t.Run(fmt.Sprint(denied), func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				switch {
				case strings.Contains(r.URL.Path, "commerce/orders/"):
					serveData(t, w, map[string]any{"order": map[string]any{"summary": map[string]any{"orderNo": "ORD_1", "statusCode": "paid"}}})
				case r.URL.Path == coreAPI+"product-fulfillment/tasks":
					if denied {
						w.WriteHeader(403)
						fmt.Fprint(w, "PRIVATE_DENIAL")
						return
					}
					tasks := []any{}
					for i := 0; i < 6; i++ {
						tasks = append(tasks, map[string]any{"orderNo": "ORD_1", "intentNo": fmt.Sprintf("pft_%d", i), "statusCode": "processing"})
					}
					serveData(t, w, map[string]any{"total": 6, "tasks": tasks, "nextCursor": "PRIVATE_CURSOR"})
				case strings.HasPrefix(r.URL.Path, coreAPI+"product-fulfillment/tasks/"):
					intent := strings.TrimPrefix(r.URL.Path, coreAPI+"product-fulfillment/tasks/")
					index := strings.TrimPrefix(intent, "pft_")
					task := fmt.Sprintf("task_%032s", index)
					serveData(t, w, map[string]any{"task": map[string]any{"orderNo": "ORD_1", "intentNo": intent, "execution": map[string]any{"resourceTaskNo": task, "statusCode": "processing"}}})
				case strings.HasPrefix(r.URL.Path, resourceAPI):
					serveData(t, w, map[string]any{"item": map[string]any{"task": map[string]any{"taskNo": strings.TrimPrefix(r.URL.Path, resourceAPI), "status": "running"}}})
				default:
					t.Error("unexpected request")
					w.WriteHeader(404)
				}
			}))
			t.Cleanup(server.Close)
			client, err := newDiagnosticClient(server.URL, "token", "")
			if err != nil {
				t.Fatal(err)
			}
			r := client.diagnose(context.Background(), "ORD_1")
			if r.Complete || len(r.Issues) == 0 || requests > 8 || (!denied && requests != 8) {
				t.Fatalf("unbounded or false complete: %+v calls=%d", r, requests)
			}
			data, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(data, []byte("PRIVATE_")) {
				t.Fatal("leaked body or cursor")
			}
		})
	}
}

func TestHTTPRejectionsAndIdentity(t *testing.T) {
	for _, kind := range []string{"unauthorized", "redirect", "oversized", "invalid", "wrong_order", "cancelled"} {
		t.Run(kind, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				switch kind {
				case "unauthorized":
					w.WriteHeader(401)
					fmt.Fprint(w, "PRIVATE_TOKEN")
				case "redirect":
					http.Redirect(w, r, "http://example.invalid/PRIVATE", 302)
				case "oversized":
					fmt.Fprint(w, strings.Repeat("x", maxBody+1))
				case "invalid":
					fmt.Fprint(w, "PRIVATE_RAW_ERROR")
				case "wrong_order":
					serveData(t, w, map[string]any{"order": map[string]any{"summary": map[string]any{"orderNo": "OTHER"}}})
				default:
					w.WriteHeader(500)
				}
			}))
			t.Cleanup(server.Close)
			client, err := newDiagnosticClient(server.URL, "token", "")
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if kind == "cancelled" {
				cancel()
			}
			r := client.diagnose(ctx, "ORD_1")
			data, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			if r.Complete || calls > 1 || bytes.Contains(data, []byte("PRIVATE")) {
				t.Fatalf("rejection failed: %+v", r)
			}
		})
	}
}

func TestCLIAndCredentials(t *testing.T) {
	for _, base := range []string{"https://example.com", "http://127.0.0.1/path", "http://user:pass@localhost", "http://localhost/?token=secret", "file:///tmp"} {
		if _, err := newDiagnosticClient(base, "token", ""); err == nil {
			t.Fatalf("accepted %s", base)
		}
	}
	if _, err := newDiagnosticClient("http://localhost", "", ""); err == nil {
		t.Fatal("accepted missing token")
	}
	p := filepath.Join(t.TempDir(), "session.json")
	if err := os.WriteFile(p, []byte(`{"accessToken":"FILE_TOKEN"}`), 0600); err != nil {
		t.Fatal(err)
	}
	client, err := newDiagnosticClient("http://localhost", "", p)
	if err != nil || client.token != "FILE_TOKEN" {
		t.Fatal("session not read")
	}
	var out, errors bytes.Buffer
	if err := runCLI([]string{"--order", "../escape"}, &out, &errors); err == nil {
		t.Fatal("accepted invalid order")
	}
	if err := runCLI([]string{"--help"}, &out, &errors); err != nil {
		t.Fatal(err)
	}
}

func TestMalformedProjectedCodesAreIncomplete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "commerce/orders/") {
			serveData(t, w, map[string]any{"order": map[string]any{"summary": map[string]any{"orderNo": "ORD_1", "statusCode": "PRIVATE_RAW\nMESSAGE"}}})
		} else {
			serveData(t, w, map[string]any{"tasks": []any{}, "total": 0})
		}
	}))
	t.Cleanup(server.Close)
	client, err := newDiagnosticClient(server.URL, "token", "")
	if err != nil {
		t.Fatal(err)
	}
	r := client.diagnose(context.Background(), "ORD_1")
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if r.Complete || len(r.Issues) == 0 || bytes.Contains(data, []byte("PRIVATE_RAW")) {
		t.Fatalf("malformed projection accepted: %+v", r)
	}
}
