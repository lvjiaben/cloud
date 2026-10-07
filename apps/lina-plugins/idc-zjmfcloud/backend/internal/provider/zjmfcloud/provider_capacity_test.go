// This file verifies Provider capacity pagination, projection, partial failure, and circuit behavior.

package zjmfcloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func TestConcurrentProviderCapacityUsesOneLoginAndWhitelistsFields(t *testing.T) {
	t.Parallel()
	var loginCount int
	var loginMu sync.Mutex
	login := loginHandler(t, &loginCount, &loginMu, "token-one")
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/administrator/v1/login":
			login(writer, request)
		case "/administrator/v1/areas":
			if request.Header.Get("access-token") != "token-one" {
				t.Error("providerCapacity request did not use the cached access token")
			}
			writeJSON(t, writer, http.StatusOK, `{
                    "data":[{
                        "id":101,"name":"Region One","country":"CN","status":"active",
                        "ip_total":100,"ip_used":20,"password":"must-not-leak",
                        "username":"must-not-leak","vnc_password":"must-not-leak"
                    }],
                    "meta":{"page":1,"per_page":10,"total_page":1,"total":1}
                }`)
		default:
			writeJSON(t, writer, http.StatusNotFound, `{"error":"not found"}`)
		}
	}))

	const goroutines = 24
	var wait sync.WaitGroup
	wait.Add(goroutines)
	errorsByCall := make(chan error, goroutines)
	for range goroutines {
		go func() {
			defer wait.Done()
			page, err := fixture.provider.ListProviderCapacity(context.Background(), resourcecap.ProviderCapacityRequest{
				Kind: resourcecap.ProviderCapacityKindRegion, Limit: 10,
			})
			if err != nil {
				errorsByCall <- err
				return
			}
			if len(page.Items) != 1 || !page.Complete {
				errorsByCall <- fmt.Errorf("unexpected providerCapacity page")
				return
			}
			var projected map[string]any
			if json.Unmarshal(page.Items[0].Data, &projected) != nil {
				errorsByCall <- fmt.Errorf("decode projected data")
				return
			}
			for _, forbidden := range []string{"password", "username", "vnc_password"} {
				if _, exists := projected[forbidden]; exists {
					errorsByCall <- fmt.Errorf("forbidden field %s survived projection", forbidden)
				}
			}
		}()
	}
	wait.Wait()
	close(errorsByCall)
	for err := range errorsByCall {
		t.Error(err)
	}
	loginMu.Lock()
	gotLoginCount := loginCount
	loginMu.Unlock()
	if gotLoginCount != 1 {
		t.Fatalf("login count=%d, want 1", gotLoginCount)
	}
	fixture.secrets.mu.Lock()
	usernameReads := fixture.secrets.reads[fixture.provider.username.OpaqueRef()]
	passwordReads := fixture.secrets.reads[fixture.provider.password.OpaqueRef()]
	fixture.secrets.mu.Unlock()
	if usernameReads != 1 || passwordReads != 1 {
		t.Fatalf("secret reads username=%d password=%d, want one each", usernameReads, passwordReads)
	}
}

