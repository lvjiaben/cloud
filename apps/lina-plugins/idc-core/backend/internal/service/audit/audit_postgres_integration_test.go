// This file verifies audit transaction, idempotency, redaction, and read-scope invariants in a random PostgreSQL schema.

package audit

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const auditIntegrationDSNEnv = "LINAPRO_IDC_CORE_TEST_DSN"

type auditActorContextKey struct{}

type contextActorResolver struct{}

func (contextActorResolver) ResolveTrustedActor(ctx context.Context) (TrustedActor, error) {
	actor, ok := ctx.Value(auditActorContextKey{}).(TrustedActor)
	if !ok {
		return TrustedActor{}, errors.New("missing integration actor")
	}
	return actor, nil
}

type integrationClock struct {
	mu    sync.Mutex
	value time.Time
}

func (c *integrationClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value
}

func (c *integrationClock) Add(duration time.Duration) {
	c.mu.Lock()
	c.value = c.value.Add(duration)
	c.mu.Unlock()
}

// TestAuditPostgresLifecycle verifies caller transaction reuse, rollback,
// complete immutable replay, owned observations, HMAC storage, and scoped search.
func TestAuditPostgresLifecycle(t *testing.T) {
	ctx := context.Background()
	configureAuditIntegrationDB(t, ctx)
	clock := &integrationClock{value: time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)}
	config := testAuditConfig()
	config.TrustedProxyCIDRs = []string{"10.0.0.0/8"}
	config.MaximumPageSize = 10
	service, err := newService(
		contextActorResolver{},
		staticAuthorizationService{grants: map[authz.PermissionKey]bool{
			authz.PermissionKey(permissionAuditRead): true,
		}},
		config,
		clock.Now,
		rand.Reader,
	)
	if err != nil {
		t.Fatalf("create audit integration service: %v", err)
	}
	customerCtx := withAuditIntegrationActor(ctx, TrustedActor{
		Type:         ActorUser,
		UserID:       501,
		AudienceCode: principal.AudienceCustomer,
		CustomerNo:   "CUS_INTEGRATION",
	})
	input := auditIntegrationTransition("lifecycle", "one")
	if _, err = service.AppendTransition(customerCtx, input); !bizerr.Is(err, CodeTransactionRequired) {
		t.Fatalf("append outside transaction returned %v", err)
	}

	rollbackMarker := errors.New("rollback audit integration event")
	err = dao.BusinessAuditEvent.Transaction(customerCtx, func(ctx context.Context, _ gdb.TX) error {
		if _, appendErr := service.AppendTransition(ctx, input); appendErr != nil {
			return appendErr
		}
		return rollbackMarker
	})
	if !errors.Is(err, rollbackMarker) {
		t.Fatalf("rollback append returned %v", err)
	}
	assertAuditIntegrationCounts(t, customerCtx, 0, 0)

	created := appendAuditIntegrationTransition(t, customerCtx, service, input)
	if created.AuditNo == "" || created.OccurredAt.IsZero() || created.CustomerNo != "CUS_INTEGRATION" {
		t.Fatalf("invalid created audit event: %+v", created)
	}
	clock.Add(24 * time.Hour)
	replayed := appendAuditIntegrationTransition(t, customerCtx, service, input)
	if replayed.AuditNo != created.AuditNo || !replayed.OccurredAt.Equal(created.OccurredAt) {
		t.Fatalf("audit replay changed immutable time or number: created=%+v replay=%+v", created, replayed)
	}
	assertAuditIntegrationCounts(t, customerCtx, 1, 1)

	stored := loadAuditIntegrationEvent(t, customerCtx, created.AuditNo)
	joined := stored.BeforeSnapshot + stored.AfterSnapshot + stored.ContextSnapshot
	if strings.Contains(joined, "first-secret") || !strings.Contains(joined, redactedValue) {
		t.Fatalf("persisted audit evidence was not redacted: %s", joined)
	}
	if len(stored.IpDigest) != 64 || len(stored.UserAgentDigest) != 64 ||
		strings.Contains(stored.IpDigest, "203.0.113.20") ||
		strings.Contains(stored.UserAgentDigest, "IntegrationAgent") {
		t.Fatalf("request evidence was not stored as HMAC only: %+v", stored)
	}
	reservation := loadAuditIntegrationReservation(t, customerCtx, created.AuditNo)
	if reservation.OwnerType != businessNumberOwnerAudit ||
		reservation.OwnerInternalId != stored.Id || reservation.AssignedAt == nil {
		t.Fatalf("audit number ownership was not frozen: %+v", reservation)
	}

	changed := auditIntegrationTransition("lifecycle", "one")
	changed.BeforeSnapshot = map[string]any{
		"state":       "stopped",
		"credentials": map[string]any{"api_key": "second-secret"},
	}
	err = dao.BusinessAuditEvent.Transaction(customerCtx, func(ctx context.Context, _ gdb.TX) error {
		_, appendErr := service.AppendTransition(ctx, changed)
		return appendErr
	})
	if !bizerr.Is(err, CodeIdempotencyConflict) {
		t.Fatalf("changed raw secret replay returned %v", err)
	}
	assertAuditIntegrationCounts(t, customerCtx, 1, 1)

	observationInput := ObservationInput{
		Fields: Fields{
			IdempotencyScope: "security.observation",
			IdempotencyKey:   "observation-one",
			OperationCode:    "server.console.denied",
			TargetType:       "server",
			TargetNo:         "SRV_INTEGRATION",
			Result:           ResultDenied,
			ReasonCode:       "cooldown_active",
			Risk:             RiskHigh,
		},
		Snapshot: map[string]any{"decision": "denied"},
	}
	observation, err := service.RecordObservation(customerCtx, observationInput)
	if err != nil || observation == nil || observation.AuditNo == created.AuditNo {
		t.Fatalf("record owned observation failed: event=%+v err=%v", observation, err)
	}
	err = dao.BusinessAuditEvent.Transaction(customerCtx, func(ctx context.Context, _ gdb.TX) error {
		_, observationErr := service.RecordObservation(ctx, ObservationInput{
			Fields: Fields{
				IdempotencyScope: "security.observation",
				IdempotencyKey:   "nested-observation",
				OperationCode:    "server.console.denied",
				TargetType:       "server",
				TargetNo:         "SRV_INTEGRATION",
				Result:           ResultDenied,
				ReasonCode:       "nested_transaction",
			},
		})
		return observationErr
	})
	if !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("observation inside ambient transaction returned %v", err)
	}
	assertAuditIntegrationCounts(t, customerCtx, 2, 2)

	if _, err = service.Search(customerCtx, SearchInput{}); !bizerr.Is(err, CodePermissionDenied) {
		t.Fatalf("customer audit search returned %v", err)
	}
	limitedStaffCtx := withAuditIntegrationActor(ctx, TrustedActor{
		Type:         ActorUser,
		UserID:       601,
		AudienceCode: principal.AudienceStaff,
		DataScope:    hostDataScopeDepartment,
	})
	if _, err = service.Search(limitedStaffCtx, SearchInput{}); !bizerr.Is(err, CodePermissionDenied) {
		t.Fatalf("department-scoped audit search returned %v", err)
	}
	staffCtx := withAuditIntegrationActor(ctx, TrustedActor{
		Type:         ActorUser,
		UserID:       602,
		AudienceCode: principal.AudienceStaff,
		DataScope:    hostDataScopeTenant,
	})
	firstPage, err := service.Search(staffCtx, SearchInput{CustomerNo: "CUS_INTEGRATION", Limit: 1})
	if err != nil || len(firstPage.Events) != 1 || firstPage.Next == nil {
		t.Fatalf("first audit page failed: page=%+v err=%v", firstPage, err)
	}
	secondPage, err := service.Search(staffCtx, SearchInput{
		CustomerNo: "CUS_INTEGRATION",
		Limit:      1,
		Cursor:     firstPage.Next,
	})
	if err != nil || len(secondPage.Events) != 1 || secondPage.Next != nil ||
		secondPage.Events[0].AuditNo == firstPage.Events[0].AuditNo {
		t.Fatalf("second audit page failed: page=%+v err=%v", secondPage, err)
	}
}

