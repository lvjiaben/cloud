// This file verifies secret lifecycle, isolation, rollback, and rotation against random PostgreSQL schemas.

package secret

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

const secretIntegrationDSNEnv = "LINAPRO_IDC_CORE_TEST_DSN"

type secretIntegrationHarness struct {
	ctx     context.Context
	service *serviceImpl
	keys    *testKeyProvider
	scope   Scope
}

func TestSecretRecoverCreatePostgreSQL(t *testing.T) {
	harness := newSecretIntegrationHarness(t)
	const idempotencyKey = "recover-create-committed"
	created, err := harness.service.Create(harness.ctx, CreateInput{
		Scope: harness.scope, IdempotencyKey: idempotencyKey, Plaintext: []byte("recoverable-secret-value"),
	})
	if err != nil {
		t.Fatalf("create recoverable secret failed: %v", err)
	}
	recovered, err := harness.service.RecoverCreate(harness.ctx, RecoverCreateInput{
		Scope: harness.scope, IdempotencyKey: idempotencyKey,
	})
	if err != nil || recovered == nil || !recovered.Reused ||
		!recovered.Handle.Equal(created.Handle) || recovered.Metadata != created.Metadata {
		t.Fatalf("recover committed create mismatch: recovered=%+v err=%v", recovered, err)
	}
	replayed, err := harness.service.RecoverCreate(harness.ctx, RecoverCreateInput{
		Scope: harness.scope, IdempotencyKey: idempotencyKey,
	})
	if err != nil || replayed == nil || !replayed.Handle.Equal(created.Handle) {
		t.Fatalf("recover replay did not converge: recovered=%+v err=%v", replayed, err)
	}

	wrongScope := harness.scope
	wrongScope.CallerScope = "provider.backup"
	if _, err = harness.service.RecoverCreate(harness.ctx, RecoverCreateInput{
		Scope: wrongScope, IdempotencyKey: idempotencyKey,
	}); !bizerr.Is(err, CodeNotConfigured) {
		t.Fatalf("cross-scope create recovery must fail closed, got %v", err)
	}
	if _, err = harness.service.RecoverCreate(harness.ctx, RecoverCreateInput{
		Scope: harness.scope, IdempotencyKey: "recover-create-wrong-key",
	}); !bizerr.Is(err, CodeNotConfigured) {
		t.Fatalf("wrong-key create recovery must fail closed, got %v", err)
	}

	partialKey := "recover-create-partial"
	partialDigest, err := normalizeIdempotencyKey(partialKey)
	if err != nil {
		t.Fatalf("normalize partial recovery key failed: %v", err)
	}
	partialNo, err := newOpaqueSecretNumber(rand.Reader)
	if err != nil {
		t.Fatalf("generate partial secret number failed: %v", err)
	}
	if _, err = dao.BusinessNumber.Ctx(harness.ctx).Data(do.BusinessNumber{
		TenantId: idccap.DefaultTenantID, NumberType: secretNumberType, BusinessNo: partialNo,
		ReservationScopeDigest: digestString(harness.scope.OwnerPluginID + "\x00" + harness.scope.CallerScope),
		IdempotencyKeyDigest:   partialDigest,
	}).Insert(); err != nil {
		t.Fatalf("insert partial create reservation failed: %v", err)
	}
	if _, err = harness.service.RecoverCreate(harness.ctx, RecoverCreateInput{
		Scope: harness.scope, IdempotencyKey: partialKey,
	}); !bizerr.Is(err, CodeNotConfigured) {
		t.Fatalf("partial create recovery must fail closed, got %v", err)
	}
}

