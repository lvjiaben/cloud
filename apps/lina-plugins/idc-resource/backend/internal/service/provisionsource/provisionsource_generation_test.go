// This file verifies frozen generation policies, retry-stable names and
// secret lifetime boundaries without supplier or database access.

package provisionsource

import (
	"crypto/rand"
	"encoding/json"
	"strings"
	"testing"

	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
)

func TestFrozenGenerationPolicyRejectsInvalidRules(t *testing.T) {
	t.Parallel()
	for _, payload := range []string{
		`{"hostname_prefix":"not-valid"}`, `{"hostname_prefix":"abcdefghijk"}`,
		`{"hostname_length":3}`, `{"hostname_length":64}`,
		`{"hostname_prefix":"abcd","hostname_length":4}`,
		`{"hostname_characters":["symbols"]}`, `{"hostname_characters":["digits","digits"]}`,
		`{"password_length":5}`, `{"password_length":21}`, `{"password_length":12.5}`,
		`{"password_characters":["unknown"]}`,
	} {
		if _, err := parseServerGenerationPolicy(json.RawMessage(payload)); err == nil {
			t.Fatalf("invalid generation rules accepted: %s", payload)
		}
	}
}

func TestFrozenHostnamePolicyPreservesLegacyAndRetryIdentity(t *testing.T) {
	t.Parallel()
	request := provisionresourcecap.TaskRequest{}
	_, legacy := frozenServerIdentity(request)
	policy, err := parseServerGenerationPolicy(json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	name, err := generatedServerHostname(request, policy)
	if err != nil || name != legacy {
		t.Fatal("legacy hostname changed")
	}
	policy, err = parseServerGenerationPolicy(json.RawMessage(`{"hostname_prefix":"Xxun","hostname_length":12,"hostname_characters":["digits"]}`))
	if err != nil {
		t.Fatal(err)
	}
	name, err = generatedServerHostname(request, policy)
	if err != nil || len(name) != 12 || !strings.HasPrefix(name, "xxun") || strings.Trim(name[4:], "0123456789") != "" {
		t.Fatalf("hostname did not follow frozen rules: %q %v", name, err)
	}
	retry, err := generatedServerHostname(request, policy)
	if err != nil || retry != name {
		t.Fatal("retry generated a different hostname")
	}
}

func TestGeneratedPasswordIncludesClassesAndClearsConsumedView(t *testing.T) {
	t.Parallel()
	policy, err := parseServerGenerationPolicy(json.RawMessage(`{"password_length":12,"password_characters":["lowercase","uppercase","digits","symbols"]}`))
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{random: rand.Reader}
	var consumed []byte
	calls := 0
	err = service.generatedPasswordProducer(policy)(func(value []byte) error {
		calls++
		if len(value) != 12 {
			t.Fatal("password length mismatch")
		}
		for _, alphabet := range []string{"abcdefghijklmnopqrstuvwxyz", "ABCDEFGHIJKLMNOPQRSTUVWXYZ", "0123456789", "!@#$%*-_=+?"} {
			if !strings.ContainsAny(string(value), alphabet) {
				t.Fatal("required character class missing")
			}
		}
		// Retain only for this negative security test: the producer must erase
		// the view immediately when its governed consumer returns.
		consumed = value
		return nil
	})
	if err != nil || calls != 1 {
		t.Fatalf("consumer calls=%d err=%v", calls, err)
	}
	for _, value := range consumed {
		if value != 0 {
			t.Fatal("password view survived consumer")
		}
	}
}

// TestGenerationRulesReachPersistedProvisionTask verifies the real task and
// secret producer consume the frozen rules, including idempotent replays.
func TestGenerationRulesReachPersistedProvisionTask(t *testing.T) {
	ctx, database := newProvisionSourceDatabase(t)
	seedProvisionRoute(t, ctx, database)
	secrets := newProvisionSecretSource(t)
	service, err := New(database, secrets, principalReady, nil, provisionManagedBuilder{})
	if err != nil {
		t.Fatal(err)
	}
	request := frozenProvisionSourceRequest(t)
	var values map[string]any
	if err = json.Unmarshal(request.ProductConfiguration, &values); err != nil {
		t.Fatal(err)
	}
	values["hostname_prefix"] = "Xxun"
	values["hostname_length"] = 12
	values["hostname_characters"] = []string{"digits"}
	values["password_length"] = 16
	values["password_characters"] = []string{"uppercase", "digits"}
	request.ProductConfiguration, err = json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	request.RequestDigest, err = provisionresourcecap.TaskRequestDigest(request)
	if err != nil {
		t.Fatal(err)
	}
	created, err := service.CreateTask(ctx, request)
	if err != nil || created == nil {
		t.Fatalf("create with frozen generation rules: %v", err)
	}
	row, err := database.GetValue(ctx, `SELECT sanitized_parameters FROM plugin_idc_resource_task WHERE task_no=?`, created.ProviderTaskNo)
	if err != nil {
		t.Fatal(err)
	}
	var parameters struct {
		Hostname string `json:"hostname"`
	}
	if err = json.Unmarshal([]byte(row.String()), &parameters); err != nil {
		t.Fatal(err)
	}
	if len(parameters.Hostname) != 12 || !strings.HasPrefix(parameters.Hostname, "xxun") || strings.Trim(parameters.Hostname[4:], "0123456789") != "" {
		t.Fatal("supplier task did not receive the generated hostname")
	}
	password := secrets.plaintext()
	if len(password) != 16 || strings.Trim(string(password), "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789") != "" || !strings.ContainsAny(string(password), "0123456789") || !strings.ContainsAny(string(password), "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		t.Fatal("secret producer did not follow frozen password rules")
	}
	if strings.Contains(row.String(), string(password)) {
		t.Fatal("supplier task stored a plaintext password")
	}
	replay, err := service.CreateTask(ctx, request)
	if err != nil || replay == nil || !replay.Reused || replay.ProviderTaskNo != created.ProviderTaskNo || secrets.createCalls() != 1 {
		t.Fatalf("replay changed generated credentials: %v", err)
	}
}