func TestAuditPostgresBatchCustomerLinksAreSealedSearchableAndReplaySafe(t *testing.T) {
	ctx := context.Background()
	configureAuditIntegrationDB(t, ctx)
	permission := "idc:admin:customer:tag:batch"
	service, err := newService(
		contextActorResolver{},
		staticAuthorizationService{grants: map[authz.PermissionKey]bool{
			authz.PermissionKey(permission):          true,
			authz.PermissionKey(permissionAuditRead): true,
		}},
		testAuditConfig(),
		time.Now,
		rand.Reader,
	)
	if err != nil {
		t.Fatalf("create batch audit service: %v", err)
	}
	staffCtx := withAuditIntegrationActor(ctx, TrustedActor{
		Type: ActorUser, UserID: 680, AudienceCode: principal.AudienceStaff,
		DataScope: hostDataScopeTenant,
	})
	customers := []string{
		"CUS_11111111111111111111111111111111",
		"CUS_22222222222222222222222222222222",
		"CUS_33333333333333333333333333333333",
	}
	for index, customerNo := range customers {
		if _, err = dao.CustomerSubject.Ctx(ctx).Data(do.CustomerSubject{
			TenantId: idccap.DefaultTenantID, CustomerNo: customerNo,
			CustomerTypeCode: "individual", StatusCode: "active",
			DisplayName: "Batch Customer " + strconv.Itoa(index+1),
			CountryCode: "CN", PreferredLocale: "zh-CN", Version: 1,
		}).Insert(); err != nil {
			t.Fatalf("insert batch audit customer %s: %v", customerNo, err)
		}
	}
	input := BatchTransitionInput{
		Fields: Fields{
			IdempotencyScope: "customer.tag.batch", IdempotencyKey: "batch-one",
			OperationCode: "customer.tag.batch.assign", TargetType: "customer_tag_batch",
			TargetNo: "TAG_11111111111111111111111111111111", Result: ResultSucceeded,
			ReasonCode: "manual_segment_update", Risk: RiskMedium,
			RequiredPermission: permission, CorrelationNo: "BATCH_ONE",
		},
		AffectedCustomerNos: []string{customers[1], customers[0]},
		BeforeSnapshot:      map[string]any{"state": "requested"},
		AfterSnapshot:       map[string]any{"state": "assigned"},
	}
	if _, err = service.AppendBatchTransition(staffCtx, input); !bizerr.Is(err, CodeTransactionRequired) {
		t.Fatalf("batch append outside transaction returned %v", err)
	}
	var created *AppendResult
	if err = dao.BusinessAuditEvent.Transaction(staffCtx, func(txCtx context.Context, _ gdb.TX) error {
		created, err = service.AppendBatchTransition(txCtx, input)
		return err
	}); err != nil || created == nil || created.AuditNo == "" || created.Replayed {
		t.Fatalf("append batch audit result=%+v err=%v", created, err)
	}
	var replayed *AppendResult
	if err = dao.BusinessAuditEvent.Transaction(staffCtx, func(txCtx context.Context, _ gdb.TX) error {
		replayed, err = service.AppendBatchTransition(txCtx, input)
		return err
	}); err != nil || replayed == nil || !replayed.Replayed || replayed.AuditNo != created.AuditNo {
		t.Fatalf("replay batch audit result=%+v err=%v", replayed, err)
	}
	var linkCount int
	if linkCount, err = dao.BusinessAuditCustomerLink.Ctx(ctx).
		Where(do.BusinessAuditCustomerLink{AuditNo: created.AuditNo}).Count(); err != nil || linkCount != 2 {
		t.Fatalf("batch audit link count=%d err=%v", linkCount, err)
	}
	for _, customerNo := range customers[:2] {
		page, searchErr := service.Search(staffCtx, SearchInput{CustomerNo: customerNo, Limit: 10})
		if searchErr != nil || len(page.Events) != 1 || page.Events[0].AuditNo != created.AuditNo {
			t.Fatalf("customer %s batch timeline=%+v err=%v", customerNo, page, searchErr)
		}
	}
	page, err := service.Search(staffCtx, SearchInput{CustomerNo: customers[2], Limit: 10})
	if err != nil || len(page.Events) != 0 {
		t.Fatalf("unlinked customer batch timeline=%+v err=%v", page, err)
	}
	if _, err = dao.BusinessAuditCustomerLink.Ctx(ctx).
		Where(do.BusinessAuditCustomerLink{AuditNo: created.AuditNo}).
		Data(do.BusinessAuditCustomerLink{CustomerNo: customers[0]}).Update(); err == nil {
		t.Fatal("sealed batch audit customer link was mutable")
	}
	if _, err = dao.BusinessAuditCustomerLink.Ctx(ctx).
		Data(do.BusinessAuditCustomerLink{
			TenantId: idccap.DefaultTenantID, AuditNo: created.AuditNo, CustomerNo: customers[2],
		}).Insert(); err == nil {
		t.Fatal("customer link was attachable after audit sealing")
	}
}

// TestAuditPostgresPublicEntriesRejectEncodedCredentialScalars verifies both
// write contracts reject reversible credential encodings before any audit row
// or business-number reservation can be persisted.
func TestAuditPostgresPublicEntriesRejectEncodedCredentialScalars(t *testing.T) {
	ctx := context.Background()
	configureAuditIntegrationDB(t, ctx)
	service, err := newService(
		contextActorResolver{},
		staticAuthorizationService{},
		testAuditConfig(),
		time.Now,
		rand.Reader,
	)
	if err != nil {
		t.Fatalf("create encoded-credential audit service: %v", err)
	}
	customerCtx := withAuditIntegrationActor(ctx, TrustedActor{
		Type:         ActorUser,
		UserID:       650,
		AudienceCode: principal.AudienceCustomer,
		CustomerNo:   "CUS_CREDENTIAL_MATRIX",
	})
	hexAssignment := "6170694b65793d6d7573742d6e6f742d70617373"
	base64Assignment := base64.StdEncoding.EncodeToString([]byte("apiKey=must-not-pass"))
	percentAssignment := url.QueryEscape(url.QueryEscape("apiKey=must-not-pass"))
	vectors := []struct {
		name   string
		mutate func(*Fields)
	}{
		{name: "hex-stable-code", mutate: func(fields *Fields) { fields.OperationCode = hexAssignment }},
		{name: "short-hex-stable-code", mutate: func(fields *Fields) { fields.OperationCode = "746f6b656e3d78" }},
		{name: "base64-reference", mutate: func(fields *Fields) { fields.TargetNo = base64Assignment }},
		{name: "short-base64-reference", mutate: func(fields *Fields) { fields.TargetNo = "dG9rZW49eA" }},
		{name: "percent-reference", mutate: func(fields *Fields) { fields.CorrelationNo = percentAssignment }},
		{name: "uri-userinfo", mutate: func(fields *Fields) { fields.TargetNo = "postgres://user:secret@host.invalid/db" }},
	}
	entries := []struct {
		name   string
		invoke func(context.Context, Service, Fields) error
	}{
		{
			name: "append-transition",
			invoke: func(ctx context.Context, service Service, fields Fields) error {
				return dao.BusinessAuditEvent.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
					_, appendErr := service.AppendTransition(ctx, TransitionInput{
						Fields:         fields,
						BeforeSnapshot: map[string]any{"state": "stopped"},
						AfterSnapshot:  map[string]any{"state": "running"},
					})
					return appendErr
				})
			},
		},
		{
			name: "record-observation",
			invoke: func(ctx context.Context, service Service, fields Fields) error {
				_, observationErr := service.RecordObservation(ctx, ObservationInput{
					Fields:   fields,
					Snapshot: map[string]any{"decision": "observed"},
				})
				return observationErr
			},
		},
	}
	baseFields := auditIntegrationTransition("credential-matrix", "base").Fields
	baseFields.CustomerNo = "CUS_CREDENTIAL_MATRIX"
	for _, entry := range entries {
		for _, vector := range vectors {
			t.Run(entry.name+"/"+vector.name, func(t *testing.T) {
				fields := baseFields
				fields.IdempotencyKey = entry.name + "-" + vector.name
				vector.mutate(&fields)
				if err := entry.invoke(customerCtx, service, fields); !bizerr.Is(err, CodeInvalid) {
					t.Fatalf("encoded credential scalar returned %v", err)
				}
			})
		}
	}
	assertAuditIntegrationCounts(t, ctx, 0, 0)
}

