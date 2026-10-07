package supportupstream

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestNewRejectsUnsafeEndpoint(t *testing.T) {
	t.Parallel()
	for _, endpoint := range []string{"http://example.com", "https://127.0.0.1", "https://example.com?secret=1"} {
		if _, err := New(endpoint, "zjmf_api_login"); err == nil {
			t.Fatalf("New(%q) accepted an unsafe endpoint", endpoint)
		}
	}
	if _, err := New("https://example.com", "unsupported"); err == nil {
		t.Fatal("New accepted an unsupported authentication mode")
	}
}

func TestHTTPClientCreateAndDetail(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/zjmf_api_login":
			_, _ = response.Write([]byte(`{"status":200,"jwt":"signed-token"}`))
		case "/ticket/create":
			if request.Header.Get("Authorization") != "Bearer signed-token" {
				response.WriteHeader(http.StatusUnauthorized)
				return
			}
			_ = request.ParseForm()
			if request.Form.Get("dptid") != "9" || request.Form.Get("title") != "Disk issue" {
				response.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = response.Write([]byte(`{"status":200,"data":{"tid":321}}`))
		case "/ticket/detail":
			if request.URL.Query().Get("tid") != "321" {
				response.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = response.Write([]byte(`{"status":200,"data":{"list":[{"id":1,"type":"q","content":"customer","time":10},{"id":2,"type":"r","content":"admin reply","admin_id":5,"time":20},{"id":3,"type":"r","content":"customer follow-up","admin_id":0,"time":30}]}}`))
		default:
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	base, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	client := &HTTPClient{base: base, authMode: "zjmf_api_login", client: server.Client()}
	created := client.Create(context.Background(), Credentials{Username: "merchant", Password: "secret"}, CreateRequest{DepartmentID: 9, Title: "Disk issue", Content: "details", Priority: "normal"})
	if created.Classification != ResultAccepted || created.TicketReference != "321" {
		t.Fatalf("Create() = %#v", created)
	}
	replies, result := client.Detail(context.Background(), Credentials{Username: "merchant", Password: "secret"}, created.TicketReference)
	if result.Classification != ResultAccepted || len(replies) != 1 || replies[0].Reference != "2" || strings.TrimSpace(replies[0].Content) != "admin reply" {
		t.Fatalf("Detail() replies = %#v, result = %#v", replies, result)
	}
}

func TestHTTPClientDoesNotClassifyUnknownAsRetryable(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/zjmf_api_login" {
			_, _ = response.Write([]byte(`{"status":200,"jwt":"signed-token"}`))
			return
		}
		response.WriteHeader(http.StatusGatewayTimeout)
		_, _ = response.Write([]byte(`{"status":504}`))
	}))
	defer server.Close()
	base, _ := url.Parse(server.URL)
	client := &HTTPClient{base: base, authMode: "zjmf_api_login", client: server.Client()}
	result := client.Create(context.Background(), Credentials{Username: "merchant", Password: "secret"}, CreateRequest{DepartmentID: 9, Title: "Issue", Content: "details"})
	if result.Classification != ResultUnknown {
		t.Fatalf("Create() classification = %q, want %q", result.Classification, ResultUnknown)
	}
}