func TestProviderCapacityPaginationUsesOpaqueAdvancingCursor(t *testing.T) {
	t.Parallel()
	var loginCount int
	var mu sync.Mutex
	login := loginHandler(t, &loginCount, &mu, "token-one")
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/administrator/v1/login" {
			login(writer, request)
			return
		}
		page, _ := strconv.Atoi(request.URL.Query().Get("page"))
		if request.URL.Query().Get("per_page") != "1" {
			t.Error("providerCapacity did not preserve the bounded page size")
		}
		writeJSON(t, writer, http.StatusOK, fmt.Sprintf(
			`{"data":[{"id":%d,"name":"Region"}],"meta":{"page":%d,"total_page":2}}`,
			page, page,
		))
	}))
	first, err := fixture.provider.ListProviderCapacity(context.Background(), resourcecap.ProviderCapacityRequest{
		Kind: resourcecap.ProviderCapacityKindRegion, Limit: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Complete || first.NextCursor != "v1:region:1:2" {
		t.Fatalf("first page boundary=%+v", first)
	}
	secondRequest := resourcecap.ProviderCapacityRequest{
		Kind: resourcecap.ProviderCapacityKindRegion, Limit: 1, Cursor: first.NextCursor,
	}
	second, err := fixture.provider.ListProviderCapacity(context.Background(), secondRequest)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Complete || second.NextCursor != "" || second.Items[0].ExternalRef != "2" {
		t.Fatalf("second page boundary=%+v", second)
	}
}

func TestProviderCapacityCursorIsBoundToKindAndLimitBeforeNetwork(t *testing.T) {
	t.Parallel()
	var requests atomic.Int64
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		writeJSON(t, writer, http.StatusInternalServerError, `{}`)
	}))
	for _, request := range []resourcecap.ProviderCapacityRequest{
		{Kind: resourcecap.ProviderCapacityKindLine, Limit: 1, Cursor: "v1:region:1:2"},
		{Kind: resourcecap.ProviderCapacityKindRegion, Limit: 2, Cursor: "v1:region:1:2"},
		{Kind: resourcecap.ProviderCapacityKindRegion, Limit: 1, Cursor: "page:2"},
		{Kind: resourcecap.ProviderCapacityKindPlan, Limit: 1, Cursor: "v1:region:1:2"},
	} {
		if _, err := fixture.provider.ListProviderCapacity(context.Background(), request); err == nil {
			t.Errorf("cursor %q was accepted for kind=%s limit=%d", request.Cursor, request.Kind, request.Limit)
		}
	}
	if requests.Load() != 0 {
		t.Fatalf("invalid cursors made %d network requests", requests.Load())
	}
}

func TestPlanProviderCapacityProjectsCreationGraphAndPinsSnapshotAcrossPages(t *testing.T) {
	t.Parallel()
	var loginCount int
	var mu sync.Mutex
	login := loginHandler(t, &loginCount, &mu, "token-one")
	var graphCalls atomic.Int64
	var storeCalls atomic.Int64
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/administrator/v1/login" {
			login(writer, request)
			return
		}
		if request.Method != http.MethodGet {
			t.Errorf("unexpected creation providerCapacity method %s", request.Method)
		}
		switch request.URL.Path {
		case "/administrator/v1/clouds/create_page":
			if request.URL.RawQuery != "" {
				t.Errorf("unexpected creation graph query %s", request.URL.RawQuery)
			}
			graphCalls.Add(1)
			writeJSON(t, writer, http.StatusOK, `{
			"node_type":"kvm",
			"data":[{
				"id":9,"name":"Region Nine","password":"discard-me",
				"store":[{
					"id":21,"show_name":"Shared Store","status":1,"free":100,"type":"ceph",
					"api_key":"discard-me"
				}],
				"node":[{
					"id":11,"name":"Node Eleven","status":1,"ssh_password":"discard-me",
					"store":[{
						"id":22,"show_name":"Node Store","status":1,"free":80,"type":"local",
						"secret":"discard-me"
					}]
				}]
			}]
		}`)
		case "/administrator/v1/stores":
			if request.URL.Query().Get("list_type") != "all" ||
				request.URL.Query().Get("enable") != "1" || len(request.URL.Query()) != 2 {
				t.Errorf("unexpected top-level storage query %s", request.URL.RawQuery)
			}
			storeCalls.Add(1)
			writeJSON(t, writer, http.StatusOK, `{"data":[{
				"id":31,"name":"Global Store","status":1,"type":"ceph","free":50,
				"password":"discard-me"
			}]}`)
		default:
			t.Errorf("unexpected creation providerCapacity path %s", request.URL.Path)
			writeJSON(t, writer, http.StatusNotFound, `{}`)
		}
	}))
	firstRequest := resourcecap.ProviderCapacityRequest{Kind: resourcecap.ProviderCapacityKindPlan, Limit: 2}
	first, err := fixture.provider.ListProviderCapacity(context.Background(), firstRequest)
	if err != nil {
		t.Fatal(err)
	}
	if first.Complete || len(first.Items) != 2 ||
		!strings.HasPrefix(first.NextCursor, "v1:plan:2:2:") {
		t.Fatalf("unexpected first creation graph page: %+v", first)
	}
	secondRequest := resourcecap.ProviderCapacityRequest{
		Kind: resourcecap.ProviderCapacityKindPlan, Limit: 2, Cursor: first.NextCursor,
	}
	second, err := fixture.provider.ListProviderCapacity(context.Background(), secondRequest)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Complete || second.NextCursor != "" || len(second.Items) != 1 {
		t.Fatalf("unexpected second creation graph page: %+v", second)
	}
	seenGlobal := false
	for _, item := range append(first.Items, second.Items...) {
		var projected map[string]any
		if err := json.Unmarshal(item.Data, &projected); err != nil {
			t.Fatalf("decode plan projection: %v", err)
		}
		if projected["candidate_type"] != "storage" || projected["store_id"] == nil ||
			projected["store_status"] == nil {
			t.Fatalf("incomplete creation candidate: %v", projected)
		}
		if item.ExternalRef == "global:store:31" && projected["source_scope"] == "global" &&
			projected["store_name"] == "Global Store" {
			seenGlobal = true
		}
		for _, forbidden := range []string{"password", "ssh_password", "api_key", "secret"} {
			if _, exists := projected[forbidden]; exists || strings.Contains(string(item.Data), "discard-me") {
				t.Fatalf("creation candidate retained confidential field %q: %s", forbidden, item.Data)
			}
		}
	}
	if !seenGlobal {
		t.Fatal("top-level storage projection was not merged into plan providerCapacity")
	}
	if graphCalls.Load() != 2 || storeCalls.Load() != 2 {
		t.Fatalf("creation providerCapacity calls graph=%d stores=%d, want two of each", graphCalls.Load(), storeCalls.Load())
	}
}