// TestAuditPostgresPublicEntriesRejectCredentialBearingSnapshotKeys verifies
// every public writer rejects property names that would preserve a secret even
// after ordinary value redaction.
func TestAuditPostgresPublicEntriesRejectCredentialBearingSnapshotKeys(t *testing.T) {
	ctx := context.Background()
	configureAuditIntegrationDB(t, ctx)
	service, err := newService(
		contextActorResolver{},
		staticAuthorizationService{},
		testAuditConfig(),
		time.Now,
		rand.Reader,
	)
	if err != nil {
		t.Fatalf("create snapshot-key audit service: %v", err)
	}
	customerCtx := withAuditIntegrationActor(ctx, TrustedActor{
		Type:         ActorUser,
		UserID:       652,
		AudienceCode: principal.AudienceCustomer,
		CustomerNo:   "CUS_SNAPSHOT_KEYS",
	})
	keys := []string{
		"apiKey=must-not-pass",
		base64.StdEncoding.EncodeToString([]byte("apiKey=must-not-pass")),
		"6170694b65793d6d7573742d6e6f742d70617373",
	}
	entries := []struct {
		name   string
		invoke func(context.Context, Service, string, string) error
	}{
		{
			name: "append-before",
			invoke: func(ctx context.Context, service Service, key string, idempotencyKey string) error {
				input := auditIntegrationTransition("snapshot-key", idempotencyKey)
				input.CustomerNo = "CUS_SNAPSHOT_KEYS"
				input.BeforeSnapshot = map[string]any{key: "ordinary"}
				return dao.BusinessAuditEvent.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
					_, appendErr := service.AppendTransition(ctx, input)
					return appendErr
				})
			},
		},
		{
			name: "append-after",
			invoke: func(ctx context.Context, service Service, key string, idempotencyKey string) error {
				input := auditIntegrationTransition("snapshot-key", idempotencyKey)
				input.CustomerNo = "CUS_SNAPSHOT_KEYS"
				input.AfterSnapshot = map[string]any{key: "ordinary"}
				return dao.BusinessAuditEvent.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
					_, appendErr := service.AppendTransition(ctx, input)
					return appendErr
				})
			},
		},
		{
			name: "append-context",
			invoke: func(ctx context.Context, service Service, key string, idempotencyKey string) error {
				input := auditIntegrationTransition("snapshot-key", idempotencyKey)
				input.CustomerNo = "CUS_SNAPSHOT_KEYS"
				input.ContextSnapshot = map[string]any{key: "ordinary"}
				return dao.BusinessAuditEvent.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
					_, appendErr := service.AppendTransition(ctx, input)
					return appendErr
				})
			},
		},
		{
			name: "record-observation",
			invoke: func(ctx context.Context, service Service, key string, idempotencyKey string) error {
				input := auditIntegrationTransition("snapshot-key", idempotencyKey)
				input.CustomerNo = "CUS_SNAPSHOT_KEYS"
				_, observationErr := service.RecordObservation(ctx, ObservationInput{
					Fields:   input.Fields,
					Snapshot: map[string]any{key: "ordinary"},
				})
				return observationErr
			},
		},
	}
	for entryIndex, entry := range entries {
		for keyIndex, key := range keys {
			t.Run(entry.name+"/"+strconv.Itoa(keyIndex), func(t *testing.T) {
				idempotencyKey := entry.name + "-" + strconv.Itoa(entryIndex) + "-" + strconv.Itoa(keyIndex)
				if err := entry.invoke(customerCtx, service, key, idempotencyKey); !bizerr.Is(err, CodeInvalid) {
					t.Fatalf("credential-bearing snapshot key returned %v", err)
				}
			})
		}
	}
	assertAuditIntegrationCounts(t, ctx, 0, 0)
}

// TestAuditPostgresPublicEntriesRejectCredentialCustomerBackfill verifies a
// trusted resolver cannot inject a credential-bearing customer number after
// the caller-owned fields have already passed their initial validation.
func TestAuditPostgresPublicEntriesRejectCredentialCustomerBackfill(t *testing.T) {
	ctx := context.Background()
	configureAuditIntegrationDB(t, ctx)
	service, err := newService(
		contextActorResolver{},
		staticAuthorizationService{},
		testAuditConfig(),
		time.Now,
		rand.Reader,
	)
	if err != nil {
		t.Fatalf("create customer-backfill audit service: %v", err)
	}
	credentialCustomerNo := "6170694b65793d6d7573742d6e6f742d70617373"
	customerCtx := withAuditIntegrationActor(ctx, TrustedActor{
		Type:         ActorUser,
		UserID:       651,
		AudienceCode: principal.AudienceCustomer,
		CustomerNo:   credentialCustomerNo,
	})
	baseFields := auditIntegrationTransition("credential-backfill", "base").Fields
	baseFields.CustomerNo = ""
	entries := []struct {
		name   string
		invoke func(context.Context, Service, Fields) error
	}{
		{
			name: "append-transition",
			invoke: func(ctx context.Context, service Service, fields Fields) error {
				return dao.BusinessAuditEvent.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
					_, appendErr := service.AppendTransition(ctx, TransitionInput{
						Fields:         fields,
						BeforeSnapshot: map[string]any{"state": "stopped"},
						AfterSnapshot:  map[string]any{"state": "running"},
					})
					return appendErr
				})
			},
		},
		{
			name: "record-observation",
			invoke: func(ctx context.Context, service Service, fields Fields) error {
				_, observationErr := service.RecordObservation(ctx, ObservationInput{
					Fields:   fields,
					Snapshot: map[string]any{"decision": "observed"},
				})
				return observationErr
			},
		},
	}
	for _, entry := range entries {
		t.Run(entry.name, func(t *testing.T) {
			fields := baseFields
			fields.IdempotencyKey = entry.name
			if err := entry.invoke(customerCtx, service, fields); !bizerr.Is(err, CodeIdentityDenied) {
				t.Fatalf("credential-bearing customer backfill returned %v", err)
			}
		})
	}
	assertAuditIntegrationCounts(t, ctx, 0, 0)
}

// TestAuditPostgresConcurrentReplayConverges verifies business-number
// idempotency serializes concurrent writers on one immutable audit event.
func TestAuditPostgresConcurrentReplayConverges(t *testing.T) {
	ctx := context.Background()
	configureAuditIntegrationDB(t, ctx)
	service, err := newService(
		contextActorResolver{},
		staticAuthorizationService{},
		testAuditConfig(),
		time.Now,
		rand.Reader,
	)
	if err != nil {
		t.Fatalf("create concurrent audit service: %v", err)
	}
	customerCtx := withAuditIntegrationActor(ctx, TrustedActor{
		Type:         ActorUser,
		UserID:       701,
		AudienceCode: principal.AudienceCustomer,
		CustomerNo:   "CUS_CONCURRENT",
	})
	input := auditIntegrationTransition("concurrent", "same")
	input.CustomerNo = "CUS_CONCURRENT"
	const callers = 6
	start := make(chan struct{})
	results := make([]*Event, callers)
	errorsByCaller := make([]error, callers)
	var waitGroup sync.WaitGroup
	for index := range callers {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			errorsByCaller[index] = dao.BusinessAuditEvent.Transaction(
				customerCtx,
				func(ctx context.Context, _ gdb.TX) error {
					var appendErr error
					results[index], appendErr = service.AppendTransition(ctx, input)
					return appendErr
				},
			)
		}()
	}
	close(start)
	waitGroup.Wait()
	winner := ""
	for index := range callers {
		if errorsByCaller[index] != nil || results[index] == nil {
			t.Fatalf("concurrent append %d failed: event=%+v err=%v", index, results[index], errorsByCaller[index])
		}
		if winner == "" {
			winner = results[index].AuditNo
		}
		if results[index].AuditNo != winner {
			t.Fatalf("concurrent append diverged: got=%q want=%q", results[index].AuditNo, winner)
		}
	}
	assertAuditIntegrationCounts(t, customerCtx, 1, 1)
}

