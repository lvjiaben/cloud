// This file tests the private Tencent configuration parser and its closed
// capability vocabulary.

package tencentess

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadConfigFileAcceptsBuyerOwnedClosedConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tencent-essbasic.json")
	payload := []byte(`{
  "apiVersion":"2021-05-26",
  "secretId":"secret-id",
  "secretKey":"secret-key",
  "appId":"app-id",
  "capabilities":["flow.query","template.read"]
}`)
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatalf("write configuration: %v", err)
	}
	config, err := LoadConfigFile(path)
	if err != nil {
		t.Fatalf("LoadConfigFile returned error: %v", err)
	}
	defer config.Clear()
	if config.APIVersion != APIVersion || !config.HasCapability(CapabilityFlowQuery) ||
		config.HasCapability(CapabilityFlowCreate) {
		t.Fatalf("unexpected configuration: %+v", config)
	}
	sorted := config.SortedCapabilities()
	if len(sorted) != 2 || sorted[0] != CapabilityFlowQuery || sorted[1] != CapabilityTemplateRead {
		t.Fatalf("unexpected sorted capabilities: %v", sorted)
	}
}

func TestLoadConfigFileAcceptsCompleteCallbackConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tencent-essbasic.json")
	payload := []byte(`{
  "apiVersion":"2021-05-26",
  "secretId":"secret-id",
  "secretKey":"secret-key",
  "appId":"app-id",
  "callbackVerifyToken":"verify-token",
  "callbackUrlKey":"0123456789abcdef0123456789abcdef",
  "capabilities":["callback.receive"]
}`)
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatalf("write configuration: %v", err)
	}
	config, err := LoadConfigFile(path)
	if err != nil {
		t.Fatalf("LoadConfigFile returned error: %v", err)
	}
	defer config.Clear()
	if !config.HasCapability(CapabilityCallback) || config.CallbackVerifyToken != "verify-token" ||
		config.CallbackURLKey != "0123456789abcdef0123456789abcdef" {
		t.Fatal("complete callback configuration was not retained")
	}
}

func TestLoadConfigFileRejectsEndpointAndCapabilityOverrides(t *testing.T) {
	root := t.TempDir()
	tests := []string{
		`{"apiVersion":"2021-05-26","secretId":"id","secretKey":"key","appId":"app","capabilities":["template.read"],"endpoint":"https://example.com"}`,
		`{"apiVersion":"2021-05-26","secretId":"id","secretKey":"key","appId":"app","capabilities":["template.read","template.read"]}`,
		`{"apiVersion":"2021-05-26","secretId":"id","secretKey":"key","appId":"app","capabilities":["unknown"]}`,
		`{"apiVersion":"2020-01-01","secretId":"id","secretKey":"key","appId":"app","capabilities":["template.read"]}`,
		`{"apiVersion":"2021-05-26","secretId":"id","secretKey":"key","appId":"app","capabilities":["callback.receive"]}`,
		`{"apiVersion":"2021-05-26","secretId":"id","secretKey":"key","appId":"app","callbackVerifyToken":"token","callbackUrlKey":"12345678901234567890123456789012","capabilities":["template.read"]}`,
		`{"apiVersion":"2021-05-26","secretId":"id","secretKey":"key","appId":"app","callbackVerifyToken":"token","capabilities":["template.read"]}`,
	}
	for index, payload := range tests {
		path := filepath.Join(root, "invalid-"+string(rune('a'+index))+".json")
		if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
			t.Fatalf("write invalid configuration: %v", err)
		}
		if _, err := LoadConfigFile(path); err == nil {
			t.Fatalf("case %d was accepted", index)
		}
	}
}

func TestLoadConfigFileRejectsOversizedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oversized.json")
	payload := make([]byte, MaximumConfigFileBytes+1)
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatalf("write oversized configuration: %v", err)
	}
	if _, err := LoadConfigFile(path); err == nil {
		t.Fatal("oversized configuration was accepted")
	}
}

func TestLoadConfigFileRejectsBroadUnixPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose Unix permission bits")
	}
	path := filepath.Join(t.TempDir(), "broad-permissions.json")
	payload := []byte(`{"apiVersion":"2021-05-26","secretId":"id","secretKey":"key","appId":"app","capabilities":["template.read"]}`)
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("write broad-permission configuration: %v", err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatalf("set broad permissions: %v", err)
	}
	if _, err := LoadConfigFile(path); err == nil {
		t.Fatal("broad Unix permissions were accepted")
	}
}

func TestOfficialClientProfileCannotRedirectOrRetry(t *testing.T) {
	clientProfile := officialClientProfile()
	if clientProfile.SignMethod != "TC3-HMAC-SHA256" || !clientProfile.DisableRegionBreaker ||
		clientProfile.NetworkFailureMaxRetries != 0 ||
		clientProfile.RateLimitExceededMaxRetries != 0 ||
		clientProfile.UnsafeRetryOnConnectionFailure || clientProfile.Debug ||
		clientProfile.HttpProfile.Endpoint != OfficialEndpoint ||
		clientProfile.HttpProfile.Scheme != "HTTPS" || clientProfile.HttpProfile.Proxy != "" ||
		clientProfile.HttpProfile.ReqTimeout != requestTimeoutSeconds {
		t.Fatalf("unexpected official profile: %+v", clientProfile)
	}
}