func TestPlanProviderCapacityRejectsSnapshotMutationBetweenPages(t *testing.T) {
	t.Parallel()
	var loginCount int
	var mu sync.Mutex
	login := loginHandler(t, &loginCount, &mu, "token-one")
	var graphCalls atomic.Int64
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/administrator/v1/login" {
			login(writer, request)
			return
		}
		if request.URL.Path == "/administrator/v1/stores" {
			writeJSON(t, writer, http.StatusOK, `{"data":[]}`)
			return
		}
		free := 100
		if graphCalls.Add(1) > 1 {
			free = 99
		}
		writeJSON(t, writer, http.StatusOK, fmt.Sprintf(
			`{"data":[{"id":9,"node":[],"store":[`+
				`{"id":21,"status":1,"free":%d},{"id":22,"status":1,"free":80}]}]}`,
			free,
		))
	}))
	first, err := fixture.provider.ListProviderCapacity(context.Background(), resourcecap.ProviderCapacityRequest{
		Kind: resourcecap.ProviderCapacityKindPlan, Limit: 1,
	})
	if err != nil || first.Complete {
		t.Fatalf("first plan page=%+v err=%v", first, err)
	}
	_, err = fixture.provider.ListProviderCapacity(context.Background(), resourcecap.ProviderCapacityRequest{
		Kind: resourcecap.ProviderCapacityKindPlan, Limit: 1, Cursor: first.NextCursor,
	})
	if !errors.Is(err, errInvalidResponse) {
		t.Fatalf("mutated creation graph error=%v, want invalid response", err)
	}
}

func TestPlanProviderCapacityRejectsMalformedTopLevelStorageSource(t *testing.T) {
	t.Parallel()
	var loginCount int
	var mu sync.Mutex
	login := loginHandler(t, &loginCount, &mu, "token-one")
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/administrator/v1/login":
			login(writer, request)
		case "/administrator/v1/clouds/create_page":
			writeJSON(t, writer, http.StatusOK, `{"data":[]}`)
		case "/administrator/v1/stores":
			writeJSON(t, writer, http.StatusOK, `{"data":{"id":31,"status":1}}`)
		default:
			writeJSON(t, writer, http.StatusNotFound, `{}`)
		}
	}))
	_, err := fixture.provider.ListProviderCapacity(context.Background(), resourcecap.ProviderCapacityRequest{
		Kind: resourcecap.ProviderCapacityKindPlan, Limit: 10,
	})
	if !errors.Is(err, errInvalidResponse) {
		t.Fatalf("malformed top-level storage error=%v, want invalid response", err)
	}
}