// TestAuditPostgresSameKeyAcrossCustomersDoesNotCollide proves the trusted
// actor/customer reservation scope prevents cross-customer idempotency capture.
func TestAuditPostgresSameKeyAcrossCustomersDoesNotCollide(t *testing.T) {
	ctx := context.Background()
	configureAuditIntegrationDB(t, ctx)
	service, err := newService(
		contextActorResolver{},
		staticAuthorizationService{},
		testAuditConfig(),
		time.Now,
		rand.Reader,
	)
	if err != nil {
		t.Fatalf("create customer-scope audit service: %v", err)
	}
	firstCtx := withAuditIntegrationActor(ctx, TrustedActor{
		Type:         ActorUser,
		UserID:       801,
		AudienceCode: principal.AudienceCustomer,
		CustomerNo:   "CUS_SCOPE_ONE",
	})
	secondCtx := withAuditIntegrationActor(ctx, TrustedActor{
		Type:         ActorUser,
		UserID:       802,
		AudienceCode: principal.AudienceCustomer,
		CustomerNo:   "CUS_SCOPE_TWO",
	})
	firstInput := auditIntegrationTransition("shared-scope", "shared-key")
	firstInput.CustomerNo = "CUS_SCOPE_ONE"
	secondInput := auditIntegrationTransition("shared-scope", "shared-key")
	secondInput.CustomerNo = "CUS_SCOPE_TWO"
	first := appendAuditIntegrationTransition(t, firstCtx, service, firstInput)
	second := appendAuditIntegrationTransition(t, secondCtx, service, secondInput)
	if first.AuditNo == second.AuditNo {
		t.Fatalf("different customers captured the same audit number: %q", first.AuditNo)
	}
	assertAuditIntegrationCounts(t, ctx, 2, 2)
}

// TestAuditPostgresRejectsTamperedPersistedRows verifies valid-JSON, scalar,
// business-number, and owner mutations all fail closed on search and replay.
func TestAuditPostgresRejectsTamperedPersistedRows(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(context.Context, *entity.BusinessAuditEvent, *entity.BusinessNumber) error
	}{
		{
			name: "valid json snapshot",
			mutate: func(ctx context.Context, row *entity.BusinessAuditEvent, _ *entity.BusinessNumber) error {
				_, err := dao.BusinessAuditEvent.Ctx(ctx).
					Where(do.BusinessAuditEvent{TenantId: idccap.DefaultTenantID, AuditNo: row.AuditNo}).
					Data(do.BusinessAuditEvent{BeforeSnapshot: `{"schema_version":1,"data":{"state":"tampered"}}`}).
					Update()
				return err
			},
		},
		{
			name: "oversized valid json snapshot",
			mutate: func(ctx context.Context, row *entity.BusinessAuditEvent, _ *entity.BusinessNumber) error {
				encoded, err := json.Marshal(stateEnvelope{
					SchemaVersion: snapshotSchemaVersion,
					Data: map[string]any{
						"payload": strings.Repeat("x", defaultMaximumSnapshotBytes),
					},
				})
				if err != nil {
					return err
				}
				_, err = dao.BusinessAuditEvent.Ctx(ctx).
					Where(do.BusinessAuditEvent{TenantId: idccap.DefaultTenantID, AuditNo: row.AuditNo}).
					Data(do.BusinessAuditEvent{BeforeSnapshot: string(encoded)}).
					Update()
				return err
			},
		},
		{
			name: "business scalar",
			mutate: func(ctx context.Context, row *entity.BusinessAuditEvent, _ *entity.BusinessNumber) error {
				_, err := dao.BusinessAuditEvent.Ctx(ctx).
					Where(do.BusinessAuditEvent{TenantId: idccap.DefaultTenantID, AuditNo: row.AuditNo}).
					Data(do.BusinessAuditEvent{TargetNo: "SRV_TAMPERED"}).
					Update()
				return err
			},
		},
		{
			name: "audit business number",
			mutate: func(ctx context.Context, row *entity.BusinessAuditEvent, _ *entity.BusinessNumber) error {
				_, err := dao.BusinessAuditEvent.Ctx(ctx).
					Where(do.BusinessAuditEvent{TenantId: idccap.DefaultTenantID, AuditNo: row.AuditNo}).
					Data(do.BusinessAuditEvent{AuditNo: "AUD_ffffffffffffffffffffffffffffffff"}).
					Update()
				return err
			},
		},
		{
			name: "owner binding",
			mutate: func(ctx context.Context, _ *entity.BusinessAuditEvent, reservation *entity.BusinessNumber) error {
				_, err := dao.BusinessNumber.Ctx(ctx).
					Where(do.BusinessNumber{TenantId: idccap.DefaultTenantID, BusinessNo: reservation.BusinessNo}).
					Data(do.BusinessNumber{OwnerInternalId: reservation.OwnerInternalId + 1}).
					Update()
				return err
			},
		},
		{
			name: "row and owner internal IDs together",
			mutate: func(
				ctx context.Context,
				row *entity.BusinessAuditEvent,
				reservation *entity.BusinessNumber,
			) error {
				tamperedID := row.Id + 1_000_000
				if _, err := g.DB().Exec(
					ctx,
					`ALTER TABLE plugin_idc_core_business_audit_event `+
						`ALTER COLUMN "id" SET GENERATED BY DEFAULT`,
				); err != nil {
					return err
				}
				return dao.BusinessAuditEvent.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
					if _, err := dao.BusinessAuditEvent.Ctx(ctx).
						Where(do.BusinessAuditEvent{Id: row.Id, TenantId: idccap.DefaultTenantID}).
						Data(do.BusinessAuditEvent{Id: tamperedID}).
						Update(); err != nil {
						return err
					}
					_, err := dao.BusinessNumber.Ctx(ctx).
						Where(do.BusinessNumber{
							TenantId:   idccap.DefaultTenantID,
							BusinessNo: reservation.BusinessNo,
						}).
						Data(do.BusinessNumber{OwnerInternalId: tamperedID}).
						Update()
					return err
				})
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			configureAuditIntegrationDB(t, ctx)
			service, err := newService(
				contextActorResolver{},
				staticAuthorizationService{grants: map[authz.PermissionKey]bool{
					authz.PermissionKey(permissionAuditRead): true,
				}},
				testAuditConfig(),
				time.Now,
				rand.Reader,
			)
			if err != nil {
				t.Fatalf("create tamper audit service: %v", err)
			}
			customerCtx := withAuditIntegrationActor(ctx, TrustedActor{
				Type:         ActorUser,
				UserID:       901,
				AudienceCode: principal.AudienceCustomer,
				CustomerNo:   "CUS_TAMPER",
			})
			input := auditIntegrationTransition("tamper", test.name)
			input.CustomerNo = "CUS_TAMPER"
			created := appendAuditIntegrationTransition(t, customerCtx, service, input)
			row := loadAuditIntegrationEvent(t, ctx, created.AuditNo)
			reservation := loadAuditIntegrationReservation(t, ctx, created.AuditNo)
			if err = test.mutate(ctx, row, reservation); err != nil {
				t.Fatalf("mutate persisted audit fixture: %v", err)
			}
			staffCtx := withAuditIntegrationActor(ctx, TrustedActor{
				Type:         ActorUser,
				UserID:       902,
				AudienceCode: principal.AudienceStaff,
				DataScope:    hostDataScopeTenant,
			})
			if _, err = service.Search(staffCtx, SearchInput{Limit: 10}); !bizerr.Is(err, CodeUnavailable) {
				t.Fatalf("tampered search returned %v", err)
			}
			err = dao.BusinessAuditEvent.Transaction(customerCtx, func(ctx context.Context, _ gdb.TX) error {
				_, replayErr := service.AppendTransition(ctx, input)
				return replayErr
			})
			if !bizerr.Is(err, CodeUnavailable) {
				t.Fatalf("tampered replay returned %v", err)
			}
		})
	}
}

