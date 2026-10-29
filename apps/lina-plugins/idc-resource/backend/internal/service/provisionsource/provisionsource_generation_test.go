// This file verifies frozen generation policies, retry-stable names and
// secret lifetime boundaries without supplier or database access.

package provisionsource

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
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
	service, err := New(database, secrets, principalReady, nil, nil, provisionManagedBuilder{})
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

type recoveredProvisionManager struct {
	secretcap.Manager
	stored  secretcap.CreateResult
	creates int
}

func (m *recoveredProvisionManager) RecoverCreate(context.Context, string) (secretcap.CreateResult, error) {
	return m.stored, nil
}
func (m *recoveredProvisionManager) Create(context.Context, string, secretcap.PlaintextProducer) (secretcap.CreateResult, error) {
	m.creates++
	return secretcap.CreateResult{}, errors.New("fresh random plaintext conflicts with committed credential")
}
func TestPreparationRetryRecoversCredentialBeforeGenerating(t *testing.T) {
	handle, err := secretcap.NewHandle("sec_preparation_retry")
	if err != nil {
		t.Fatal(err)
	}
	manager := &recoveredProvisionManager{stored: secretcap.CreateResult{Handle: handle, Metadata: secretcap.Metadata{Configured: true}, Reused: true}}
	service := &Service{random: bytes.NewReader(nil)}
	for range 2 {
		result, err := service.createProvisionCredential(t.Context(), manager, provisionresourcecap.TaskRequest{}, resourcecap.ServerGenerationPolicy{})
		if err != nil || result.Handle != handle || !result.Reused {
			t.Fatalf("preparation credential recovery err=%v", err)
		}
	}
	if manager.creates != 0 {
		t.Fatal("preparation retry generated another random credential")
	}
	manager.stored.Metadata.Configured = false
	if _, err = service.createProvisionCredential(t.Context(), manager, provisionresourcecap.TaskRequest{}, resourcecap.ServerGenerationPolicy{}); err == nil {
		t.Fatal("revoked credential resurrected")
	}
}