func TestPlanProviderCapacityRejectsTopLevelStorageMutationBetweenPages(t *testing.T) {
	t.Parallel()
	var loginCount int
	var mu sync.Mutex
	login := loginHandler(t, &loginCount, &mu, "token-one")
	var storeCalls atomic.Int64
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/administrator/v1/login":
			login(writer, request)
		case "/administrator/v1/clouds/create_page":
			writeJSON(t, writer, http.StatusOK, `{"data":[{"id":9,"node":[],"store":[
				{"id":21,"status":1,"free":100},{"id":22,"status":1,"free":80}
			]}]}`)
		case "/administrator/v1/stores":
			name := "Global Store"
			if storeCalls.Add(1) > 1 {
				name = "Changed Store"
			}
			writeJSON(t, writer, http.StatusOK, fmt.Sprintf(
				`{"data":[{"id":31,"name":%q,"status":1}]}`,
				name,
			))
		default:
			writeJSON(t, writer, http.StatusNotFound, `{}`)
		}
	}))
	first, err := fixture.provider.ListProviderCapacity(context.Background(), resourcecap.ProviderCapacityRequest{
		Kind: resourcecap.ProviderCapacityKindPlan, Limit: 2,
	})
	if err != nil || first.Complete {
		t.Fatalf("first merged plan page=%+v err=%v", first, err)
	}
	_, err = fixture.provider.ListProviderCapacity(context.Background(), resourcecap.ProviderCapacityRequest{
		Kind: resourcecap.ProviderCapacityKindPlan, Limit: 2, Cursor: first.NextCursor,
	})
	if !errors.Is(err, errInvalidResponse) {
		t.Fatalf("mutated top-level storage error=%v, want invalid response", err)
	}
}

func TestUnauthorizedResponseRefreshesTokenExactlyOnce(t *testing.T) {
	t.Parallel()
	var loginCount atomic.Int64
	var providerCapacityCount atomic.Int64
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/administrator/v1/login" {
			count := loginCount.Add(1)
			writeJSON(t, writer, http.StatusCreated, strconv.Quote("token-"+strconv.FormatInt(count, 10)))
			return
		}
		providerCapacityCount.Add(1)
		if request.Header.Get("access-token") == "token-1" {
			writeJSON(t, writer, http.StatusUnauthorized, `{"error":"expired"}`)
			return
		}
		if request.Header.Get("access-token") != "token-2" {
			t.Error("refreshed request used an unexpected token")
		}
		writeJSON(t, writer, http.StatusOK, `{"data":[],"meta":{"page":1,"total_page":1}}`)
	}))
	page, err := fixture.provider.ListProviderCapacity(context.Background(), resourcecap.ProviderCapacityRequest{
		Kind: resourcecap.ProviderCapacityKindRegion, Limit: 10,
	})
	if err != nil || !page.Complete {
		t.Fatalf("providerCapacity after refresh: page=%+v err=%v", page, err)
	}
	if loginCount.Load() != 2 || providerCapacityCount.Load() != 2 {
		t.Fatalf("login=%d providerCapacity=%d, want 2 and 2", loginCount.Load(), providerCapacityCount.Load())
	}
}

