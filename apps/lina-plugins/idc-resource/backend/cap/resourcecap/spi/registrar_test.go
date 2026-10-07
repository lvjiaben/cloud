// This file verifies Provider declaration validation, freezing, and concurrency safety.

package spi

import (
	"context"
	"strings"
	"sync"
	"testing"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

// TestNewRegistrarRejectsInvalidSupportedVersion verifies the composition root
// cannot start with an ambiguous contract version.
func TestNewRegistrarRejectsInvalidSupportedVersion(t *testing.T) {
	for _, version := range []string{"", "1", "v0", "v01", "v1-beta", "v1.1", "v2"} {
		t.Run(version, func(t *testing.T) {
			if _, err := newRegistrar(version); err == nil {
				t.Fatalf("expected supported version %q to be rejected", version)
			}
		})
	}
}

// TestDeclareRejectsInvalidDeclarations verifies every startup-fatal declaration path.
func TestDeclareRejectsInvalidDeclarations(t *testing.T) {
	tests := []struct {
		name        string
		declaration Declaration
		wantError   string
	}{
		{
			name: "empty plugin ID",
			declaration: testDeclaration(resourcecap.ProviderKey{
				ProviderID: "provider", ContractVersion: "v1",
			}),
			wantError: "plugin ID is required",
		},
		{
			name: "invalid plugin ID",
			declaration: testDeclaration(resourcecap.ProviderKey{
				PluginID: "Invalid", ProviderID: "provider", ContractVersion: "v1",
			}),
			wantError: "plugin ID",
		},
		{
			name: "empty Provider ID",
			declaration: testDeclaration(resourcecap.ProviderKey{
				PluginID: "source-plugin", ContractVersion: "v1",
			}),
			wantError: "provider ID is required",
		},
		{
			name: "invalid Provider ID",
			declaration: testDeclaration(resourcecap.ProviderKey{
				PluginID: "source-plugin", ProviderID: "invalid_provider", ContractVersion: "v1",
			}),
			wantError: "provider ID",
		},
		{
			name: "empty contract version",
			declaration: testDeclaration(resourcecap.ProviderKey{
				PluginID: "source-plugin", ProviderID: "provider",
			}),
			wantError: "contract version",
		},
		{
			name: "invalid contract version",
			declaration: testDeclaration(resourcecap.ProviderKey{
				PluginID: "source-plugin", ProviderID: "provider", ContractVersion: "v1-beta",
			}),
			wantError: "contract version",
		},
		{
			name: "unsupported contract version",
			declaration: testDeclaration(resourcecap.ProviderKey{
				PluginID: "source-plugin", ProviderID: "provider", ContractVersion: "v2",
			}),
			wantError: "outside the owner-supported range",
		},
		{
			name: "nil factory",
			declaration: Declaration{Key: resourcecap.ProviderKey{
				PluginID: "source-plugin", ProviderID: "provider", ContractVersion: "v1",
			}},
			wantError: "factory is required",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registrar := newTestRegistrar(t)
			err := registrar.declare(test.declaration)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("Declare() error=%v, want substring %q", err, test.wantError)
			}
		})
	}

	var nilRegistrar *declarationRegistrar
	if err := nilRegistrar.declare(testDeclaration(testKey("nil"))); err == nil {
		t.Fatal("expected nil registrar declaration to fail")
	}
}

// TestDeclareUsesTheCompleteExactKey verifies duplicate detection and the
// owner's explicit exact-version support range.
func TestDeclareUsesTheCompleteExactKey(t *testing.T) {
	registrar := newTestRegistrar(t)
	base := testDeclaration(testKey("provider"))
	if err := registrar.declare(base); err != nil {
		t.Fatalf("declare base: %v", err)
	}
	if err := registrar.declare(base); err == nil || !strings.Contains(err.Error(), "already declared") {
		t.Fatalf("expected exact duplicate to fail, got %v", err)
	}
	aliasVersion := base
	aliasVersion.Key.ContractVersion = "v1.0"
	if err := registrar.declare(aliasVersion); err == nil || !strings.Contains(err.Error(), "not canonical") {
		t.Fatalf("expected semantic version alias to fail, got %v", err)
	}

	compatibleVersion := base
	compatibleVersion.Key.ContractVersion = "v1.1"
	if err := registrar.declare(compatibleVersion); err == nil || !strings.Contains(err.Error(), "owner-supported range") {
		t.Fatalf("expected undeclared same-major compatibility to fail, got %v", err)
	}
	differentPlugin := base
	differentPlugin.Key.PluginID = "alternate-plugin"
	if err := registrar.declare(differentPlugin); err != nil {
		t.Fatalf("declare different plugin: %v", err)
	}

	snapshot, err := registrar.freeze()
	if err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if snapshot.Len() != 2 {
		t.Fatalf("snapshot length=%d, want 2", snapshot.Len())
	}
}