// TestSecretLifecyclePostgreSQL proves idempotency, scope isolation, callback
// lifetime, metadata minimization, versioning, and permanent revocation.
func TestSecretLifecyclePostgreSQL(t *testing.T) {
	harness := newSecretIntegrationHarness(t)
	plaintext := []byte("alpha-super-secret-1234")
	createInput := CreateInput{
		Scope:          harness.scope,
		IdempotencyKey: "integration-create-1",
		Plaintext:      plaintext,
	}
	created, err := harness.service.Create(harness.ctx, createInput)
	if err != nil {
		t.Fatalf("create encrypted secret failed: %v", err)
	}
	if created.Reused || !created.Metadata.Configured || created.Metadata.MaskedTail != "****1234" ||
		!strings.HasPrefix(created.Handle.OpaqueRef(), secretNumberPrefix) {
		t.Fatalf("unexpected create result: %+v", created)
	}
	replayed, err := harness.service.Create(harness.ctx, createInput)
	if err != nil || !replayed.Reused || !replayed.Handle.Equal(created.Handle) {
		t.Fatalf("idempotent create did not converge: result=%+v err=%v", replayed, err)
	}
	changedCreate := createInput
	changedCreate.Plaintext = []byte("different-sensitive-content")
	if _, err = harness.service.Create(harness.ctx, changedCreate); !bizerr.Is(err, CodeIdempotencyConflict) {
		t.Fatalf("changed create replay should conflict, got %v", err)
	}
	assertPersistedSecretIsEncrypted(t, harness.ctx, created.Handle.OpaqueRef(), plaintext)

	metadata, err := harness.service.Metadata(harness.ctx, harness.scope, created.Handle)
	if err != nil || metadata != created.Metadata {
		t.Fatalf("safe metadata projection changed: metadata=%+v err=%v", metadata, err)
	}
	wrongScope := harness.scope
	wrongScope.CallerScope = "provider.backup"
	foreignMetadata, err := harness.service.Metadata(harness.ctx, wrongScope, created.Handle)
	if err != nil || foreignMetadata.Configured || foreignMetadata.MaskedTail != "" || !foreignMetadata.UpdatedAt.IsZero() {
		t.Fatalf("foreign scope observed secret metadata: metadata=%+v err=%v", foreignMetadata, err)
	}

	purpose, err := secretcap.NewPurpose("provider.authentication")
	if err != nil {
		t.Fatalf("create access purpose failed: %v", err)
	}
	request, err := secretcap.NewAccessRequest(created.Handle, purpose)
	if err != nil {
		t.Fatalf("create access request failed: %v", err)
	}
	accessor, err := harness.service.Accessor(harness.scope)
	if err != nil {
		t.Fatalf("create scoped accessor failed: %v", err)
	}
	var retained []byte
	err = accessor.Access(harness.ctx, request, func(value []byte) error {
		if !bytes.Equal(value, plaintext) {
			t.Fatalf("callback received wrong plaintext: %q", value)
		}
		retained = value
		return nil
	})
	if err != nil {
		t.Fatalf("callback access failed: %v", err)
	}
	if len(retained) == 0 || !allZero(retained) {
		t.Fatalf("callback plaintext buffer was not cleared after return: %x", retained)
	}
	nonWhitelistedPurpose, err := secretcap.NewPurpose("provider.healthcheck")
	if err != nil {
		t.Fatalf("create non-whitelisted purpose failed: %v", err)
	}
	nonWhitelistedRequest, err := secretcap.NewAccessRequest(created.Handle, nonWhitelistedPurpose)
	if err != nil {
		t.Fatalf("create non-whitelisted access request failed: %v", err)
	}
	nonWhitelistedCalled := false
	if err = accessor.Access(harness.ctx, nonWhitelistedRequest, func([]byte) error {
		nonWhitelistedCalled = true
		return nil
	}); !bizerr.Is(err, CodeAccessDenied) || nonWhitelistedCalled {
		t.Fatalf("non-whitelisted purpose should fail closed: called=%v err=%v", nonWhitelistedCalled, err)
	}

	wrongAccessor, err := harness.service.Accessor(wrongScope)
	if err != nil {
		t.Fatalf("create foreign scoped accessor failed: %v", err)
	}
	foreignCalled := false
	if err = wrongAccessor.Access(harness.ctx, request, func([]byte) error {
		foreignCalled = true
		return nil
	}); !bizerr.Is(err, CodeNotConfigured) || foreignCalled {
		t.Fatalf("foreign scope access should fail closed: called=%v err=%v", foreignCalled, err)
	}

	const callbackCanary = "callback-sensitive-error-token"
	err = accessor.Access(harness.ctx, request, func([]byte) error {
		return errors.New(callbackCanary)
	})
	if !bizerr.Is(err, CodeConsumerFailed) || strings.Contains(err.Error(), callbackCanary) {
		t.Fatalf("callback error leaked sensitive text: %v", err)
	}
	const panicCanary = "callback-sensitive-panic-token"
	var panicRetained []byte
	err = accessor.Access(harness.ctx, request, func(value []byte) error {
		panicRetained = value
		panic(panicCanary)
	})
	if !bizerr.Is(err, CodeConsumerFailed) || strings.Contains(err.Error(), panicCanary) {
		t.Fatalf("callback panic escaped or leaked sensitive text: %v", err)
	}
	if len(panicRetained) == 0 || !allZero(panicRetained) {
		t.Fatalf("callback plaintext buffer was not cleared after panic: %x", panicRetained)
	}

	updatedPlaintext := []byte("updated-sensitive-secret-9876")
	updated, err := harness.service.Update(harness.ctx, UpdateInput{
		Scope:             harness.scope,
		Handle:            created.Handle,
		ExpectedUpdatedAt: created.Metadata.UpdatedAt,
		IdempotencyKey:    "integration-update-1",
		Plaintext:         updatedPlaintext,
	})
	if err != nil || updated.Reused || updated.Metadata.MaskedTail != "****9876" ||
		!updated.Metadata.UpdatedAt.After(created.Metadata.UpdatedAt) {
		t.Fatalf("update encrypted secret failed: result=%+v err=%v", updated, err)
	}
	reusedUpdate, err := harness.service.Update(harness.ctx, UpdateInput{
		Scope:             harness.scope,
		Handle:            created.Handle,
		ExpectedUpdatedAt: created.Metadata.UpdatedAt,
		IdempotencyKey:    "integration-update-1",
		Plaintext:         updatedPlaintext,
	})
	if err != nil || !reusedUpdate.Reused || reusedUpdate.Metadata != updated.Metadata {
		t.Fatalf("idempotent update did not converge: result=%+v err=%v", reusedUpdate, err)
	}
	if _, err = harness.service.Update(harness.ctx, UpdateInput{
		Scope:             harness.scope,
		Handle:            created.Handle,
		ExpectedUpdatedAt: created.Metadata.UpdatedAt,
		IdempotencyKey:    "integration-update-1",
		Plaintext:         []byte("changed-update-sensitive-content"),
	}); !bizerr.Is(err, CodeIdempotencyConflict) {
		t.Fatalf("changed update replay should conflict, got %v", err)
	}
	if _, err = harness.service.Update(harness.ctx, UpdateInput{
		Scope:             harness.scope,
		Handle:            created.Handle,
		ExpectedUpdatedAt: created.Metadata.UpdatedAt,
		IdempotencyKey:    "integration-update-stale",
		Plaintext:         []byte("stale-concurrent-write"),
	}); !bizerr.Is(err, CodeVersionConflict) {
		t.Fatalf("stale metadata timestamp should fence an update, got %v", err)
	}
	versionCount, countErr := dao.SecretVersion.Ctx(harness.ctx).Where(do.SecretVersion{
		TenantId: idccap.DefaultTenantID, SecretNo: created.Handle.OpaqueRef(),
	}).Count()
	if countErr != nil || versionCount != 2 {
		t.Fatalf("stale update appended a payload: count=%d err=%v", versionCount, countErr)
	}

	var observed []byte
	if err = accessor.Access(harness.ctx, request, func(value []byte) error {
		observed = append([]byte(nil), value...)
		return nil
	}); err != nil || !bytes.Equal(observed, updatedPlaintext) {
		t.Fatalf("access did not observe current encrypted version: got=%q err=%v", observed, err)
	}
	zeroBytes(observed)
	assertPersistedSecretIsEncrypted(t, harness.ctx, created.Handle.OpaqueRef(), updatedPlaintext)

	revoked, err := harness.service.Revoke(harness.ctx, RevokeInput{
		Scope: harness.scope, Handle: created.Handle, ExpectedUpdatedAt: updated.Metadata.UpdatedAt,
	})
	if err != nil || revoked.Configured || revoked.MaskedTail != "" || !revoked.UpdatedAt.IsZero() {
		t.Fatalf("revoke did not hide metadata: metadata=%+v err=%v", revoked, err)
	}
	secondRevocation, err := harness.service.Revoke(harness.ctx, RevokeInput{
		Scope: harness.scope, Handle: created.Handle, ExpectedUpdatedAt: updated.Metadata.UpdatedAt,
	})
	if err != nil || secondRevocation.Configured {
		t.Fatalf("repeated revocation should be idempotent: metadata=%+v err=%v", secondRevocation, err)
	}
	calledAfterRevoke := false
	if err = accessor.Access(harness.ctx, request, func([]byte) error {
		calledAfterRevoke = true
		return nil
	}); !bizerr.Is(err, CodeNotConfigured) || calledAfterRevoke {
		t.Fatalf("revoked secret callback should not run: called=%v err=%v", calledAfterRevoke, err)
	}

	var logOutput bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logOutput, nil))
	logger.Info("secret-result", "result", created, "error", err)
	logText := logOutput.String()
	for _, forbidden := range []string{
		string(plaintext), string(updatedPlaintext), created.Handle.OpaqueRef(), callbackCanary, panicCanary,
	} {
		if strings.Contains(logText, forbidden) {
			t.Fatalf("structured log leaked forbidden secret material %q: %s", forbidden, logText)
		}
	}
}