func TestExpiredLocalTokenTriggersNewLogin(t *testing.T) {
	t.Parallel()
	var loginCount atomic.Int64
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/administrator/v1/login" {
			count := loginCount.Add(1)
			writeJSON(t, writer, http.StatusCreated, strconv.Quote("token-"+strconv.FormatInt(count, 10)))
			return
		}
		writeJSON(t, writer, http.StatusOK, `{"data":[],"meta":{"page":1,"total_page":1}}`)
	}))
	request := resourcecap.ProviderCapacityRequest{Kind: resourcecap.ProviderCapacityKindRegion, Limit: 10}
	if _, err := fixture.provider.ListProviderCapacity(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	fixture.clock.Advance(defaultTokenCacheTTL + time.Second)
	if _, err := fixture.provider.ListProviderCapacity(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if loginCount.Load() != 2 {
		t.Fatalf("login count=%d, want 2", loginCount.Load())
	}
}

func TestTransientReadStatusesRemainRetryable(t *testing.T) {
	t.Parallel()
	for _, statusCode := range []int{
		http.StatusRequestTimeout,
		http.StatusTooEarly,
		http.StatusTooManyRequests,
		http.StatusServiceUnavailable,
	} {
		statusCode := statusCode
		t.Run(strconv.Itoa(statusCode), func(t *testing.T) {
			t.Parallel()
			var loginCount int
			var mu sync.Mutex
			login := loginHandler(t, &loginCount, &mu, "token-one")
			fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/administrator/v1/login" {
					login(writer, request)
					return
				}
				writeJSON(t, writer, statusCode, `{"error":"discard-me"}`)
			}))
			_, err := fixture.provider.ListProviderCapacity(context.Background(), resourcecap.ProviderCapacityRequest{
				Kind: resourcecap.ProviderCapacityKindRegion, Limit: 10,
			})
			var callError *resourcecap.ProviderCallError
			if !errors.As(err, &callError) || !callError.Retryable || callError.ResultUnknown {
				t.Fatalf("status %d error=%v, want retryable known read failure", statusCode, err)
			}
			if statusCode == http.StatusRequestTimeout && callError.Code != resourcecap.ProviderCallErrorCodeTimeout {
				t.Fatalf("status %d code=%s, want timeout", statusCode, callError.Code)
			}
			if statusCode != http.StatusRequestTimeout && callError.Code != resourcecap.ProviderCallErrorCodeTransport {
				t.Fatalf("status %d code=%s, want transport", statusCode, callError.Code)
			}
		})
	}
}

func TestAuthenticationTransportFailureIsNotMisclassifiedAsBadCredentials(t *testing.T) {
	t.Parallel()
	fixture := newProviderFixture(t, http.NotFoundHandler())
	fixture.provider.env.HTTPClient = &interceptClient{
		base: fixture.client,
		path: "/administrator/v1/login",
		err:  context.DeadlineExceeded,
	}
	_, err := fixture.provider.ListProviderCapacity(context.Background(), resourcecap.ProviderCapacityRequest{
		Kind: resourcecap.ProviderCapacityKindRegion, Limit: 10,
	})
	var callError *resourcecap.ProviderCallError
	if !errors.As(err, &callError) || callError.Code != resourcecap.ProviderCallErrorCodeTimeout ||
		!callError.Retryable || callError.ResultUnknown {
		t.Fatalf("authentication transport error=%v, want retryable timeout", err)
	}
}

func TestPartialProviderCapacityPaginationFailureDoesNotInventCompletion(t *testing.T) {
	t.Parallel()
	var loginCount int
	var mu sync.Mutex
	login := loginHandler(t, &loginCount, &mu, "token-one")
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/administrator/v1/login" {
			login(writer, request)
			return
		}
		if request.URL.Query().Get("page") == "1" {
			writeJSON(t, writer, http.StatusOK, `{"data":[{"id":1,"name":"Region"}],"meta":{"page":1,"total_page":2}}`)
			return
		}
		writeJSON(t, writer, http.StatusInternalServerError, `{"error":"temporary"}`)
	}))
	first, err := fixture.provider.ListProviderCapacity(context.Background(), resourcecap.ProviderCapacityRequest{
		Kind: resourcecap.ProviderCapacityKindRegion, Limit: 1,
	})
	if err != nil || first.Complete {
		t.Fatalf("first page=%+v err=%v", first, err)
	}
	_, err = fixture.provider.ListProviderCapacity(context.Background(), resourcecap.ProviderCapacityRequest{
		Kind: resourcecap.ProviderCapacityKindRegion, Limit: 1, Cursor: first.NextCursor,
	})
	if err == nil {
		t.Fatal("second page failure was accepted as a complete snapshot")
	}
}