// TestAuditPostgresRejectsCommittedUnsealedRow proves the deferred database
// guard rolls back an unsigned insert even when application sealing is skipped.
func TestAuditPostgresRejectsCommittedUnsealedRow(t *testing.T) {
	ctx := context.Background()
	configureAuditIntegrationDB(t, ctx)
	_, directErr := dao.BusinessAuditEvent.Ctx(ctx).Data(do.BusinessAuditEvent{
		TenantId:          idccap.DefaultTenantID,
		AuditNo:           "AUD_dddddddddddddddddddddddddddddddd",
		ActorType:         string(ActorSystem),
		ActorAudienceCode: "",
		OperationCode:     "audit.unsealed.direct",
		TargetType:        "server",
		TargetNo:          "SRV_UNSEALED_DIRECT",
		ResultCode:        string(ResultSucceeded),
		RiskCode:          string(RiskLow),
		BeforeSnapshot:    `{}`,
		AfterSnapshot:     `{}`,
		ContextSnapshot:   auditUnsealedContext,
	}).Insert()
	directCount, directCountErr := dao.BusinessAuditEvent.Ctx(ctx).Count()
	if directCountErr != nil || directCount != 0 {
		t.Fatalf(
			"autocommit unsealed audit row remained readable: insertErr=%v count=%d countErr=%v",
			directErr,
			directCount,
			directCountErr,
		)
	}
	err := dao.BusinessAuditEvent.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		_, insertErr := dao.BusinessAuditEvent.Ctx(ctx).Data(do.BusinessAuditEvent{
			TenantId:          idccap.DefaultTenantID,
			AuditNo:           "AUD_eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
			ActorType:         string(ActorSystem),
			ActorAudienceCode: "",
			OperationCode:     "audit.unsealed.test",
			TargetType:        "server",
			TargetNo:          "SRV_UNSEALED",
			ResultCode:        string(ResultSucceeded),
			RiskCode:          string(RiskLow),
			BeforeSnapshot:    `{}`,
			AfterSnapshot:     `{}`,
			ContextSnapshot:   auditUnsealedContext,
		}).Insert()
		return insertErr
	})
	if err == nil {
		t.Fatal("unsealed audit row committed without the deferred integrity guard")
	}
	if count, countErr := dao.BusinessAuditEvent.Ctx(ctx).Count(); countErr != nil || count != 0 {
		t.Fatalf("unsealed audit row remained readable: count=%d err=%v", count, countErr)
	}
}

// TestAuditPostgresRetainedIntegrityKeyVerifiesOldRows proves row-integrity key
// rotation preserves old reads/replays only while the old key remains retained.
func TestAuditPostgresRetainedIntegrityKeyVerifiesOldRows(t *testing.T) {
	ctx := context.Background()
	configureAuditIntegrationDB(t, ctx)
	oldConfig := testAuditConfig()
	oldConfig.ActiveIntegrityKey = IntegrityKey{
		ID:     "test-old",
		Secret: []byte("old-integrity-key-0123456789abcdef"),
	}
	oldService, err := newService(
		contextActorResolver{},
		staticAuthorizationService{},
		oldConfig,
		time.Now,
		rand.Reader,
	)
	if err != nil {
		t.Fatalf("create old-key audit service: %v", err)
	}
	customerCtx := withAuditIntegrationActor(ctx, TrustedActor{
		Type:         ActorUser,
		UserID:       1001,
		AudienceCode: principal.AudienceCustomer,
		CustomerNo:   "CUS_ROTATION",
	})
	oldInput := auditIntegrationTransition("rotation", "old")
	oldInput.CustomerNo = "CUS_ROTATION"
	oldEvent := appendAuditIntegrationTransition(t, customerCtx, oldService, oldInput)

	rotatedConfig := testAuditConfig()
	rotatedConfig.RetainedIntegrityKeys = []IntegrityKey{oldConfig.ActiveIntegrityKey}
	rotatedService, err := newService(
		contextActorResolver{},
		staticAuthorizationService{grants: map[authz.PermissionKey]bool{
			authz.PermissionKey(permissionAuditRead): true,
		}},
		rotatedConfig,
		time.Now,
		rand.Reader,
	)
	if err != nil {
		t.Fatalf("create rotated audit service: %v", err)
	}
	staffCtx := withAuditIntegrationActor(ctx, TrustedActor{
		Type:         ActorUser,
		UserID:       1002,
		AudienceCode: principal.AudienceStaff,
		DataScope:    hostDataScopeTenant,
	})
	page, err := rotatedService.Search(staffCtx, SearchInput{CustomerNo: "CUS_ROTATION", Limit: 10})
	if err != nil || len(page.Events) != 1 || page.Events[0].AuditNo != oldEvent.AuditNo {
		t.Fatalf("retained-key audit search failed: page=%+v err=%v", page, err)
	}
	replayed := appendAuditIntegrationTransition(t, customerCtx, rotatedService, oldInput)
	if replayed.AuditNo != oldEvent.AuditNo {
		t.Fatalf("retained-key replay changed audit number: got=%q want=%q", replayed.AuditNo, oldEvent.AuditNo)
	}
	newInput := auditIntegrationTransition("rotation", "new")
	newInput.CustomerNo = "CUS_ROTATION"
	newEvent := appendAuditIntegrationTransition(t, customerCtx, rotatedService, newInput)
	newRow := loadAuditIntegrationEvent(t, ctx, newEvent.AuditNo)
	parsed, err := rotatedService.parseStoredContextEnvelope(newRow.ContextSnapshot)
	if err != nil || parsed.integrity.KeyID != rotatedConfig.ActiveIntegrityKey.ID {
		t.Fatalf("new row did not use active integrity key: parsed=%+v err=%v", parsed.integrity, err)
	}

	withoutOldKey, err := newService(
		contextActorResolver{},
		staticAuthorizationService{grants: map[authz.PermissionKey]bool{
			authz.PermissionKey(permissionAuditRead): true,
		}},
		testAuditConfig(),
		time.Now,
		rand.Reader,
	)
	if err != nil {
		t.Fatalf("create no-retained-key audit service: %v", err)
	}
	if _, err = withoutOldKey.Search(staffCtx, SearchInput{CustomerNo: "CUS_ROTATION", Limit: 10}); !bizerr.Is(err, CodeUnavailable) {
		t.Fatalf("missing retained key search returned %v", err)
	}
	err = dao.BusinessAuditEvent.Transaction(customerCtx, func(ctx context.Context, _ gdb.TX) error {
		_, replayErr := withoutOldKey.AppendTransition(ctx, oldInput)
		return replayErr
	})
	if !bizerr.Is(err, CodeUnavailable) {
		t.Fatalf("missing retained key replay returned %v", err)
	}
}

// TestAuditPostgresRetainedReplayKeyPreservesIdempotency proves replay-key
// rotation finds the old reservation, verifies old evidence with its stored key
// ID, and writes subsequent events only with the new active key.
func TestAuditPostgresRetainedReplayKeyPreservesIdempotency(t *testing.T) {
	ctx := context.Background()
	configureAuditIntegrationDB(t, ctx)
	oldConfig := testAuditConfig()
	oldConfig.ActiveReplayDigestKey = IntegrityKey{
		ID:     "test-replay-old",
		Secret: []byte("old-replay-digest-key-0123456789abcdef"),
	}
	oldService, err := newService(
		contextActorResolver{},
		staticAuthorizationService{},
		oldConfig,
		time.Now,
		rand.Reader,
	)
	if err != nil {
		t.Fatalf("create old replay-key audit service: %v", err)
	}
	customerCtx := withAuditIntegrationActor(ctx, TrustedActor{
		Type:         ActorUser,
		UserID:       1101,
		AudienceCode: principal.AudienceCustomer,
		CustomerNo:   "CUS_REPLAY_ROTATION",
	})
	oldInput := auditIntegrationTransition("replay-rotation", "same-request")
	oldInput.CustomerNo = "CUS_REPLAY_ROTATION"
	oldEvent := appendAuditIntegrationTransition(t, customerCtx, oldService, oldInput)

	rotatedConfig := testAuditConfig()
	rotatedConfig.RetainedReplayDigestKeys = []IntegrityKey{oldConfig.ActiveReplayDigestKey}
	rotatedService, err := newService(
		contextActorResolver{},
		staticAuthorizationService{grants: map[authz.PermissionKey]bool{
			authz.PermissionKey(permissionAuditRead): true,
		}},
		rotatedConfig,
		time.Now,
		rand.Reader,
	)
	if err != nil {
		t.Fatalf("create rotated replay-key audit service: %v", err)
	}
	replayed := appendAuditIntegrationTransition(t, customerCtx, rotatedService, oldInput)
	if replayed.AuditNo != oldEvent.AuditNo {
		t.Fatalf(
			"replay-key rotation created a second event: got=%q want=%q",
			replayed.AuditNo,
			oldEvent.AuditNo,
		)
	}
	assertAuditIntegrationCounts(t, ctx, 1, 1)
	oldRow := loadAuditIntegrationEvent(t, ctx, oldEvent.AuditNo)
	oldParsed, err := rotatedService.parseStoredContextEnvelope(oldRow.ContextSnapshot)
	if err != nil || oldParsed.integrity.ReplayKeyID != oldConfig.ActiveReplayDigestKey.ID {
		t.Fatalf("old row replay key ID was not retained: parsed=%+v err=%v", oldParsed.integrity, err)
	}
	staffCtx := withAuditIntegrationActor(ctx, TrustedActor{
		Type:         ActorUser,
		UserID:       1102,
		AudienceCode: principal.AudienceStaff,
		DataScope:    hostDataScopeTenant,
	})
	page, err := rotatedService.Search(staffCtx, SearchInput{
		CustomerNo: "CUS_REPLAY_ROTATION",
		Limit:      10,
	})
	if err != nil || len(page.Events) != 1 || page.Events[0].AuditNo != oldEvent.AuditNo {
		t.Fatalf("retained replay-key search failed: page=%+v err=%v", page, err)
	}

	newInput := auditIntegrationTransition("replay-rotation", "new-request")
	newInput.CustomerNo = "CUS_REPLAY_ROTATION"
	newEvent := appendAuditIntegrationTransition(t, customerCtx, rotatedService, newInput)
	newRow := loadAuditIntegrationEvent(t, ctx, newEvent.AuditNo)
	newParsed, err := rotatedService.parseStoredContextEnvelope(newRow.ContextSnapshot)
	if err != nil || newParsed.integrity.ReplayKeyID != rotatedConfig.ActiveReplayDigestKey.ID {
		t.Fatalf("new row did not use active replay key: parsed=%+v err=%v", newParsed.integrity, err)
	}
	assertAuditIntegrationCounts(t, ctx, 2, 2)
}