// TestGeneratedSecretPostgreSQL proves envelope-owned random credentials are
// generated once and recovered by handle under concurrent idempotent calls.
func TestGeneratedSecretPostgreSQL(t *testing.T) {
	harness := newSecretIntegrationHarness(t)
	managerValue, err := harness.service.Manager(harness.scope)
	if err != nil {
		t.Fatalf("construct generated secret manager: %v", err)
	}
	manager, ok := managerValue.(secretcap.GeneratedManager)
	if !ok {
		t.Fatal("secret manager does not expose generated credential capability")
	}
	const callers = 16
	results := make(chan secretcap.CreateResult, callers)
	errorsOut := make(chan error, callers)
	var group sync.WaitGroup
	for range callers {
		group.Add(1)
		go func() {
			defer group.Done()
			result, createErr := manager.CreateGenerated(
				harness.ctx,
				"generated-credential-concurrent",
				secretcap.GeneratedSecretPolicyURLSafe256,
			)
			if createErr != nil {
				errorsOut <- createErr
				return
			}
			results <- result
		}()
	}
	group.Wait()
	close(results)
	close(errorsOut)
	for createErr := range errorsOut {
		t.Fatalf("concurrent generated secret failed: %v", createErr)
	}
	var handle secretcap.Handle
	created := 0
	for result := range results {
		if result.Handle.IsZero() || !result.Metadata.Configured {
			t.Fatalf("invalid generated secret result: %+v", result)
		}
		if handle.IsZero() {
			handle = result.Handle
		} else if !handle.Equal(result.Handle) {
			t.Fatalf("generated secret handles diverged: %s / %s", handle, result.Handle)
		}
		if !result.Reused {
			created++
		}
	}
	if created != 1 {
		t.Fatalf("generated secret create winners=%d, want 1", created)
	}
	versionCount, err := dao.SecretVersion.Ctx(harness.ctx).
		Where(do.SecretVersion{TenantId: idccap.DefaultTenantID, SecretNo: handle.OpaqueRef()}).
		Count()
	if err != nil || versionCount != 1 {
		t.Fatalf("generated secret versions=%d err=%v", versionCount, err)
	}
	purpose, err := secretcap.NewPurpose("provider.authentication")
	if err != nil {
		t.Fatal(err)
	}
	request, err := secretcap.NewAccessRequest(handle, purpose)
	if err != nil {
		t.Fatal(err)
	}
	accessor, err := harness.service.Accessor(harness.scope)
	if err != nil {
		t.Fatal(err)
	}
	var retained []byte
	var plaintextCopy []byte
	err = accessor.Access(harness.ctx, request, func(value []byte) error {
		decoded := make([]byte, generatedURLSafe256Bytes)
		count, decodeErr := base64.RawURLEncoding.Decode(decoded, value)
		zeroBytes(decoded)
		if decodeErr != nil || count != generatedURLSafe256Bytes {
			return errors.New("generated credential policy mismatch")
		}
		plaintextCopy = append([]byte(nil), value...)
		retained = value
		return nil
	})
	if err != nil {
		t.Fatalf("access generated credential: %v", err)
	}
	if len(retained) == 0 || !allZero(retained) {
		t.Fatalf("generated callback buffer was not cleared: %x", retained)
	}
	defer zeroBytes(plaintextCopy)
	assertPersistedSecretIsEncrypted(t, harness.ctx, handle.OpaqueRef(), plaintextCopy)
	if _, err = harness.service.Create(harness.ctx, CreateInput{
		Scope:          harness.scope,
		IdempotencyKey: "generated-credential-concurrent",
		Plaintext:      []byte("caller-controlled-collision"),
	}); !bizerr.Is(err, CodeIdempotencyConflict) {
		t.Fatalf("plain create collided with generated policy without conflict: %v", err)
	}
	if _, err = manager.CreateGenerated(
		harness.ctx,
		"generated-credential-invalid-policy",
		"caller-chosen",
	); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("invalid generated policy returned %v", err)
	}
}

// TestSecretReplayContextPostgreSQL proves replay authentication binds the
// operation, idempotency digest, and original optimistic version fence.
func TestSecretReplayContextPostgreSQL(t *testing.T) {
	harness := newSecretIntegrationHarness(t)
	created, err := harness.service.Create(harness.ctx, CreateInput{
		Scope:          harness.scope,
		IdempotencyKey: "replay-operation-key",
		Plaintext:      []byte("replay-context-secret-v1"),
	})
	if err != nil {
		t.Fatalf("create replay-context fixture failed: %v", err)
	}
	if _, err = harness.service.Update(harness.ctx, UpdateInput{
		Scope:             harness.scope,
		Handle:            created.Handle,
		ExpectedUpdatedAt: created.Metadata.UpdatedAt,
		IdempotencyKey:    "replay-operation-key",
		Plaintext:         []byte("replay-context-secret-v1"),
	}); !bizerr.Is(err, CodeIdempotencyConflict) {
		t.Fatalf("cross-operation replay should conflict, got %v", err)
	}

	updatedPlaintext := []byte("replay-context-secret-v2")
	updated, err := harness.service.Update(harness.ctx, UpdateInput{
		Scope:             harness.scope,
		Handle:            created.Handle,
		ExpectedUpdatedAt: created.Metadata.UpdatedAt,
		IdempotencyKey:    "replay-fence-original",
		Plaintext:         updatedPlaintext,
	})
	if err != nil {
		t.Fatalf("create replay-fence fixture failed: %v", err)
	}
	if _, err = harness.service.Update(harness.ctx, UpdateInput{
		Scope:             harness.scope,
		Handle:            created.Handle,
		ExpectedUpdatedAt: updated.Metadata.UpdatedAt,
		IdempotencyKey:    "replay-fence-original",
		Plaintext:         updatedPlaintext,
	}); !bizerr.Is(err, CodeIdempotencyConflict) {
		t.Fatalf("same idempotency key under a different optimistic fence should conflict, got %v", err)
	}

	tamperedDigest := digestString("replay-fence-tampered")
	result, err := dao.SecretVersion.Ctx(harness.ctx).
		Where(do.SecretVersion{
			TenantId: idccap.DefaultTenantID,
			SecretNo: created.Handle.OpaqueRef(),
			Version:  2,
		}).
		Data(do.SecretVersion{IdempotencyKeyDigest: tamperedDigest}).
		Update()
	if err != nil {
		t.Fatalf("tamper persisted idempotency digest fixture failed: %v", err)
	}
	if err = requireOneRow(result, "tamper persisted idempotency digest"); err != nil {
		t.Fatalf("tamper persisted idempotency digest row count failed: %v", err)
	}
	if _, err = harness.service.Update(harness.ctx, UpdateInput{
		Scope:             harness.scope,
		Handle:            created.Handle,
		ExpectedUpdatedAt: created.Metadata.UpdatedAt,
		IdempotencyKey:    "replay-fence-tampered",
		Plaintext:         updatedPlaintext,
	}); !bizerr.Is(err, CodeIdempotencyConflict) {
		t.Fatalf("authenticated digest tamper should conflict, got %v", err)
	}
	versionCount, countErr := dao.SecretVersion.Ctx(harness.ctx).Where(do.SecretVersion{
		TenantId: idccap.DefaultTenantID,
		SecretNo: created.Handle.OpaqueRef(),
	}).Count()
	if countErr != nil || versionCount != 2 {
		t.Fatalf("replay conflicts appended an unexpected payload: count=%d err=%v", versionCount, countErr)
	}
}