func TestDefaultProvisionPasswordMatchesDeclaredCloudLimits(t *testing.T) {
	service := &Service{random: rand.Reader}
	err := service.passwordProducer()(func(value []byte) error {
		if len(value) != 20 {
			t.Fatal("default cloud password exceeds declared maximum")
		}
		for _, class := range []string{"abcdefghijklmnopqrstuvwxyz", "ABCDEFGHIJKLMNOPQRSTUVWXYZ", "0123456789", "!@#$%*-_=+?"} {
			if !strings.ContainsAny(string(value), class) {
				t.Fatal("default cloud password misses required class")
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCustomerHostnameKeepsExactValueWithoutGeneratedPrefix(t *testing.T) {
	request := provisionresourcecap.TaskRequest{Fulfillment: provisionresourcecap.FrozenFulfillmentIdentity{Quantity: 1}, ServerLogin: &provisionresourcecap.ServerLoginChoice{Hostname: "customer.example"}}
	policy := resourcecap.ServerGenerationPolicy{HostnameCustomerInput: true, HostnamePrefix: "other"}
	actual, err := generatedServerHostname(request, policy)
	if err != nil || actual != "customer.example" {
		t.Fatal("manual hostname changed")
	}
	policy.HostnameCustomerInput = false
	if _, err = generatedServerHostname(request, policy); err == nil {
		t.Fatal("disabled manual hostname accepted")
	}
	policy.HostnameCustomerInput = true
	request.Fulfillment.Quantity = 2
	if _, err = generatedServerHostname(request, policy); err == nil {
		t.Fatal("manual hostname copied across servers")
	}
	request.ServerLogin = nil
	if _, err = generatedServerHostname(request, policy); err != nil {
		t.Fatal("historical automatic generation failed")
	}
}

func TestCustomerPasswordUsesOwnedSecretWithoutGeneratingAnother(t *testing.T) {
	secrets := newProvisionSecretSource(t)
	original, err := secrets.Create(t.Context(), "customer-input", func(consume secretcap.Consumer) error { return consume([]byte("TypedPass123")) })
	if err != nil {
		t.Fatal(err)
	}
	request := provisionresourcecap.TaskRequest{Fulfillment: provisionresourcecap.FrozenFulfillmentIdentity{Quantity: 1}, ServerLogin: &provisionresourcecap.ServerLoginChoice{PasswordSecretRef: original.Handle.OpaqueRef()}}
	service := &Service{}
	result, err := service.createProvisionCredential(t.Context(), secrets, request, resourcecap.ServerGenerationPolicy{PasswordCustomerInput: true})
	if err != nil || !result.Handle.Equal(original.Handle) || secrets.producerCount != 1 {
		t.Fatal("customer password was not reused privately")
	}
	if _, err = service.createProvisionCredential(t.Context(), secrets, request, resourcecap.ServerGenerationPolicy{}); err == nil {
		t.Fatal("disabled customer password accepted")
	}
	request.Fulfillment.Quantity = 2
	if _, err = service.createProvisionCredential(t.Context(), secrets, request, resourcecap.ServerGenerationPolicy{PasswordCustomerInput: true}); err == nil {
		t.Fatal("customer password copied to multiple servers")
	}
}

func TestCustomerServerLoginReachesDurableProvisionTaskPostgreSQL(t *testing.T) {
	ctx, database := newProvisionSourceDatabase(t)
	seedProvisionRoute(t, ctx, database)
	secrets := newProvisionSecretSource(t)
	stored, err := secrets.Create(ctx, "customer-purchase", func(consume secretcap.Consumer) error { return consume([]byte("ExactTypedPass7")) })
	if err != nil {
		t.Fatal(err)
	}
	owner, err := New(database, secrets, principalReady, nil, nil, provisionManagedBuilder{})
	if err != nil {
		t.Fatal(err)
	}
	request := frozenProvisionSourceRequest(t)
	var product map[string]json.RawMessage
	if err = json.Unmarshal(request.ProductConfiguration, &product); err != nil {
		t.Fatal(err)
	}
	product["hostname_customer_input"] = json.RawMessage(`true`)
	product["password_customer_input"] = json.RawMessage(`true`)
	request.ProductConfiguration, err = json.Marshal(product)
	if err != nil {
		t.Fatal(err)
	}
	request.ServerLogin = &provisionresourcecap.ServerLoginChoice{Hostname: "customer.example", PasswordSecretRef: stored.Handle.OpaqueRef(), ChannelNo: "channel-provision-1"}
	request.Fulfillment.Quantity = 1
	request.RequestDigest, err = provisionresourcecap.TaskRequestDigest(request)
	if err != nil {
		t.Fatal(err)
	}
	task, err := owner.CreateTask(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	var persisted struct{ Parameters, OwnerMetadata, PasswordSecretRef string }
	err = database.GetScan(ctx, &persisted, `SELECT task.sanitized_parameters::text AS parameters, task.owner_metadata::text AS owner_metadata,binding.secret_ref AS password_secret_ref FROM plugin_idc_resource_task task JOIN plugin_idc_resource_task_secret_binding binding ON binding.task_no=task.task_no AND binding.binding_name='password' WHERE task.task_no=?`, task.ProviderTaskNo)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(persisted.Parameters, `"hostname": "customer.example"`) || persisted.PasswordSecretRef != stored.Handle.OpaqueRef() || secrets.producerCount != 1 {
		t.Fatal("customer choices did not reach durable provision task")
	}
	if strings.Contains(persisted.Parameters, "ExactTypedPass7") || strings.Contains(persisted.OwnerMetadata, "ExactTypedPass7") {
		t.Fatal("plaintext password persisted in task")
	}
	replayed, err := owner.CreateTask(ctx, request)
	if err != nil || replayed.ProviderTaskNo != task.ProviderTaskNo || secrets.producerCount != 1 {
		t.Fatal("retry changed customer server credentials")
	}
}
