// This file exercises real HTTP streaming without exposing Basic credentials
// or accepting corrupted immutable source evidence as an upstream file token.
package composition

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/channelconfig"
)

func TestPurchaseFileTransferStreamsWithBoundAuthenticationAndChecksDigest(t *testing.T) {
	content := strings.Repeat("purchase-file\n", 100000)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok || user != "app_test" || password != "test-secret" {
			t.Error("missing resource-owned authentication")
		}
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			_, _ = io.WriteString(w, `{"code":0,"data":{"uploadToken":"CFU_0123456789ABCDEF0123456789ABCDEF","expiresAt":1}}`)
			return
		}
		if r.Method != http.MethodPut || r.URL.Path != peerPurchaseFilePath+"/file-uploads/CFU_0123456789ABCDEF0123456789ABCDEF/content" {
			t.Error("unexpected stream destination")
		}
		body, err := io.ReadAll(r.Body)
		if err != nil || string(body) != content {
			t.Error("file stream corrupted")
		}
		response := map[string]any{"code": 0, "data": map[string]any{"fileToken": "CFU_0123456789ABCDEF0123456789ABCDEF", "status": "ready", "sizeBytes": len(content)}}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()
	user, _ := secretcap.NewHandle("secret_app_identity")
	secret, _ := secretcap.NewHandle("secret_app_secret")
	handler := &confidentialHTTPHandler{configuration: channelconfig.Configuration{Config: resourcecap.ProviderConfig{Endpoint: server.URL, Settings: json.RawMessage(`{"__host":{"policy":{"mode":"per_request_basic","mappings":[{"role":"username","slot":"app_id"},{"role":"password","slot":"app_secret"}]}}}`), SecretBindings: []resourcecap.SecretBinding{{Name: "app_id", Handle: user}, {Name: "app_secret", Handle: secret}}}}, environment: resourcecap.ProviderEnv{HTTPClient: confidentialHTTPClient{client: server.Client()}, Secrets: confidentialHTTPAccessor{"secret_app_identity": []byte("app_test"), "secret_app_secret": []byte("test-secret")}}}
	sum := sha256.Sum256([]byte(content))
	input := resourceinterfaceprojectioncap.PurchaseFileTransfer{PublicSlug: "product", ConfigurationRevision: "revision", FieldToken: "field", FileName: "upload.txt", SizeBytes: int64(len(content)), SHA256Hex: hex.EncodeToString(sum[:]), Body: strings.NewReader(content)}
	token, err := handler.transferPurchaseFile(t.Context(), input)
	if err != nil || token != "CFU_0123456789ABCDEF0123456789ABCDEF" {
		t.Fatalf("stream transfer failed token=%q err=%v", token, err)
	}
	input.Body = strings.NewReader(content)
	input.SHA256Hex = strings.Repeat("0", 64)
	if token, err = handler.transferPurchaseFile(t.Context(), input); err == nil || token != "" {
		t.Fatal("corrupted source evidence accepted")
	}
}