// TestSecretAccessRejectsAggregateRollbackPostgreSQL proves a stale aggregate
// pointer cannot make Access decrypt an older immutable payload version.
func TestSecretAccessRejectsAggregateRollbackPostgreSQL(t *testing.T) {
	harness := newSecretIntegrationHarness(t)
	created, err := harness.service.Create(harness.ctx, CreateInput{
		Scope: harness.scope, IdempotencyKey: "rollback-watermark-create", Plaintext: []byte("rollback-watermark-v1"),
	})
	if err != nil {
		t.Fatalf("create rollback watermark fixture failed: %v", err)
	}
	if _, err = harness.service.Update(harness.ctx, UpdateInput{
		Scope:             harness.scope,
		Handle:            created.Handle,
		ExpectedUpdatedAt: created.Metadata.UpdatedAt,
		IdempotencyKey:    "rollback-watermark-update",
		Plaintext:         []byte("rollback-watermark-v2"),
	}); err != nil {
		t.Fatalf("update rollback watermark fixture failed: %v", err)
	}
	result, err := dao.Secret.Ctx(harness.ctx).
		Where(do.Secret{TenantId: idccap.DefaultTenantID, SecretNo: created.Handle.OpaqueRef()}).
		Data(do.Secret{CurrentVersion: 1}).
		Update()
	if err != nil {
		t.Fatalf("install aggregate rollback fixture failed: %v", err)
	}
	if err = requireOneRow(result, "install aggregate rollback fixture"); err != nil {
		t.Fatalf("install aggregate rollback fixture row count failed: %v", err)
	}
	purpose, err := secretcap.NewPurpose("provider.authentication")
	if err != nil {
		t.Fatalf("create rollback access purpose failed: %v", err)
	}
	request, err := secretcap.NewAccessRequest(created.Handle, purpose)
	if err != nil {
		t.Fatalf("create rollback access request failed: %v", err)
	}
	accessor, err := harness.service.Accessor(harness.scope)
	if err != nil {
		t.Fatalf("create rollback accessor failed: %v", err)
	}
	called := false
	if err = accessor.Access(harness.ctx, request, func([]byte) error {
		called = true
		return nil
	}); !bizerr.Is(err, CodeUnavailable) || called {
		t.Fatalf("aggregate rollback should fail before callback: called=%v err=%v", called, err)
	}
}

// TestSecretRotationConcurrentAndRecoverablePostgreSQL proves a failed batch
// leaves the old path intact, concurrent workers converge, and new writes never
// encrypt with the retired key.
func TestSecretRotationConcurrentAndRecoverablePostgreSQL(t *testing.T) {
	harness := newSecretIntegrationHarness(t)
	created, err := harness.service.Create(harness.ctx, CreateInput{
		Scope: harness.scope, IdempotencyKey: "rotation-create", Plaintext: []byte("rotation-secret-value"),
	})
	if err != nil {
		t.Fatalf("create rotation fixture failed: %v", err)
	}
	historicalUpdate := UpdateInput{
		Scope:             harness.scope,
		Handle:            created.Handle,
		ExpectedUpdatedAt: created.Metadata.UpdatedAt,
		IdempotencyKey:    "rotation-update-v1",
		Plaintext:         []byte("historical-update-under-v1"),
	}
	historicalUpdated, err := harness.service.Update(harness.ctx, historicalUpdate)
	if err != nil {
		t.Fatalf("create historical v1 update failed: %v", err)
	}
	harness.keys.SetActive("master-v2", bytes.Repeat([]byte{0x22}, dekSize))
	installRotationFailureTrigger(t, harness.ctx)
	const triggerCanary = "rotation-trigger-sensitive-canary"
	if _, err = harness.service.Rotate(harness.ctx, RotateInput{Scope: harness.scope, Limit: 10}); !bizerr.Is(err, CodeUnavailable) || strings.Contains(err.Error(), triggerCanary) {
		t.Fatalf("failed rotation did not collapse safely: %v", err)
	}
	assertSecretWrapCount(t, harness.ctx, created.Handle.OpaqueRef(), 1, "master-v2", 0)
	assertSecretWrapCount(t, harness.ctx, created.Handle.OpaqueRef(), 2, "master-v2", 0)
	dropRotationFailureTrigger(t, harness.ctx)

	const workers = 8
	start := make(chan struct{})
	results := make(chan RotateResult, workers)
	errorsChannel := make(chan error, workers)
	var wait sync.WaitGroup
	for range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			result, rotateErr := harness.service.Rotate(
				harness.ctx,
				RotateInput{Scope: harness.scope, Limit: 10},
			)
			results <- result
			errorsChannel <- rotateErr
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	close(errorsChannel)
	for rotateErr := range errorsChannel {
		if rotateErr != nil {
			t.Fatalf("concurrent rotation failed: %v", rotateErr)
		}
	}
	totalRewrapped := 0
	for result := range results {
		totalRewrapped += result.Rewrapped
	}
	if totalRewrapped != 2 {
		t.Fatalf("concurrent rotation should insert two wrappers, inserted %d", totalRewrapped)
	}
	assertSecretWrapCount(t, harness.ctx, created.Handle.OpaqueRef(), 1, "master-v1", 1)
	assertSecretWrapCount(t, harness.ctx, created.Handle.OpaqueRef(), 1, "master-v2", 1)
	assertSecretWrapCount(t, harness.ctx, created.Handle.OpaqueRef(), 2, "master-v1", 1)
	assertSecretWrapCount(t, harness.ctx, created.Handle.OpaqueRef(), 2, "master-v2", 1)
	stable, err := harness.service.Rotate(harness.ctx, RotateInput{Scope: harness.scope, Limit: 10})
	if err != nil || stable.Rewrapped != 0 || stable.Remaining {
		t.Fatalf("repeated rotation should be idempotent: result=%+v err=%v", stable, err)
	}

	harness.keys.Remove("master-v1")
	replayedCreate, err := harness.service.Create(harness.ctx, CreateInput{
		Scope: harness.scope, IdempotencyKey: "rotation-create", Plaintext: []byte("rotation-secret-value"),
	})
	if err != nil || !replayedCreate.Reused || !replayedCreate.Handle.Equal(created.Handle) {
		t.Fatalf("create replay still depended on retired key: result=%+v err=%v", replayedCreate, err)
	}
	replayedUpdate, err := harness.service.Update(harness.ctx, historicalUpdate)
	if err != nil || !replayedUpdate.Reused {
		t.Fatalf("update replay still depended on retired key: result=%+v err=%v", replayedUpdate, err)
	}
	changedHistoricalUpdate := historicalUpdate
	changedHistoricalUpdate.Plaintext = []byte("changed-after-old-key-retirement")
	if _, err = harness.service.Update(harness.ctx, changedHistoricalUpdate); !bizerr.Is(err, CodeIdempotencyConflict) {
		t.Fatalf("changed historical replay should conflict after old-key retirement, got %v", err)
	}

	updated, err := harness.service.Update(harness.ctx, UpdateInput{
		Scope:             harness.scope,
		Handle:            created.Handle,
		ExpectedUpdatedAt: historicalUpdated.Metadata.UpdatedAt,
		IdempotencyKey:    "rotation-update-v2",
		Plaintext:         []byte("written-only-with-active-v2"),
	})
	if err != nil || !updated.Metadata.Configured {
		t.Fatalf("update after rotation failed: result=%+v err=%v", updated, err)
	}
	assertSecretWrapCount(t, harness.ctx, created.Handle.OpaqueRef(), 3, "master-v1", 0)
	assertSecretWrapCount(t, harness.ctx, created.Handle.OpaqueRef(), 3, "master-v2", 1)
}