// TestAuditPostgresRetiredReplayKeyFailsClosedWithoutDuplicate proves the
// stable reservation locator still finds the original row after its replay key
// is no longer configured. Verification then fails closed without appending a
// second event or reserving a second number.
func TestAuditPostgresRetiredReplayKeyFailsClosedWithoutDuplicate(t *testing.T) {
	ctx := context.Background()
	configureAuditIntegrationDB(t, ctx)
	oldConfig := testAuditConfig()
	oldConfig.ActiveReplayDigestKey = IntegrityKey{
		ID:     "test-replay-retired",
		Secret: []byte("retired-replay-key-0123456789abcdef"),
	}
	oldService, err := newService(
		contextActorResolver{},
		staticAuthorizationService{},
		oldConfig,
		time.Now,
		rand.Reader,
	)
	if err != nil {
		t.Fatalf("create retired-key fixture service: %v", err)
	}
	customerCtx := withAuditIntegrationActor(ctx, TrustedActor{
		Type:         ActorUser,
		UserID:       1201,
		AudienceCode: principal.AudienceCustomer,
		CustomerNo:   "CUS_REPLAY_RETIRED",
	})
	input := auditIntegrationTransition("replay-retired", "same-request")
	input.CustomerNo = "CUS_REPLAY_RETIRED"
	appendAuditIntegrationTransition(t, customerCtx, oldService, input)

	withoutRetiredKey, err := newService(
		contextActorResolver{},
		staticAuthorizationService{},
		testAuditConfig(),
		time.Now,
		rand.Reader,
	)
	if err != nil {
		t.Fatalf("create service without retired replay key: %v", err)
	}
	err = dao.BusinessAuditEvent.Transaction(customerCtx, func(ctx context.Context, _ gdb.TX) error {
		_, replayErr := withoutRetiredKey.AppendTransition(ctx, input)
		return replayErr
	})
	if !bizerr.Is(err, CodeUnavailable) {
		t.Fatalf("retired replay key returned %v, want unavailable", err)
	}
	assertAuditIntegrationCounts(t, ctx, 1, 1)
}

// TestAuditPostgresFilterIndexesMatchKeysetOrder verifies every selective
// audit search index carries the deterministic ID tie-breaker.
func TestAuditPostgresFilterIndexesMatchKeysetOrder(t *testing.T) {
	ctx := context.Background()
	configureAuditIntegrationDB(t, ctx)
	tests := []struct {
		name    string
		columns string
	}{
		{"idx_plugin_idc_core_audit_actor", `(tenant_id, actor_user_id, occurred_at desc, id desc)`},
		{"idx_plugin_idc_core_audit_acting", `(tenant_id, acting_user_id, occurred_at desc, id desc)`},
		{"idx_plugin_idc_core_audit_correlation", `(tenant_id, correlation_no, occurred_at desc, id desc)`},
		{"idx_plugin_idc_core_audit_operation", `(tenant_id, operation_code, occurred_at desc, id desc)`},
		{"idx_plugin_idc_core_audit_result", `(tenant_id, result_code, occurred_at desc, id desc)`},
		{"idx_plugin_idc_core_audit_risk", `(tenant_id, risk_code, occurred_at desc, id desc)`},
	}
	for _, test := range tests {
		definition, err := g.DB().GetValue(ctx, `
SELECT pg_get_indexdef(indexrelid)
  FROM pg_index
 WHERE indexrelid = ?::regclass`, test.name)
		if err != nil {
			t.Fatalf("read %s definition: %v", test.name, err)
		}
		normalized := strings.ReplaceAll(
			strings.Join(strings.Fields(strings.ToLower(definition.String())), " "),
			`"`,
			"",
		)
		if !strings.Contains(normalized, test.columns) {
			t.Errorf("%s does not match keyset order: %s", test.name, definition.String())
		}
	}
}

// TestAuditPostgresTargetTimelineIndexMatchesKeysetOrder verifies both the
// installed index structure and the PostgreSQL plan for a hot target whose
// events all share one timestamp.
func TestAuditPostgresTargetTimelineIndexMatchesKeysetOrder(t *testing.T) {
	ctx := context.Background()
	configureAuditIntegrationDB(t, ctx)
	db := g.DB()
	definition, err := db.GetValue(ctx, `
SELECT pg_get_indexdef(indexrelid)
  FROM pg_index
 WHERE indexrelid = 'idx_plugin_idc_core_audit_target'::regclass`)
	if err != nil {
		t.Fatalf("read target timeline index definition: %v", err)
	}
	normalizedDefinition := strings.Join(strings.Fields(strings.ToLower(definition.String())), " ")
	wantColumns := `(tenant_id, target_type, target_no, occurred_at desc, id desc)`
	if !strings.Contains(strings.ReplaceAll(normalizedDefinition, `"`, ""), wantColumns) {
		t.Fatalf("target timeline index does not match keyset order: %s", definition.String())
	}
	sealedContext := `{"schema_version":1,"data":{},"security":{},"integrity":{"version":1,"key_id":"plan-integrity","replay_key_id":"plan-replay","raw_replay_hmac":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","row_hmac":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}}`
	if _, err = db.Exec(ctx, `
INSERT INTO plugin_idc_core_business_audit_event (
    tenant_id, audit_no, occurred_at, actor_type, operation_code,
    target_type, target_no, result_code, context_snapshot
)
SELECT 0,
       'AUD_PLAN_' || series::text,
       TIMESTAMPTZ '2026-08-07 00:00:00+00',
       'system',
       'server.plan',
       'server',
       'SRV_HOT_TARGET',
       'succeeded',
       ?::jsonb
  FROM generate_series(1, 100000) AS series`, sealedContext); err != nil {
		t.Fatalf("seed hot target audit timeline: %v", err)
	}
	if _, err = db.Exec(ctx, `ANALYZE plugin_idc_core_business_audit_event`); err != nil {
		t.Fatalf("analyze hot target audit timeline: %v", err)
	}
	plan, err := db.GetValue(ctx, `
EXPLAIN (ANALYZE, FORMAT JSON)
SELECT id, occurred_at
  FROM plugin_idc_core_business_audit_event
 WHERE tenant_id = 0
   AND target_type = 'server'
   AND target_no = 'SRV_HOT_TARGET'
 ORDER BY occurred_at DESC, id DESC
 LIMIT 20`)
	if err != nil {
		t.Fatalf("explain target keyset query: %v", err)
	}
	planText := plan.String()
	if !strings.Contains(planText, "idx_plugin_idc_core_audit_target") ||
		strings.Contains(planText, `"Node Type": "Sort"`) ||
		strings.Contains(planText, `"Node Type": "Incremental Sort"`) {
		t.Fatalf("target keyset query did not use the exact ordered index:\n%s", planText)
	}
}

