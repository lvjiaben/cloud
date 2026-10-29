// These tests cover actual bracket-form HTTP transmission and reject malformed
// or credential-bearing public arrays before authentication or supplier access.
package composition

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/channelconfig"
)

func TestConfidentialFormArrayPreservesDataDiskInActualHTTP(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		if r.PostForm.Get("other_data_disk[0][size]") != "22" || r.PostForm.Get("cpu") != "2" || r.PostForm.Get("data_disk_size") != "" {
			t.Error("frozen data disk or CPU configuration lost in supplier form")
		}
		if _, err := io.WriteString(w, `{"data":{"id":41,"taskid":71}}`); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	h := &confidentialHTTPHandler{
		login:         &resourcecap.ConfidentialHTTPRequest{Kind: resourcecap.ConfidentialHTTPKindAuthenticate, Method: http.MethodPost, Path: "/login"},
		configuration: channelconfig.Configuration{PluginID: "idc-zjmfcloud", Config: resourcecap.ProviderConfig{Endpoint: server.URL, Settings: json.RawMessage(`{"__host":{"policy":{"mode":"login_session","mappings":[{"role":"password","slot":"password"},{"role":"username","slot":"username"}],"path":"/login","readPath":["$"],"header":"Access-Token","ttlSeconds":3600}}}`)}},
		environment:   resourcecap.ProviderEnv{HTTPClient: confidentialHTTPClient{client: server.Client()}, AuthenticationSessions: &confidentialHTTPSessions{token: []byte("test-session")}},
	}
	result := executeConfidentialHTTPStep(t, h, 2, resourcecap.ConfidentialHTTPRequest{Kind: resourcecap.ConfidentialHTTPKindRequest, Method: http.MethodPost, Path: "/clouds", UseAuthentication: true, ResponseFields: []string{"data", "id", "taskid"}, FormFields: resourcecap.NewConfidentialFormFields(map[string]string{"other_data_disk[0][size]": "22", "cpu": "2"})})
	if calls != 1 || len(result) == 0 {
		t.Fatal("expected exactly one supplier submission")
	}
}

func TestConfidentialFormArrayRejectsUnsafeInputBeforeSupplierAccess(t *testing.T) {
	for _, fields := range [][]resourcecap.ConfidentialFormField{
		{{Field: "cpu", Value: "1"}, {Field: "cpu", Value: "2"}},
		{{Field: "password", Value: "private-value"}},
		{{Field: "rootpass", Value: "private-value"}},
		{{Field: "user[password]", Value: "private-value"}},
		{{Field: "access_token", Value: "private-value"}},
		{{Field: "username", Value: "private-value"}},
		{{Field: "bad&field", Value: "1"}},
		{{Field: "other_data_disk[0][size", Value: "22"}},
		make([]resourcecap.ConfidentialFormField, 129),
	} {
		request := resourcecap.ConfidentialHTTPRequest{Kind: resourcecap.ConfidentialHTTPKindRequest, Method: http.MethodPost, Path: "/clouds", UseAuthentication: true, ResponseFields: []string{"data"}, FormFields: fields}
		payload, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		// No environment exists: reaching authentication/HTTP would panic.
		h := &confidentialHTTPHandler{}
		if result, callErr := h.Execute(context.Background(), &pluginhost.ConfidentialOutboundRequest{Sequence: 2, Payload: payload}); callErr == nil || result != nil {
			t.Fatal("unsafe public form reached supplier execution")
		}
		if len(request.Form) != 0 {
			t.Fatal("invalid form partially mutated")
		}
	}
	request := resourcecap.ConfidentialHTTPRequest{Kind: resourcecap.ConfidentialHTTPKindRequest, Form: map[string]string{"cpu": "1"}, FormFields: []resourcecap.ConfidentialFormField{{Field: "cpu", Value: "2"}}}
	if normalizeConfidentialPublicFormFields(&request) == nil || request.Form["cpu"] != "1" {
		t.Fatal("mixed public projections were accepted or mutated")
	}
}