// TestSecretRotationFencesPausedOldKeyWriterPostgreSQL uses two service
// instances to prove an old-key write holding the shared state lock commits
// before, and is authenticated by, the rotation that returns Remaining=false.
func TestSecretRotationFencesPausedOldKeyWriterPostgreSQL(t *testing.T) {
	harness := newSecretIntegrationHarness(t)
	created, err := harness.service.Create(harness.ctx, CreateInput{
		Scope: harness.scope, IdempotencyKey: "paused-writer-create", Plaintext: []byte("paused-writer-v1"),
	})
	if err != nil {
		t.Fatalf("create paused-writer fixture failed: %v", err)
	}
	rotationService, err := newService(Config{}, harness.keys, testAuthorizer{}, time.Now, rand.Reader)
	if err != nil {
		t.Fatalf("create second rotation service instance failed: %v", err)
	}
	entered, release := harness.keys.PauseActiveAfter(2)
	type updateOutcome struct {
		result *UpdateResult
		err    error
	}
	updateDone := make(chan updateOutcome, 1)
	go func() {
		result, updateErr := harness.service.Update(harness.ctx, UpdateInput{
			Scope:             harness.scope,
			Handle:            created.Handle,
			ExpectedUpdatedAt: created.Metadata.UpdatedAt,
			IdempotencyKey:    "paused-writer-update",
			Plaintext:         []byte("paused-old-key-write"),
		})
		updateDone <- updateOutcome{result: result, err: updateErr}
	}()
	waitSecretTestSignal(t, entered, "paused old-key writer did not reach the locked Active check")
	harness.keys.SetActive("master-v2", bytes.Repeat([]byte{0x22}, dekSize))
	observedV2 := harness.keys.ObserveActiveID("master-v2")
	type rotateOutcome struct {
		result RotateResult
		err    error
	}
	rotateDone := make(chan rotateOutcome, 1)
	go func() {
		result, rotateErr := rotationService.Rotate(
			harness.ctx,
			RotateInput{Scope: harness.scope, Limit: 10},
		)
		rotateDone <- rotateOutcome{result: result, err: rotateErr}
	}()
	waitSecretTestSignal(t, observedV2, "rotation did not observe the new active key")
	close(release)
	update := <-updateDone
	if update.err != nil || update.result == nil {
		t.Fatalf("paused old-key update failed: result=%+v err=%v", update.result, update.err)
	}
	rotation := <-rotateDone
	if rotation.err != nil || rotation.result.Remaining {
		t.Fatalf("rotation did not certify the paused write: result=%+v err=%v", rotation.result, rotation.err)
	}
	assertSecretWrapCount(t, harness.ctx, created.Handle.OpaqueRef(), 2, "master-v1", 1)
	assertSecretWrapCount(t, harness.ctx, created.Handle.OpaqueRef(), 2, "master-v2", 1)
	harness.keys.Remove("master-v1")
	assertSecretAccessValue(t, harness, created.Handle, []byte("paused-old-key-write"))
}

// TestSecretRotationRejectsConcurrentActiveWrapperTamperPostgreSQL pauses a
// rotation before it acquires the database fence, commits a valid-hex active
// wrapper mutation, and proves the resumed pass cannot report completion.
func TestSecretRotationRejectsConcurrentActiveWrapperTamperPostgreSQL(t *testing.T) {
	harness := newSecretIntegrationHarness(t)
	created, err := harness.service.Create(harness.ctx, CreateInput{
		Scope: harness.scope, IdempotencyKey: "active-tamper-create", Plaintext: []byte("active-tamper-value"),
	})
	if err != nil {
		t.Fatalf("create active-wrapper tamper fixture failed: %v", err)
	}
	harness.keys.SetActive("master-v2", bytes.Repeat([]byte{0x22}, dekSize))
	initial, err := harness.service.Rotate(harness.ctx, RotateInput{Scope: harness.scope, Limit: 10})
	if err != nil || initial.Remaining {
		t.Fatalf("initial active-wrapper rotation failed: result=%+v err=%v", initial, err)
	}
	entered, release := harness.keys.PauseActiveAfter(1)
	type rotateOutcome struct {
		result RotateResult
		err    error
	}
	rotateDone := make(chan rotateOutcome, 1)
	go func() {
		result, rotateErr := harness.service.Rotate(
			harness.ctx,
			RotateInput{Scope: harness.scope, Limit: 10},
		)
		rotateDone <- rotateOutcome{result: result, err: rotateErr}
	}()
	waitSecretTestSignal(t, entered, "active-wrapper verification did not reach the pause")
	var wrapper *entity.SecretKeyWrap
	if err = dao.SecretKeyWrap.Ctx(harness.ctx).Where(do.SecretKeyWrap{
		TenantId:      idccap.DefaultTenantID,
		SecretNo:      created.Handle.OpaqueRef(),
		SecretVersion: 1,
		MasterKeyId:   "master-v2",
	}).Scan(&wrapper); err != nil || wrapper == nil {
		t.Fatalf("load active wrapper tamper fixture failed: row=%+v err=%v", wrapper, err)
	}
	tampered := []byte(wrapper.WrappedDek)
	if tampered[0] == '0' {
		tampered[0] = '1'
	} else {
		tampered[0] = '0'
	}
	result, err := dao.SecretKeyWrap.Ctx(harness.ctx).
		Where(do.SecretKeyWrap{Id: wrapper.Id}).
		Data(do.SecretKeyWrap{WrappedDek: string(tampered)}).
		Update()
	if err != nil {
		t.Fatalf("commit concurrent active-wrapper tamper failed: %v", err)
	}
	if err = requireOneRow(result, "tamper active secret wrapper"); err != nil {
		t.Fatalf("tamper active secret wrapper row count failed: %v", err)
	}
	close(release)
	rotation := <-rotateDone
	if !bizerr.Is(rotation.err, CodeUnavailable) ||
		(rotation.err == nil && !rotation.result.Remaining) {
		t.Fatalf("tampered active wrapper was reported as certified: result=%+v err=%v", rotation.result, rotation.err)
	}
	state, stateErr := loadRotationState(harness.ctx, harness.scope, false)
	if stateErr != nil || state == nil || state.StatusCode != rotationStatusRotating {
		t.Fatalf("tamper did not leave rotation certification invalidated: state=%+v err=%v", state, stateErr)
	}
}