// TestAuditPostgresSearchResponseBudgetHasNoGaps verifies a bounded projected
// search page stops at its byte budget without skipping or repeating rows.
func TestAuditPostgresSearchResponseBudgetHasNoGaps(t *testing.T) {
	ctx := context.Background()
	configureAuditIntegrationDB(t, ctx)
	config := testAuditConfig()
	config.MaximumSnapshotBytes = 12 * 1024
	config.MaximumPageSize = 10
	config.MaximumResponseBytes = config.MaximumSnapshotBytes*3 + minimumEventScalarAllowance
	service, err := newService(
		contextActorResolver{},
		staticAuthorizationService{grants: map[authz.PermissionKey]bool{
			authz.PermissionKey(permissionAuditRead): true,
		}},
		config,
		time.Now,
		rand.Reader,
	)
	if err != nil {
		t.Fatalf("create response-budget audit service: %v", err)
	}
	customerCtx := withAuditIntegrationActor(ctx, TrustedActor{
		Type:         ActorUser,
		UserID:       1101,
		AudienceCode: principal.AudienceCustomer,
		CustomerNo:   "CUS_BUDGET",
	})
	const eventCount = 7
	for index := range eventCount {
		input := auditIntegrationTransition("budget", fmt.Sprintf("event-%d", index))
		input.CustomerNo = "CUS_BUDGET"
		input.TargetNo = fmt.Sprintf("SRV_BUDGET_%d", index)
		input.BeforeSnapshot = map[string]any{"state": "stopped", "payload": strings.Repeat("a", 7000)}
		input.AfterSnapshot = map[string]any{"state": "running", "payload": strings.Repeat("b", 7000)}
		appendAuditIntegrationTransition(t, customerCtx, service, input)
	}
	staffCtx := withAuditIntegrationActor(ctx, TrustedActor{
		Type:         ActorUser,
		UserID:       1102,
		AudienceCode: principal.AudienceStaff,
		DataScope:    hostDataScopeTenant,
	})
	seen := make(map[string]struct{}, eventCount)
	var cursor *Cursor
	pages := 0
	for {
		page, searchErr := service.Search(staffCtx, SearchInput{
			CustomerNo: "CUS_BUDGET",
			Limit:      10,
			Cursor:     cursor,
		})
		if searchErr != nil || len(page.Events) == 0 {
			t.Fatalf("budgeted audit search failed: page=%+v err=%v", page, searchErr)
		}
		encoded, encodeErr := json.Marshal(page)
		if encodeErr != nil || len(encoded) > config.MaximumResponseBytes {
			t.Fatalf("budgeted page bytes=%d bound=%d err=%v", len(encoded), config.MaximumResponseBytes, encodeErr)
		}
		for _, event := range page.Events {
			if _, duplicate := seen[event.AuditNo]; duplicate {
				t.Fatalf("budget pagination repeated audit number %q", event.AuditNo)
			}
			seen[event.AuditNo] = struct{}{}
		}
		pages++
		if page.Next == nil {
			break
		}
		cursor = page.Next
		if pages > eventCount {
			t.Fatal("budget pagination cursor did not advance")
		}
	}
	if pages < 2 || len(seen) != eventCount {
		t.Fatalf("budget pagination pages=%d events=%d want=%d", pages, len(seen), eventCount)
	}
}

func withAuditIntegrationActor(ctx context.Context, actor TrustedActor) context.Context {
	return context.WithValue(ctx, auditActorContextKey{}, actor)
}

func auditIntegrationTransition(scope string, key string) TransitionInput {
	return TransitionInput{
		Fields: Fields{
			IdempotencyScope: scope,
			IdempotencyKey:   key,
			OperationCode:    "server.start",
			TargetType:       "server",
			TargetNo:         "SRV_INTEGRATION",
			CustomerNo:       "CUS_INTEGRATION",
			Result:           ResultSucceeded,
			Risk:             RiskMedium,
			CorrelationNo:    "COR_INTEGRATION",
			ContextSnapshot: map[string]any{
				"source": "postgres-integration",
			},
			Request: RequestEvidence{
				RemoteAddress: "10.0.0.2:443",
				ForwardedFor:  "203.0.113.20",
				UserAgent:     "IntegrationAgent/1.0",
				RequestID:     "REQ_INTEGRATION",
				TraceID:       "TRACE_INTEGRATION",
			},
		},
		BeforeSnapshot: map[string]any{
			"state":       "stopped",
			"credentials": map[string]any{"api_key": "first-secret"},
		},
		AfterSnapshot: map[string]any{
			"state":    "running",
			"capacity": json.Number("1e2"),
			"ratio":    json.Number("1.20e1"),
			"zero":     json.Number("-0e-2"),
		},
	}
}

func appendAuditIntegrationTransition(
	t *testing.T,
	ctx context.Context,
	service Service,
	input TransitionInput,
) *Event {
	t.Helper()
	var event *Event
	err := dao.BusinessAuditEvent.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var appendErr error
		event, appendErr = service.AppendTransition(ctx, input)
		return appendErr
	})
	if err != nil || event == nil {
		t.Fatalf("append audit integration transition failed: event=%+v err=%v", event, err)
	}
	return event
}

func loadAuditIntegrationEvent(t *testing.T, ctx context.Context, auditNo string) *entity.BusinessAuditEvent {
	t.Helper()
	var row *entity.BusinessAuditEvent
	if err := dao.BusinessAuditEvent.Ctx(ctx).Where(do.BusinessAuditEvent{
		TenantId: idccap.DefaultTenantID,
		AuditNo:  auditNo,
	}).Scan(&row); err != nil || row == nil {
		t.Fatalf("load audit integration row failed: row=%+v err=%v", row, err)
	}
	return row
}

func loadAuditIntegrationReservation(t *testing.T, ctx context.Context, auditNo string) *entity.BusinessNumber {
	t.Helper()
	var row *entity.BusinessNumber
	if err := dao.BusinessNumber.Ctx(ctx).Where(do.BusinessNumber{
		TenantId:   idccap.DefaultTenantID,
		BusinessNo: auditNo,
	}).Scan(&row); err != nil || row == nil {
		t.Fatalf("load audit integration reservation failed: row=%+v err=%v", row, err)
	}
	return row
}

func assertAuditIntegrationCounts(t *testing.T, ctx context.Context, events int, numbers int) {
	t.Helper()
	eventCount, err := dao.BusinessAuditEvent.Ctx(ctx).Count()
	if err != nil || eventCount != events {
		t.Fatalf("audit event count=%d want=%d err=%v", eventCount, events, err)
	}
	numberCount, err := dao.BusinessNumber.Ctx(ctx).Count()
	if err != nil || numberCount != numbers {
		t.Fatalf("audit business number count=%d want=%d err=%v", numberCount, numbers, err)
	}
}