// TestFreezeIsDeterministicImmutableAndFinal verifies declaration order cannot
// affect the frozen composition and callers cannot mutate snapshot contents.
func TestFreezeIsDeterministicImmutableAndFinal(t *testing.T) {
	keys := []resourcecap.ProviderKey{
		{PluginID: "z-plugin", ProviderID: "provider-b", ContractVersion: "v1"},
		{PluginID: "a-plugin", ProviderID: "provider-z", ContractVersion: "v1"},
		{PluginID: "a-plugin", ProviderID: "provider-a", ContractVersion: "v1"},
	}
	registrar := newTestRegistrar(t)
	for _, key := range keys {
		if err := registrar.declare(testDeclaration(key)); err != nil {
			t.Fatalf("declare %+v: %v", key, err)
		}
	}
	snapshot, err := registrar.freeze()
	if err != nil {
		t.Fatalf("freeze: %v", err)
	}
	wantOrder := []resourcecap.ProviderKey{keys[2], keys[1], keys[0]}
	assertSnapshotKeys(t, snapshot, wantOrder)

	returned := snapshot.Declarations()
	returned[0].Key.PluginID = "mutated"
	assertSnapshotKeys(t, snapshot, wantOrder)

	repeated, err := registrar.freeze()
	if err != nil {
		t.Fatalf("repeat freeze: %v", err)
	}
	assertSnapshotKeys(t, repeated, wantOrder)
	if err := registrar.declare(testDeclaration(testKey("late"))); err == nil || !strings.Contains(err.Error(), "frozen") {
		t.Fatalf("expected post-freeze declaration to fail, got %v", err)
	}

	var nilRegistrar *declarationRegistrar
	if _, err := nilRegistrar.freeze(); err == nil {
		t.Fatal("expected nil registrar freeze to fail")
	}
}

// TestFreezeIsIndependentOfDeclarationOrder verifies two valid startup orders
// produce the same immutable declaration snapshot.
func TestFreezeIsIndependentOfDeclarationOrder(t *testing.T) {
	keys := []resourcecap.ProviderKey{
		{PluginID: "third-plugin", ProviderID: "gamma", ContractVersion: "v1"},
		{PluginID: "first-plugin", ProviderID: "alpha", ContractVersion: "v1"},
		{PluginID: "second-plugin", ProviderID: "beta", ContractVersion: "v1"},
	}
	left := freezeKeys(t, keys)
	reversed := []resourcecap.ProviderKey{keys[2], keys[1], keys[0]}
	right := freezeKeys(t, reversed)

	leftItems := left.Declarations()
	rightItems := right.Declarations()
	if len(leftItems) != len(rightItems) {
		t.Fatalf("snapshot lengths differ: %d != %d", len(leftItems), len(rightItems))
	}
	for index := range leftItems {
		if leftItems[index].Key != rightItems[index].Key {
			t.Fatalf("snapshot key %d differs: %+v != %+v", index, leftItems[index].Key, rightItems[index].Key)
		}
	}
}