// TestSecretCreateRollbackPostgreSQL proves a wrapper write failure rolls back
// the number reservation, aggregate, payload, and wrapper as one unit.
func TestSecretCreateRollbackPostgreSQL(t *testing.T) {
	harness := newSecretIntegrationHarness(t)
	installRotationFailureTrigger(t, harness.ctx)
	input := CreateInput{
		Scope: harness.scope, IdempotencyKey: "rollback-create", Plaintext: []byte("rollback-sensitive-value"),
	}
	if _, err := harness.service.Create(harness.ctx, input); !bizerr.Is(err, CodeUnavailable) {
		t.Fatalf("triggered create should return unavailable, got %v", err)
	}
	for _, table := range []string{
		dao.BusinessNumber.Table(), dao.Secret.Table(), dao.SecretVersion.Table(), dao.SecretKeyWrap.Table(),
	} {
		count, err := dao.Secret.DB().Model(table).Safe().Ctx(harness.ctx).Count()
		if err != nil || count != 0 {
			t.Fatalf("transaction rollback left rows in %s: count=%d err=%v", table, count, err)
		}
	}
	dropRotationFailureTrigger(t, harness.ctx)
	created, err := harness.service.Create(harness.ctx, input)
	if err != nil || !created.Metadata.Configured {
		t.Fatalf("create did not recover after rollback: result=%+v err=%v", created, err)
	}
}

// TestSecretTamperAndWrongKeyPostgreSQL proves persisted valid-hex mutation and
// an incorrect deployment key both fail authentication before callback use.
func TestSecretTamperAndWrongKeyPostgreSQL(t *testing.T) {
	harness := newSecretIntegrationHarness(t)
	purpose, err := secretcap.NewPurpose("provider.authentication")
	if err != nil {
		t.Fatalf("create tamper-test purpose failed: %v", err)
	}
	accessor, err := harness.service.Accessor(harness.scope)
	if err != nil {
		t.Fatalf("create tamper-test accessor failed: %v", err)
	}
	tamperedSecret, err := harness.service.Create(harness.ctx, CreateInput{
		Scope: harness.scope, IdempotencyKey: "tamper-create", Plaintext: []byte("tamper-sensitive-value"),
	})
	if err != nil {
		t.Fatalf("create tamper fixture failed: %v", err)
	}
	var version *entity.SecretVersion
	if err = dao.SecretVersion.Ctx(harness.ctx).Where(do.SecretVersion{
		TenantId: idccap.DefaultTenantID, SecretNo: tamperedSecret.Handle.OpaqueRef(), Version: 1,
	}).Scan(&version); err != nil || version == nil {
		t.Fatalf("load tamper fixture failed: row=%+v err=%v", version, err)
	}
	tamperedCiphertext := []byte(version.Ciphertext)
	if tamperedCiphertext[0] == '0' {
		tamperedCiphertext[0] = '1'
	} else {
		tamperedCiphertext[0] = '0'
	}
	result, err := dao.SecretVersion.Ctx(harness.ctx).
		Where(do.SecretVersion{
			TenantId: idccap.DefaultTenantID, SecretNo: tamperedSecret.Handle.OpaqueRef(), Version: 1,
		}).
		Data(do.SecretVersion{Ciphertext: string(tamperedCiphertext)}).Update()
	if err != nil {
		t.Fatalf("tamper encrypted payload fixture failed: %v", err)
	}
	if err = requireOneRow(result, "tamper secret test payload"); err != nil {
		t.Fatalf("tamper encrypted payload fixture row count failed: %v", err)
	}
	tamperedRequest, err := secretcap.NewAccessRequest(tamperedSecret.Handle, purpose)
	if err != nil {
		t.Fatalf("create tampered access request failed: %v", err)
	}
	tamperedCalled := false
	if err = accessor.Access(harness.ctx, tamperedRequest, func([]byte) error {
		tamperedCalled = true
		return nil
	}); !bizerr.Is(err, CodeUnavailable) || tamperedCalled {
		t.Fatalf("tampered payload should fail authentication: called=%v err=%v", tamperedCalled, err)
	}

	wrongKeySecret, err := harness.service.Create(harness.ctx, CreateInput{
		Scope: harness.scope, IdempotencyKey: "wrong-key-create", Plaintext: []byte("wrong-key-sensitive-value"),
	})
	if err != nil {
		t.Fatalf("create wrong-key fixture failed: %v", err)
	}
	harness.keys.Replace("master-v1", bytes.Repeat([]byte{0x33}, dekSize))
	wrongKeyRequest, err := secretcap.NewAccessRequest(wrongKeySecret.Handle, purpose)
	if err != nil {
		t.Fatalf("create wrong-key access request failed: %v", err)
	}
	wrongKeyCalled := false
	if err = accessor.Access(harness.ctx, wrongKeyRequest, func([]byte) error {
		wrongKeyCalled = true
		return nil
	}); !bizerr.Is(err, CodeUnavailable) || wrongKeyCalled {
		t.Fatalf("wrong master key should fail authentication: called=%v err=%v", wrongKeyCalled, err)
	}
}