// configureAuditIntegrationDB creates only the two audit-owned tables in a
// collision-resistant schema selected through an explicit opt-in DSN.
func configureAuditIntegrationDB(t *testing.T, ctx context.Context) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(auditIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL audit integration tests", auditIntegrationDSNEnv)
	}
	adminDB, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create audit integration admin database failed: %v", err)
	}
	schema := newAuditIntegrationSchema(t)
	if _, err = adminDB.Exec(ctx, "CREATE SCHEMA "+quoteAuditPostgreSQLIdentifier(schema)); err != nil {
		if closeErr := adminDB.Close(ctx); closeErr != nil {
			t.Errorf("close audit integration admin database after schema failure: %v", closeErr)
		}
		t.Fatalf("create isolated audit schema failed: %v", err)
	}
	originalConfig := gdb.GetAllConfig()
	var scopedDB gdb.DB
	t.Cleanup(func() {
		if scopedDB != nil {
			if closeErr := scopedDB.Close(ctx); closeErr != nil {
				t.Errorf("close scoped audit database failed: %v", closeErr)
			}
		}
		if restoreErr := gdb.SetConfig(originalConfig); restoreErr != nil {
			t.Errorf("restore audit database configuration failed: %v", restoreErr)
		}
		if !strings.HasPrefix(schema, "linapro_audit_test_") {
			t.Errorf("refusing to drop unexpected audit schema %q", schema)
		} else if _, dropErr := adminDB.Exec(
			ctx,
			"DROP SCHEMA "+quoteAuditPostgreSQLIdentifier(schema)+" CASCADE",
		); dropErr != nil {
			t.Errorf("drop isolated audit schema failed: %v", dropErr)
		}
		if closeErr := adminDB.Close(ctx); closeErr != nil {
			t.Errorf("close audit integration admin database failed: %v", closeErr)
		}
	})
	for _, statement := range auditIntegrationTableDDL(schema) {
		if _, err = adminDB.Exec(ctx, statement); err != nil {
			t.Fatalf("create isolated audit table failed: %v", err)
		}
	}
	scopedDSN := appendAuditDSNParameter(dsn, "search_path="+schema)
	if err = gdb.SetConfig(gdb.Config{
		gdb.DefaultGroupName: gdb.ConfigGroup{{Link: scopedDSN}},
	}); err != nil {
		t.Fatalf("configure scoped audit integration database failed: %v", err)
	}
	scopedDB = g.DB()
	currentSchema, err := scopedDB.GetValue(ctx, "SELECT current_schema()")
	if err != nil || currentSchema.String() != schema {
		t.Fatalf("scoped audit search path mismatch: got=%q want=%q err=%v", currentSchema.String(), schema, err)
	}
	migration, err := os.ReadFile(auditMigrationPath(t, "006-audit-replay-lookup-and-target-index.sql"))
	if err != nil {
		t.Fatalf("read audit replay/index migration: %v", err)
	}
	if _, err = scopedDB.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("execute audit replay/index migration: %v", err)
	}
	if _, err = scopedDB.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("repeat audit replay/index migration idempotently: %v", err)
	}
	keysetMigration, err := os.ReadFile(auditMigrationPath(t, "008-audit-keyset-filter-indexes.sql"))
	if err != nil {
		t.Fatalf("read audit keyset filter migration: %v", err)
	}
	if _, err = scopedDB.Exec(ctx, string(keysetMigration)); err != nil {
		t.Fatalf("execute audit keyset filter migration: %v", err)
	}
	if _, err = scopedDB.Exec(ctx, string(keysetMigration)); err != nil {
		t.Fatalf("repeat audit keyset filter migration idempotently: %v", err)
	}
	linkMigration, err := os.ReadFile(auditMigrationPath(t, "009-audit-customer-link.sql"))
	if err != nil {
		t.Fatalf("read audit customer link migration: %v", err)
	}
	if _, err = scopedDB.Exec(ctx, string(linkMigration)); err != nil {
		t.Fatalf("execute audit customer link migration: %v", err)
	}
	if _, err = scopedDB.Exec(ctx, string(linkMigration)); err != nil {
		t.Fatalf("repeat audit customer link migration idempotently: %v", err)
	}
}

func auditIntegrationTableDDL(schema string) []string {
	qualify := func(table string) string {
		return quoteAuditPostgreSQLIdentifier(schema) + "." + quoteAuditPostgreSQLIdentifier(table)
	}
	auditTable := qualify(dao.BusinessAuditEvent.Table())
	sealFunction := qualify("plugin_idc_core_require_sealed_audit_event")
	return []string{
		fmt.Sprintf(`CREATE TABLE %s (
            "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
            "tenant_id" INT NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
            "customer_no" VARCHAR(40) NOT NULL UNIQUE,
            "customer_type_code" VARCHAR(16) NOT NULL,
            "status_code" VARCHAR(32) NOT NULL,
            "display_name" VARCHAR(256) NOT NULL,
            "country_code" VARCHAR(2) NOT NULL DEFAULT '',
            "preferred_locale" VARCHAR(32) NOT NULL DEFAULT 'zh-CN',
            "version" BIGINT NOT NULL DEFAULT 1,
            "anonymized_at" TIMESTAMPTZ,
            "created_by" INT NOT NULL DEFAULT 0,
            "updated_by" INT NOT NULL DEFAULT 0,
            "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
            "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
            "search_name" VARCHAR(512) NOT NULL DEFAULT '',
            "status_reason_code" VARCHAR(64) NOT NULL DEFAULT '',
            "status_reason" VARCHAR(512) NOT NULL DEFAULT '',
            "status_changed_by" INT NOT NULL DEFAULT 0,
            "status_changed_at" TIMESTAMPTZ
        )`, qualify(dao.CustomerSubject.Table())),
		fmt.Sprintf(`CREATE TABLE %s (
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
        )`, qualify(dao.BusinessNumber.Table())),
		fmt.Sprintf(`CREATE TABLE %s (
            "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
            "tenant_id" INT NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
            "audit_no" VARCHAR(40) NOT NULL UNIQUE,
            "occurred_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
            "actor_type" VARCHAR(32) NOT NULL CHECK ("actor_type" IN ('user','system','external')),
            "actor_user_id" INT,
            "acting_user_id" INT,
            "actor_audience_code" VARCHAR(16) NOT NULL DEFAULT '' CHECK ("actor_audience_code" IN ('','customer','staff')),
            "actor_customer_no" VARCHAR(40) NOT NULL DEFAULT '',
            "customer_no" VARCHAR(40) NOT NULL DEFAULT '',
            "operation_code" VARCHAR(128) NOT NULL,
            "target_type" VARCHAR(64) NOT NULL,
            "target_no" VARCHAR(64) NOT NULL,
            "result_code" VARCHAR(24) NOT NULL CHECK ("result_code" IN ('succeeded','denied','failed','result_unknown')),
            "reason_code" VARCHAR(128) NOT NULL DEFAULT '',
            "risk_code" VARCHAR(16) NOT NULL DEFAULT 'low' CHECK ("risk_code" IN ('low','medium','high','critical')),
            "before_snapshot" JSONB NOT NULL DEFAULT '{}'::JSONB,
            "after_snapshot" JSONB NOT NULL DEFAULT '{}'::JSONB,
            "context_snapshot" JSONB NOT NULL DEFAULT '{}'::JSONB,
            "request_id" VARCHAR(128) NOT NULL DEFAULT '',
            "trace_id" VARCHAR(128) NOT NULL DEFAULT '',
            "correlation_no" VARCHAR(64) NOT NULL DEFAULT '',
            "ip_digest" CHAR(64) NOT NULL DEFAULT '',
            "user_agent_digest" CHAR(64) NOT NULL DEFAULT '',
            "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`, auditTable),
		fmt.Sprintf(`CREATE INDEX idx_plugin_idc_core_audit_target
		ON %s ("tenant_id", "target_type", "target_no", "occurred_at" DESC)`, auditTable),
		fmt.Sprintf(`CREATE FUNCTION %s()
        RETURNS TRIGGER
        LANGUAGE plpgsql
        AS $$
        DECLARE
            target_id BIGINT;
            stored_context JSONB;
        BEGIN
            IF TG_OP = 'DELETE' THEN
                target_id := OLD.id;
            ELSE
                target_id := NEW.id;
            END IF;
            SELECT event.context_snapshot
              INTO stored_context
              FROM %s AS event
             WHERE event.id = target_id;
            IF stored_context IS NULL
               OR stored_context = '{}'::JSONB
               OR COALESCE(stored_context #>> '{schema_version}', '') <> '1'
               OR COALESCE(stored_context #>> '{integrity,version}', '') <> '1'
               OR COALESCE(stored_context #>> '{integrity,key_id}', '') !~ '^[a-z0-9][a-z0-9._:-]{0,63}$'
               OR COALESCE(stored_context #>> '{integrity,replay_key_id}', '') !~ '^[a-z0-9][a-z0-9._:-]{0,63}$'
               OR COALESCE(stored_context #>> '{integrity,raw_replay_hmac}', '') !~ '^[0-9a-f]{64}$'
               OR COALESCE(stored_context #>> '{integrity,row_hmac}', '') !~ '^[0-9a-f]{64}$' THEN
                RAISE EXCEPTION 'audit event %% cannot be committed without a valid integrity seal', target_id
                    USING ERRCODE = '23514';
            END IF;
            RETURN NULL;
        END;
        $$;`, sealFunction, auditTable),
		fmt.Sprintf(`CREATE CONSTRAINT TRIGGER trg_plugin_idc_core_require_sealed_audit_event
        AFTER INSERT OR UPDATE OR DELETE
        ON %s
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW
        EXECUTE FUNCTION %s()`, auditTable, sealFunction),
	}
}

func auditMigrationPath(t *testing.T, name string) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve audit integration test path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "manifest", "sql", name))
}

func appendAuditDSNParameter(dsn string, parameter string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + parameter
}

func quoteAuditPostgreSQLIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func newAuditIntegrationSchema(t *testing.T) string {
	t.Helper()
	var content [8]byte
	if _, err := rand.Read(content[:]); err != nil {
		t.Fatalf("generate audit integration schema failed: %v", err)
	}
	return "linapro_audit_test_" + hex.EncodeToString(content[:])
}