// TestRegistrarConcurrentDeclarations verifies unique and duplicate declarations
// are race-safe and exactly one duplicate contender succeeds.
func TestRegistrarConcurrentDeclarations(t *testing.T) {
	const declarationCount = 64
	registrar := newTestRegistrar(t)
	start := make(chan struct{})
	results := make(chan error, declarationCount)
	var wait sync.WaitGroup
	for index := 0; index < declarationCount; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			key := resourcecap.ProviderKey{
				PluginID:        "source-plugin",
				ProviderID:      providerID(index),
				ContractVersion: "v1",
			}
			results <- registrar.declare(testDeclaration(key))
		}(index)
	}
	close(start)
	wait.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatalf("concurrent unique declaration failed: %v", err)
		}
	}
	snapshot, err := registrar.freeze()
	if err != nil {
		t.Fatalf("freeze unique declarations: %v", err)
	}
	if snapshot.Len() != declarationCount {
		t.Fatalf("unique snapshot length=%d, want %d", snapshot.Len(), declarationCount)
	}

	duplicateRegistrar := newTestRegistrar(t)
	duplicateStart := make(chan struct{})
	duplicateResults := make(chan error, declarationCount)
	for index := 0; index < declarationCount; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-duplicateStart
			duplicateResults <- duplicateRegistrar.declare(testDeclaration(testKey("duplicate")))
		}()
	}
	close(duplicateStart)
	wait.Wait()
	close(duplicateResults)
	successes := 0
	for err := range duplicateResults {
		if err == nil {
			successes++
			continue
		}
		if !strings.Contains(err.Error(), "already declared") {
			t.Fatalf("unexpected duplicate error: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("duplicate declaration successes=%d, want 1", successes)
	}
}

// TestDeclareAndFreezeAreLinearizable verifies concurrent finalization includes
// exactly the declarations accepted before the freeze boundary.
func TestDeclareAndFreezeAreLinearizable(t *testing.T) {
	const declarationCount = 128
	registrar := newTestRegistrar(t)
	start := make(chan struct{})
	results := make(chan error, declarationCount)
	freezeResult := make(chan error, 1)
	var wait sync.WaitGroup
	for index := 0; index < declarationCount; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			results <- registrar.declare(testDeclaration(resourcecap.ProviderKey{
				PluginID:        "source-plugin",
				ProviderID:      providerID(index),
				ContractVersion: "v1",
			}))
		}(index)
	}
	wait.Add(1)
	go func() {
		defer wait.Done()
		<-start
		_, err := registrar.freeze()
		freezeResult <- err
	}()
	close(start)
	wait.Wait()
	close(results)
	if err := <-freezeResult; err != nil {
		t.Fatalf("concurrent freeze: %v", err)
	}

	accepted := 0
	for err := range results {
		if err == nil {
			accepted++
			continue
		}
		if !strings.Contains(err.Error(), "frozen") {
			t.Fatalf("unexpected concurrent declaration error: %v", err)
		}
	}
	snapshot, err := registrar.freeze()
	if err != nil {
		t.Fatalf("read final snapshot: %v", err)
	}
	if snapshot.Len() != accepted {
		t.Fatalf("snapshot length=%d, accepted declarations=%d", snapshot.Len(), accepted)
	}
}

// newTestRegistrar constructs an isolated exact-version registrar.
func newTestRegistrar(t *testing.T) *declarationRegistrar {
	t.Helper()
	registrar, err := newRegistrar(resourcecap.ContractVersion)
	if err != nil {
		t.Fatalf("newRegistrar: %v", err)
	}
	return registrar
}

// testKey constructs one valid exact-version declaration identity.
func testKey(providerID string) resourcecap.ProviderKey {
	return resourcecap.ProviderKey{
		PluginID:        "source-plugin",
		ProviderID:      providerID,
		ContractVersion: resourcecap.ContractVersion,
	}
}

// testDeclaration wraps one key with the inert test factory.
func testDeclaration(key resourcecap.ProviderKey) Declaration {
	return Declaration{Key: key, Factory: testProviderFactory}
}

// testProviderFactory is an inert factory used only for declaration tests.
func testProviderFactory(context.Context, resourcecap.ProviderEnv, resourcecap.ProviderConfig) (resourcecap.Provider, error) {
	return nil, nil
}

// providerID builds a deterministic valid ID without formatting dependencies.
func providerID(index int) string {
	const digits = "0123456789"
	if index == 0 {
		return "provider-0"
	}
	buffer := make([]byte, 0, 16)
	for index > 0 {
		buffer = append(buffer, digits[index%10])
		index /= 10
	}
	for left, right := 0, len(buffer)-1; left < right; left, right = left+1, right-1 {
		buffer[left], buffer[right] = buffer[right], buffer[left]
	}
	return "provider-" + string(buffer)
}

// freezeKeys declares and freezes one isolated ordered test set.
func freezeKeys(t *testing.T, keys []resourcecap.ProviderKey) Snapshot {
	t.Helper()
	registrar := newTestRegistrar(t)
	for _, key := range keys {
		if err := registrar.declare(testDeclaration(key)); err != nil {
			t.Fatalf("declare %+v: %v", key, err)
		}
	}
	snapshot, err := registrar.freeze()
	if err != nil {
		t.Fatalf("freeze: %v", err)
	}
	return snapshot
}

// assertSnapshotKeys compares a detached snapshot's exact deterministic order.
func assertSnapshotKeys(t *testing.T, snapshot Snapshot, want []resourcecap.ProviderKey) {
	t.Helper()
	items := snapshot.Declarations()
	if len(items) != len(want) {
		t.Fatalf("snapshot length=%d, want %d", len(items), len(want))
	}
	for index := range want {
		if items[index].Key != want[index] {
			t.Fatalf("snapshot key %d=%+v, want %+v", index, items[index].Key, want[index])
		}
	}
}