func newSecretIntegrationHarness(t *testing.T) *secretIntegrationHarness {
	t.Helper()
	ctx := context.Background()
	configureSecretIntegrationDB(t, ctx)
	keys := newTestKeyProvider("master-v1", bytes.Repeat([]byte{0x11}, dekSize))
	scope := Scope{
		TenantID: idccap.DefaultTenantID, OwnerPluginID: "provider-test", CallerScope: "provider.authentication",
	}
	service, err := newService(Config{}, keys, testAuthorizer{}, time.Now, rand.Reader)
	if err != nil {
		t.Fatalf("create secret integration service failed: %v", err)
	}
	return &secretIntegrationHarness{ctx: ctx, service: service, keys: keys, scope: scope}
}

func configureSecretIntegrationDB(t *testing.T, ctx context.Context) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(secretIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL secret integration tests", secretIntegrationDSNEnv)
	}
	adminDB, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create secret integration admin database failed: %v", err)
	}
	schema := newSecretIntegrationSchema(t)
	if _, err = adminDB.Exec(ctx, "CREATE SCHEMA "+quoteSecretPostgreSQLIdentifier(schema)); err != nil {
		if closeErr := adminDB.Close(ctx); closeErr != nil {
			t.Errorf("close secret integration admin database after schema failure: %v", closeErr)
		}
		t.Fatalf("create isolated secret schema failed: %v", err)
	}
	originalConfig := gdb.GetAllConfig()
	var scopedDB gdb.DB
	t.Cleanup(func() {
		if scopedDB != nil {
			if closeErr := scopedDB.Close(ctx); closeErr != nil {
				t.Errorf("close scoped secret database failed: %v", closeErr)
			}
		}
		if restoreErr := gdb.SetConfig(originalConfig); restoreErr != nil {
			t.Errorf("restore secret integration database config failed: %v", restoreErr)
		}
		if !strings.HasPrefix(schema, "linapro_secret_test_") {
			t.Errorf("refusing to drop unexpected secret integration schema %q", schema)
		} else if _, dropErr := adminDB.Exec(
			ctx,
			"DROP SCHEMA "+quoteSecretPostgreSQLIdentifier(schema)+" CASCADE",
		); dropErr != nil {
			t.Errorf("drop isolated secret schema failed: %v", dropErr)
		}
		if closeErr := adminDB.Close(ctx); closeErr != nil {
			t.Errorf("close secret integration admin database failed: %v", closeErr)
		}
	})
	scopedDSN := appendSecretDSNParameter(dsn, "search_path="+schema)
	if err = gdb.SetConfig(gdb.Config{gdb.DefaultGroupName: gdb.ConfigGroup{{Link: scopedDSN}}}); err != nil {
		t.Fatalf("configure secret integration database failed: %v", err)
	}
	scopedDB = g.DB()
	if _, err = scopedDB.Exec(ctx, secretIntegrationBusinessNumberDDL()); err != nil {
		t.Fatalf("create isolated secret business-number table failed: %v", err)
	}
	for _, migrationPath := range secretMigrationPaths(t) {
		migration, readErr := os.ReadFile(migrationPath)
		if readErr != nil {
			t.Fatalf("read secret migration %s failed: %v", filepath.Base(migrationPath), readErr)
		}
		for attempt := 1; attempt <= 2; attempt++ {
			if _, err = scopedDB.Exec(ctx, string(migration)); err != nil {
				t.Fatalf(
					"execute secret migration %s attempt %d failed: %v",
					filepath.Base(migrationPath), attempt, err,
				)
			}
		}
	}
	currentSchema, err := scopedDB.GetValue(ctx, "SELECT current_schema()")
	if err != nil || currentSchema.String() != schema {
		t.Fatalf("secret integration search path mismatch: got=%q want=%q err=%v", currentSchema.String(), schema, err)
	}
}

func secretIntegrationBusinessNumberDDL() string {
	return fmt.Sprintf(`CREATE TABLE %s (
        "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
        "tenant_id" INT NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
        "number_type" VARCHAR(64) NOT NULL,
        "business_no" VARCHAR(64) NOT NULL UNIQUE,
        "reservation_scope_digest" CHAR(64) NOT NULL,
        "idempotency_key_digest" CHAR(64) NOT NULL,
        "owner_type" VARCHAR(64) NOT NULL DEFAULT '',
        "owner_internal_id" BIGINT,
        "assigned_at" TIMESTAMPTZ,
        "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
        "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
        UNIQUE ("tenant_id", "number_type", "reservation_scope_digest", "idempotency_key_digest")
    )`, quoteSecretPostgreSQLIdentifier(dao.BusinessNumber.Table()))
}

func secretMigrationPaths(t *testing.T) []string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve secret integration test path failed")
	}
	sqlDirectory := filepath.Clean(filepath.Join(
		filepath.Dir(filename), "..", "..", "..", "..", "manifest", "sql",
	))
	return []string{
		filepath.Join(sqlDirectory, "002-secret-envelope.sql"),
		filepath.Join(sqlDirectory, "039-generated-secret-policy.sql"),
	}
}

func installRotationFailureTrigger(t *testing.T, ctx context.Context) {
	t.Helper()
	statement := fmt.Sprintf(`
        CREATE OR REPLACE FUNCTION reject_secret_wrap_for_test() RETURNS trigger AS $$
        BEGIN
            RAISE EXCEPTION 'rotation-trigger-sensitive-canary';
        END;
        $$ LANGUAGE plpgsql;
        CREATE TRIGGER reject_secret_wrap_for_test
        BEFORE INSERT ON %s
        FOR EACH ROW EXECUTE FUNCTION reject_secret_wrap_for_test();
    `, quoteSecretPostgreSQLIdentifier(dao.SecretKeyWrap.Table()))
	if _, err := dao.Secret.DB().Exec(ctx, statement); err != nil {
		t.Fatalf("install secret wrapper failure trigger failed: %v", err)
	}
}

func dropRotationFailureTrigger(t *testing.T, ctx context.Context) {
	t.Helper()
	statement := fmt.Sprintf(`
        DROP TRIGGER IF EXISTS reject_secret_wrap_for_test ON %s;
        DROP FUNCTION IF EXISTS reject_secret_wrap_for_test();
    `, quoteSecretPostgreSQLIdentifier(dao.SecretKeyWrap.Table()))
	if _, err := dao.Secret.DB().Exec(ctx, statement); err != nil {
		t.Fatalf("drop secret wrapper failure trigger failed: %v", err)
	}
}

func assertPersistedSecretIsEncrypted(t *testing.T, ctx context.Context, secretRef string, plaintext []byte) {
	t.Helper()
	var versions []entity.SecretVersion
	if err := dao.SecretVersion.Ctx(ctx).Where(do.SecretVersion{
		TenantId: idccap.DefaultTenantID, SecretNo: secretRef,
	}).Scan(&versions); err != nil {
		t.Fatalf("load persisted secret versions failed: %v", err)
	}
	if len(versions) == 0 {
		t.Fatal("encrypted secret has no payload version")
	}
	for _, version := range versions {
		if bytes.Contains(mustDecodePersistedHex(version.Ciphertext), plaintext) || version.RequestFingerprint == string(plaintext) ||
			version.IdempotencyKeyDigest == string(plaintext) {
			t.Fatalf("persisted payload contains plaintext: %+v", version)
		}
	}
	var wrappers []entity.SecretKeyWrap
	if err := dao.SecretKeyWrap.Ctx(ctx).Where(do.SecretKeyWrap{
		TenantId: idccap.DefaultTenantID, SecretNo: secretRef,
	}).Scan(&wrappers); err != nil {
		t.Fatalf("load persisted secret wrappers failed: %v", err)
	}
	for _, wrapper := range wrappers {
		if bytes.Contains(mustDecodePersistedHex(wrapper.WrappedDek), plaintext) {
			t.Fatalf("persisted DEK wrapper contains plaintext: %+v", wrapper)
		}
	}
}

func assertSecretWrapCount(
	t *testing.T,
	ctx context.Context,
	secretRef string,
	version int,
	keyID string,
	want int,
) {
	t.Helper()
	count, err := dao.SecretKeyWrap.Ctx(ctx).Where(do.SecretKeyWrap{
		TenantId: idccap.DefaultTenantID, SecretNo: secretRef, SecretVersion: version, MasterKeyId: keyID,
	}).Count()
	if err != nil || count != want {
		t.Fatalf("secret wrapper count mismatch: ref=%s version=%d key=%s got=%d want=%d err=%v", secretRef, version, keyID, count, want, err)
	}
}

func assertSecretAccessValue(
	t *testing.T,
	harness *secretIntegrationHarness,
	handle secretcap.Handle,
	want []byte,
) {
	t.Helper()
	purpose, err := secretcap.NewPurpose("provider.authentication")
	if err != nil {
		t.Fatalf("create secret access assertion purpose failed: %v", err)
	}
	request, err := secretcap.NewAccessRequest(handle, purpose)
	if err != nil {
		t.Fatalf("create secret access assertion request failed: %v", err)
	}
	accessor, err := harness.service.Accessor(harness.scope)
	if err != nil {
		t.Fatalf("create secret access assertion accessor failed: %v", err)
	}
	var observed []byte
	if err = accessor.Access(harness.ctx, request, func(value []byte) error {
		observed = append([]byte(nil), value...)
		return nil
	}); err != nil || !bytes.Equal(observed, want) {
		t.Fatalf("secret access assertion mismatch: got=%q want=%q err=%v", observed, want, err)
	}
	zeroBytes(observed)
}

func waitSecretTestSignal(t *testing.T, signal <-chan struct{}, failure string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(5 * time.Second):
		t.Fatal(failure)
	}
}

func newSecretIntegrationSchema(t *testing.T) string {
	t.Helper()
	var content [8]byte
	if _, err := rand.Read(content[:]); err != nil {
		t.Fatalf("generate secret integration schema failed: %v", err)
	}
	return "linapro_secret_test_" + hex.EncodeToString(content[:])
}

func appendSecretDSNParameter(dsn string, parameter string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + parameter
}

func quoteSecretPostgreSQLIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func allZero(value []byte) bool {
	for _, character := range value {
		if character != 0 {
			return false
		}
	}
	return true
}

type testAuthorizer struct{}

func (testAuthorizer) Authorize(_ context.Context, request AuthorizationRequest) error {
	if request.Action == ActionAccess && request.Purpose != secretcap.Purpose("provider.authentication") {
		return errors.New("access purpose is not whitelisted for caller scope")
	}
	return nil
}

type testKeyProvider struct {
	mutex          sync.RWMutex
	activeID       string
	keys           map[string][]byte
	activeCalls    int
	pauseAtCall    int
	pauseEntered   chan struct{}
	pauseRelease   chan struct{}
	observeID      string
	observeChannel chan struct{}
}

func newTestKeyProvider(activeID string, key []byte) *testKeyProvider {
	return &testKeyProvider{activeID: activeID, keys: map[string][]byte{activeID: append([]byte(nil), key...)}}
}

func (p *testKeyProvider) Active(context.Context) (KeyMaterial, error) {
	p.mutex.Lock()
	p.activeCalls++
	key := p.keys[p.activeID]
	if len(key) != dekSize {
		p.mutex.Unlock()
		return KeyMaterial{}, errors.New("active test key is unavailable")
	}
	material := KeyMaterial{ID: p.activeID, Key: append([]byte(nil), key...)}
	var release <-chan struct{}
	if p.pauseAtCall == p.activeCalls {
		close(p.pauseEntered)
		release = p.pauseRelease
		p.pauseAtCall = 0
		p.pauseEntered = nil
		p.pauseRelease = nil
	}
	if p.observeID == material.ID && p.observeChannel != nil {
		close(p.observeChannel)
		p.observeID = ""
		p.observeChannel = nil
	}
	p.mutex.Unlock()
	if release != nil {
		<-release
	}
	return material, nil
}

func (p *testKeyProvider) Resolve(_ context.Context, keyIDs []string) (map[string][]byte, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	result := make(map[string][]byte, len(keyIDs))
	for _, keyID := range keyIDs {
		if key := p.keys[keyID]; len(key) == dekSize {
			result[keyID] = append([]byte(nil), key...)
		}
	}
	return result, nil
}

func (p *testKeyProvider) SetActive(keyID string, key []byte) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.keys[keyID] = append([]byte(nil), key...)
	p.activeID = keyID
}

func (p *testKeyProvider) Remove(keyID string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if key := p.keys[keyID]; len(key) > 0 {
		zeroBytes(key)
	}
	delete(p.keys, keyID)
}

func (p *testKeyProvider) Replace(keyID string, key []byte) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if existing := p.keys[keyID]; len(existing) > 0 {
		zeroBytes(existing)
	}
	p.keys[keyID] = append([]byte(nil), key...)
}

func (p *testKeyProvider) PauseActiveAfter(calls int) (<-chan struct{}, chan<- struct{}) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if calls <= 0 || p.pauseAtCall != 0 {
		panic("invalid or overlapping test key-provider pause")
	}
	p.pauseAtCall = p.activeCalls + calls
	p.pauseEntered = make(chan struct{})
	p.pauseRelease = make(chan struct{})
	return p.pauseEntered, p.pauseRelease
}

func (p *testKeyProvider) ObserveActiveID(keyID string) <-chan struct{} {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if p.observeChannel != nil {
		panic("overlapping test key-provider observation")
	}
	p.observeID = keyID
	p.observeChannel = make(chan struct{})
	return p.observeChannel
}

var _ KeyProvider = (*testKeyProvider)(nil)
var _ Authorizer = testAuthorizer{}
