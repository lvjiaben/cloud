// This file verifies principal transaction invariants against an explicitly selected PostgreSQL test database.

package principal

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

const principalIntegrationDSNEnv = "LINAPRO_IDC_CORE_TEST_DSN"

type principalIntegrationIntent struct {
	IntentNo string
	RawToken string
	Username string
}

// TestDAOPersistenceRedeemConvergesConcurrentRequests proves the real
// PostgreSQL transaction creates one customer and invokes one audited transition under replay.
func TestDAOPersistenceRedeemConvergesConcurrentRequests(t *testing.T) {
	ctx := context.Background()
	configurePrincipalIntegrationDB(t, ctx)

	userID := newIntegrationUserID(t)
	intent := insertVerifiedPrincipalIntegrationIntent(t, ctx, userID)
	t.Cleanup(func() {
		cleanupPrincipalIntegrationRows(t, ctx, userID, intent.IntentNo)
	})

	const callers = 6
	var (
		waitGroup sync.WaitGroup
		start     = make(chan struct{})
		results   = make([]*CustomerBinding, callers)
		errors    = make([]error, callers)
	)
	auditWriter := &principalTestAuditWriter{}
	persistence := newDAOPersistence(auditWriter)
	input := redeemInput{
		UserID: userID, HostRegistrationUserID: userID,
		DisplayName: intent.Username, UsernameDigest: digestValue(intent.Username),
		TokenDigest: digestValue(intent.RawToken), BrowserDigest: digestValue("browser:" + intent.IntentNo),
	}
	for index := range callers {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			results[index], errors[index] = persistence.redeem(ctx, input)
		}()
	}
	close(start)
	waitGroup.Wait()

	customerNo := ""
	created := 0
	for index := range callers {
		if errors[index] != nil {
			t.Fatalf("concurrent redemption %d failed: %v", index, errors[index])
		}
		if results[index] == nil || !strings.HasPrefix(results[index].CustomerNo, "CUS_") {
			t.Fatalf("concurrent redemption %d returned invalid result: %+v", index, results[index])
		}
		if customerNo == "" {
			customerNo = results[index].CustomerNo
		}
		if results[index].CustomerNo != customerNo {
			t.Fatalf("concurrent redemptions diverged: %q and %q", customerNo, results[index].CustomerNo)
		}
		if !results[index].Reused {
			created++
		}
	}
	if created != 1 {
		t.Fatalf("expected exactly one creating redemption, got %d", created)
	}

	assertPrincipalIntegrationCount(t, ctx, dao.PrincipalAudience.Table(), "sys_user_id", userID, 1)
	assertPrincipalIntegrationCount(t, ctx, dao.CustomerIdentityBinding.Table(), "sys_user_id", userID, 1)
	assertPrincipalIntegrationCount(t, ctx, dao.CustomerSubject.Table(), "customer_no", customerNo, 1)
	assertPrincipalIntegrationCount(t, ctx, dao.CustomerIndividualProfile.Table(), "customer_no", customerNo, 1)
	assertPrincipalIntegrationCount(t, ctx, dao.CustomerEnterpriseProfile.Table(), "customer_no", customerNo, 0)
	if auditWriter.count() != 1 {
		t.Fatalf("onboarding audit transitions=%d, want one", auditWriter.count())
	}
	assertPrincipalIntegrationCount(
		t,
		ctx,
		dao.BusinessNumber.Table(),
		"reservation_scope_digest",
		digestValue("customer-onboarding:"+intent.IntentNo),
		2,
	)

	var consumed *entity.CustomerOnboardingIntent
	if err := dao.CustomerOnboardingIntent.Ctx(ctx).
		Where(do.CustomerOnboardingIntent{TenantId: idccap.DefaultTenantID, IntentNo: intent.IntentNo}).
		Scan(&consumed); err != nil {
		t.Fatalf("load consumed integration intent failed: %v", err)
	}
	if consumed == nil ||
		onboardingStatus(consumed.StatusCode) != onboardingStatusConsumed ||
		consumed.ConsumedByUserId != userID ||
		consumed.ResultCustomerNo != customerNo {
		t.Fatalf("unexpected consumed intent state: %+v", consumed)
	}

	current, err := persistence.current(ctx, userID)
	if err != nil || current == nil || current.Code != AudienceCustomer || current.CustomerNo != customerNo {
		t.Fatalf("expected consistent current customer projection, current=%+v err=%v", current, err)
	}
	if _, err = persistence.ensureStaff(ctx, userID); !bizerr.Is(err, CodeAudienceConflict) {
		t.Fatalf("expected customer-to-staff implicit conversion rejection, got %v", err)
	}
}

func TestDAOPersistenceRedeemCreatesEnterpriseProfileShell(t *testing.T) {
	ctx := context.Background()
	configurePrincipalIntegrationDB(t, ctx)
	userID := newIntegrationUserID(t)
	intent := insertVerifiedPrincipalIntegrationIntentOfType(t, ctx, userID, CustomerTypeEnterprise)
	t.Cleanup(func() {
		cleanupPrincipalIntegrationRows(t, ctx, userID, intent.IntentNo)
	})
	persistence := newDAOPersistence(&principalTestAuditWriter{})
	result, err := persistence.redeem(ctx, redeemInput{
		UserID: userID, HostRegistrationUserID: userID, DisplayName: intent.Username,
		UsernameDigest: digestValue(intent.Username), TokenDigest: digestValue(intent.RawToken), BrowserDigest: digestValue("browser:" + intent.IntentNo),
	})
	if err != nil || result == nil || result.CustomerType != CustomerTypeEnterprise {
		t.Fatalf("enterprise redemption result=%+v err=%v", result, err)
	}
	assertPrincipalIntegrationCount(t, ctx, dao.CustomerEnterpriseProfile.Table(), "customer_no", result.CustomerNo, 1)
	assertPrincipalIntegrationCount(t, ctx, dao.CustomerIndividualProfile.Table(), "customer_no", result.CustomerNo, 0)
}

func TestDAOPersistenceRedeemAttachesCurrentHostIdentityPostgreSQL(t *testing.T) {
	ctx := context.Background()
	configurePrincipalIntegrationDB(t, ctx)
	userID := newIntegrationUserID(t)
	intent := insertVerifiedPrincipalIntegrationIntent(t, ctx, userID)
	t.Cleanup(func() { cleanupPrincipalIntegrationRows(t, ctx, userID, intent.IntentNo) })
	if _, err := dao.CustomerOnboardingIntent.Ctx(ctx).
		Where(do.CustomerOnboardingIntent{TenantId: idccap.DefaultTenantID, IntentNo: intent.IntentNo}).
		Data(map[string]any{"host_registration_user_id": nil}).Update(); err != nil {
		t.Fatal(err)
	}
	persistence := newDAOPersistence(&principalTestAuditWriter{})
	result, err := persistence.redeem(ctx, redeemInput{UserID: userID, HostRegistrationUserID: userID, DisplayName: intent.Username, UsernameDigest: digestValue(intent.Username), TokenDigest: digestValue(intent.RawToken), BrowserDigest: digestValue("browser:" + intent.IntentNo)})
	if err != nil || result == nil {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	var stored *entity.CustomerOnboardingIntent
	if err = dao.CustomerOnboardingIntent.Ctx(ctx).Where(do.CustomerOnboardingIntent{TenantId: idccap.DefaultTenantID, IntentNo: intent.IntentNo}).Scan(&stored); err != nil || stored == nil || stored.HostRegistrationUserId != userID {
		t.Fatalf("stored=%+v err=%v", stored, err)
	}
}

func TestDAOPersistenceRedeemRejectsBrowserTamperingPostgreSQL(t *testing.T) {
	ctx := context.Background()
	configurePrincipalIntegrationDB(t, ctx)
	userID := newIntegrationUserID(t)
	intent := insertVerifiedPrincipalIntegrationIntent(t, ctx, userID)
	t.Cleanup(func() { cleanupPrincipalIntegrationRows(t, ctx, userID, intent.IntentNo) })
	persistence := newDAOPersistence(&principalTestAuditWriter{})
	result, err := persistence.redeem(ctx, redeemInput{UserID: userID, HostRegistrationUserID: userID, DisplayName: intent.Username, UsernameDigest: digestValue(intent.Username), TokenDigest: digestValue(intent.RawToken), BrowserDigest: digestValue("tampered-browser")})
	if result != nil || !bizerr.Is(err, CodeOnboardingUnavailable) {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	assertPrincipalIntegrationCount(t, ctx, dao.PrincipalAudience.Table(), "sys_user_id", userID, 0)
}

func TestDAOPersistenceRedeemRollsBackWhenAuditAppendFails(t *testing.T) {
	ctx := context.Background()
	configurePrincipalIntegrationDB(t, ctx)
	userID := newIntegrationUserID(t)
	intent := insertVerifiedPrincipalIntegrationIntent(t, ctx, userID)
	t.Cleanup(func() {
		cleanupPrincipalIntegrationRows(t, ctx, userID, intent.IntentNo)
	})

	persistence := newDAOPersistence(&principalTestAuditWriter{err: errors.New("audit unavailable")})
	result, err := persistence.redeem(ctx, redeemInput{
		UserID: userID, HostRegistrationUserID: userID, DisplayName: intent.Username,
		UsernameDigest: digestValue(intent.Username), TokenDigest: digestValue(intent.RawToken), BrowserDigest: digestValue("browser:" + intent.IntentNo),
	})
	if err == nil || result != nil || !strings.Contains(err.Error(), "append onboarding audit transition") {
		t.Fatalf("audit failure result=%+v err=%v", result, err)
	}
	assertPrincipalIntegrationCount(t, ctx, dao.PrincipalAudience.Table(), "sys_user_id", userID, 0)
	assertPrincipalIntegrationCount(t, ctx, dao.CustomerIdentityBinding.Table(), "sys_user_id", userID, 0)
	assertPrincipalIntegrationCount(t, ctx, dao.CustomerSubject.Table(), "created_by", userID, 0)
	assertPrincipalIntegrationCount(t, ctx, dao.BusinessNumber.Table(), "reservation_scope_digest",
		digestValue("customer-onboarding:"+intent.IntentNo), 0)

	var stored *entity.CustomerOnboardingIntent
	if err = dao.CustomerOnboardingIntent.Ctx(ctx).
		Where(do.CustomerOnboardingIntent{TenantId: idccap.DefaultTenantID, IntentNo: intent.IntentNo}).
		Scan(&stored); err != nil {
		t.Fatalf("load rolled back intent: %v", err)
	}
	if stored == nil || onboardingStatus(stored.StatusCode) != onboardingStatusVerified ||
		stored.ResultCustomerNo != "" || stored.ConsumedByUserId != 0 {
		t.Fatalf("audit failure left partial intent state: %+v", stored)
	}
}

// TestDAOPersistenceStaffBindingIsConcurrentAndIdempotent proves concurrent
// authorized calls converge on one active staff audience.
func TestDAOPersistenceStaffBindingIsConcurrentAndIdempotent(t *testing.T) {
	ctx := context.Background()
	configurePrincipalIntegrationDB(t, ctx)
	userID := newIntegrationUserID(t)
	t.Cleanup(func() {
		cleanupPrincipalIntegrationRows(t, ctx, userID, "")
	})

	const callers = 6
	var (
		waitGroup sync.WaitGroup
		start     = make(chan struct{})
		results   = make([]*Audience, callers)
		errors    = make([]error, callers)
	)
	persistence := newDAOPersistence()
	for index := range callers {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			results[index], errors[index] = persistence.ensureStaff(ctx, userID)
		}()
	}
	close(start)
	waitGroup.Wait()
	for index := range callers {
		if errors[index] != nil || results[index] == nil || results[index].Code != AudienceStaff {
			t.Fatalf("concurrent staff binding %d failed: result=%+v err=%v", index, results[index], errors[index])
		}
	}
	assertPrincipalIntegrationCount(t, ctx, dao.PrincipalAudience.Table(), "sys_user_id", userID, 1)
}

// TestDAOPersistenceCompetingIntentsForSameUserConverge proves two different
// verified intents cannot create two customer graphs for one host identity.
func TestDAOPersistenceCompetingIntentsForSameUserConverge(t *testing.T) {
	ctx := context.Background()
	configurePrincipalIntegrationDB(t, ctx)
	userID := newIntegrationUserID(t)
	firstIntent := insertVerifiedPrincipalIntegrationIntent(t, ctx, userID)
	t.Cleanup(func() {
		cleanupPrincipalIntegrationRows(t, ctx, userID, firstIntent.IntentNo)
	})
	secondIntent := insertVerifiedPrincipalIntegrationIntent(t, ctx, userID)
	t.Cleanup(func() {
		cleanupPrincipalIntegrationRows(t, ctx, userID, secondIntent.IntentNo)
	})
	intents := [2]principalIntegrationIntent{firstIntent, secondIntent}

	auditWriter := &principalTestAuditWriter{}
	persistence := newDAOPersistence(auditWriter)
	var (
		waitGroup sync.WaitGroup
		start     = make(chan struct{})
		results   [2]*CustomerBinding
		errors    [2]error
	)
	for index := range intents {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			intent := intents[index]
			results[index], errors[index] = persistence.redeem(ctx, redeemInput{
				UserID: userID, HostRegistrationUserID: userID,
				DisplayName: intent.Username, UsernameDigest: digestValue(intent.Username),
				TokenDigest: digestValue(intent.RawToken), BrowserDigest: digestValue("browser:" + intent.IntentNo),
			})
		}()
	}
	close(start)
	waitGroup.Wait()

	winners := 0
	conflicts := 0
	winnerCustomerNo := ""
	for index := range intents {
		switch {
		case errors[index] == nil:
			winners++
			if results[index] == nil || results[index].Reused {
				t.Fatalf("intent %d returned invalid creating result: %+v", index, results[index])
			}
			winnerCustomerNo = results[index].CustomerNo
		case bizerr.Is(errors[index], CodeAudienceConflict):
			conflicts++
		default:
			t.Fatalf("intent %d returned unexpected error: %v", index, errors[index])
		}
	}
	if winners != 1 || conflicts != 1 {
		t.Fatalf("expected one winner and one conflict, winners=%d conflicts=%d errors=%v", winners, conflicts, errors)
	}
	assertPrincipalIntegrationCount(t, ctx, dao.PrincipalAudience.Table(), "sys_user_id", userID, 1)
	assertPrincipalIntegrationCount(t, ctx, dao.CustomerIdentityBinding.Table(), "sys_user_id", userID, 1)
	assertPrincipalIntegrationCount(t, ctx, dao.CustomerSubject.Table(), "customer_no", winnerCustomerNo, 1)
	assertPrincipalIntegrationCount(t, ctx, dao.CustomerSubject.Table(), "created_by", userID, 1)

	consumed := 0
	verified := 0
	reservedCounts := 0
	for _, intent := range intents {
		var row *entity.CustomerOnboardingIntent
		if err := dao.CustomerOnboardingIntent.Ctx(ctx).
			Where(do.CustomerOnboardingIntent{TenantId: idccap.DefaultTenantID, IntentNo: intent.IntentNo}).
			Scan(&row); err != nil {
			t.Fatalf("load competing intent failed: %v", err)
		}
		if row == nil {
			t.Fatalf("competing intent %s disappeared", intent.IntentNo)
		}
		switch onboardingStatus(row.StatusCode) {
		case onboardingStatusConsumed:
			consumed++
			if row.ResultCustomerNo != winnerCustomerNo {
				t.Fatalf("consumed intent points to %q, want %q", row.ResultCustomerNo, winnerCustomerNo)
			}
		case onboardingStatusVerified:
			verified++
			if row.ResultCustomerNo != "" || row.ConsumedByUserId != 0 {
				t.Fatalf("losing intent was partially consumed: %+v", row)
			}
		default:
			t.Fatalf("unexpected competing intent state: %+v", row)
		}
		count, err := dao.BusinessNumber.Ctx(ctx).
			Where(do.BusinessNumber{
				TenantId:               idccap.DefaultTenantID,
				ReservationScopeDigest: digestValue("customer-onboarding:" + intent.IntentNo),
			}).Count()
		if err != nil {
			t.Fatalf("count competing intent number reservations failed: %v", err)
		}
		if count != 0 && count != 2 {
			t.Fatalf("intent %s left %d number reservations, want 0 or 2", intent.IntentNo, count)
		}
		reservedCounts += count
	}
	if consumed != 1 || verified != 1 || reservedCounts != 2 {
		t.Fatalf("unexpected competing intent convergence: consumed=%d verified=%d reservations=%d", consumed, verified, reservedCounts)
	}
	if auditWriter.count() != 1 {
		t.Fatalf("competing intents audit transitions=%d, want one", auditWriter.count())
	}
}

// TestDAOPersistenceStaffAndCustomerRaceConverges proves missing-row locks do
// not permit staff and customer audiences to commit for the same identity.
func TestDAOPersistenceStaffAndCustomerRaceConverges(t *testing.T) {
	ctx := context.Background()
	configurePrincipalIntegrationDB(t, ctx)
	userID := newIntegrationUserID(t)
	intent := insertVerifiedPrincipalIntegrationIntent(t, ctx, userID)
	t.Cleanup(func() {
		cleanupPrincipalIntegrationRows(t, ctx, userID, intent.IntentNo)
	})

	auditWriter := &principalTestAuditWriter{}
	persistence := newDAOPersistence(auditWriter)
	start := make(chan struct{})
	var (
		waitGroup      sync.WaitGroup
		staffResult    *Audience
		staffErr       error
		customerResult *CustomerBinding
		customerErr    error
	)
	waitGroup.Add(2)
	go func() {
		defer waitGroup.Done()
		<-start
		staffResult, staffErr = persistence.ensureStaff(ctx, userID)
	}()
	go func() {
		defer waitGroup.Done()
		<-start
		customerResult, customerErr = persistence.redeem(ctx, redeemInput{
			UserID: userID, HostRegistrationUserID: userID,
			DisplayName: intent.Username, UsernameDigest: digestValue(intent.Username),
			TokenDigest: digestValue(intent.RawToken), BrowserDigest: digestValue("browser:" + intent.IntentNo),
		})
	}()
	close(start)
	waitGroup.Wait()

	staffWon := staffErr == nil
	customerWon := customerErr == nil
	if staffWon == customerWon {
		t.Fatalf("expected exactly one audience winner, staff=%+v/%v customer=%+v/%v", staffResult, staffErr, customerResult, customerErr)
	}
	if !staffWon && !bizerr.Is(staffErr, CodeAudienceConflict) {
		t.Fatalf("staff loser returned unexpected error: %v", staffErr)
	}
	if !customerWon && !bizerr.Is(customerErr, CodeAudienceConflict) {
		t.Fatalf("customer loser returned unexpected error: %v", customerErr)
	}
	assertPrincipalIntegrationCount(t, ctx, dao.PrincipalAudience.Table(), "sys_user_id", userID, 1)
	if staffWon {
		if staffResult == nil || staffResult.Code != AudienceStaff {
			t.Fatalf("invalid staff winner: %+v", staffResult)
		}
		assertPrincipalIntegrationCount(t, ctx, dao.CustomerIdentityBinding.Table(), "sys_user_id", userID, 0)
		assertPrincipalIntegrationCount(t, ctx, dao.CustomerSubject.Table(), "created_by", userID, 0)
		assertPrincipalIntegrationCount(t, ctx, dao.BusinessNumber.Table(), "reservation_scope_digest", digestValue("customer-onboarding:"+intent.IntentNo), 0)
	} else {
		if customerResult == nil || customerResult.Reused {
			t.Fatalf("invalid customer winner: %+v", customerResult)
		}
		assertPrincipalIntegrationCount(t, ctx, dao.CustomerIdentityBinding.Table(), "sys_user_id", userID, 1)
		assertPrincipalIntegrationCount(t, ctx, dao.CustomerSubject.Table(), "customer_no", customerResult.CustomerNo, 1)
		assertPrincipalIntegrationCount(t, ctx, dao.BusinessNumber.Table(), "reservation_scope_digest", digestValue("customer-onboarding:"+intent.IntentNo), 2)
		if auditWriter.count() != 1 {
			t.Fatalf("customer winner audit transitions=%d, want one", auditWriter.count())
		}
	}

	var storedIntent *entity.CustomerOnboardingIntent
	if err := dao.CustomerOnboardingIntent.Ctx(ctx).
		Where(do.CustomerOnboardingIntent{TenantId: idccap.DefaultTenantID, IntentNo: intent.IntentNo}).
		Scan(&storedIntent); err != nil {
		t.Fatalf("load staff/customer race intent failed: %v", err)
	}
	wantStatus := onboardingStatusVerified
	if customerWon {
		wantStatus = onboardingStatusConsumed
	}
	if storedIntent == nil || onboardingStatus(storedIntent.StatusCode) != wantStatus {
		t.Fatalf("unexpected race intent state: %+v, want %s", storedIntent, wantStatus)
	}
}

// TestDAOPersistenceCurrentRejectsOrphanCustomerSubject proves a customer
// audience and identity binding are not sufficient without their subject row.
func TestDAOPersistenceCurrentRejectsOrphanCustomerSubject(t *testing.T) {
	ctx := context.Background()
	configurePrincipalIntegrationDB(t, ctx)
	userID := newIntegrationUserID(t)
	customerNo, err := newOpaqueBusinessNumber(rand.Reader, "TCUS_")
	if err != nil {
		t.Fatalf("generate orphan customer number failed: %v", err)
	}
	bindingNo, err := newOpaqueBusinessNumber(rand.Reader, "TBND_")
	if err != nil {
		t.Fatalf("generate orphan binding number failed: %v", err)
	}
	t.Cleanup(func() {
		cleanupPrincipalIntegrationRows(t, ctx, userID, "")
	})
	if _, err = dao.PrincipalAudience.Ctx(ctx).Data(do.PrincipalAudience{
		TenantId:     idccap.DefaultTenantID,
		SysUserId:    userID,
		AudienceCode: string(AudienceCustomer),
		StatusCode:   string(AudienceStatusActive),
	}).Insert(); err != nil {
		t.Fatalf("insert orphan audience failed: %v", err)
	}
	if _, err = dao.CustomerIdentityBinding.Ctx(ctx).Data(do.CustomerIdentityBinding{
		TenantId:   idccap.DefaultTenantID,
		BindingNo:  bindingNo,
		SysUserId:  userID,
		CustomerNo: customerNo,
		SourceCode: string(bindingSourceOnboarding),
		StatusCode: string(AudienceStatusActive),
	}).Insert(); err != nil {
		t.Fatalf("insert orphan identity binding failed: %v", err)
	}
	if current, err := newDAOPersistence().current(ctx, userID); !bizerr.Is(err, CodeAudienceInconsistent) || current != nil {
		t.Fatalf("expected orphan subject to fail closed, current=%+v err=%v", current, err)
	}
}

// configurePrincipalIntegrationDB opts into an explicitly selected database so
// normal unit tests never write to a developer or production database.
func configurePrincipalIntegrationDB(t *testing.T, ctx context.Context) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(principalIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL principal integration tests", principalIntegrationDSNEnv)
	}
	originalConfig := gdb.GetAllConfig()
	if err := gdb.SetConfig(gdb.Config{
		gdb.DefaultGroupName: gdb.ConfigGroup{{Link: dsn}},
	}); err != nil {
		t.Fatalf("configure principal integration database failed: %v", err)
	}
	t.Cleanup(func() {
		if err := gdb.SetConfig(originalConfig); err != nil {
			t.Errorf("restore principal integration database config failed: %v", err)
		}
	})
	db := g.DB()
	t.Cleanup(func() {
		if err := db.Close(ctx); err != nil {
			t.Errorf("close principal integration database failed: %v", err)
		}
	})
	if _, err := db.Exec(ctx, "SELECT 1"); err != nil {
		t.Fatalf("connect principal integration database failed: %v", err)
	}
}

// insertVerifiedPrincipalIntegrationIntent creates one isolated verified intent.
func insertVerifiedPrincipalIntegrationIntent(
	t *testing.T,
	ctx context.Context,
	userID int,
) principalIntegrationIntent {
	return insertVerifiedPrincipalIntegrationIntentOfType(t, ctx, userID, CustomerTypeIndividual)
}

func insertVerifiedPrincipalIntegrationIntentOfType(
	t *testing.T,
	ctx context.Context,
	userID int,
	customerType CustomerType,
) principalIntegrationIntent {
	t.Helper()
	intentNo, err := newOpaqueBusinessNumber(rand.Reader, "TINT_")
	if err != nil {
		t.Fatalf("generate integration intent number failed: %v", err)
	}
	rawToken, err := newOpaqueBusinessNumber(rand.Reader, "TTOK_")
	if err != nil {
		t.Fatalf("generate integration token failed: %v", err)
	}
	username := strings.ToLower(intentNo)
	verifiedAt := time.Now().UTC().Add(-time.Minute)
	expiresAt := verifiedAt.Add(10 * time.Minute)
	if _, err = dao.CustomerOnboardingIntent.Ctx(ctx).Data(do.CustomerOnboardingIntent{
		TenantId:               idccap.DefaultTenantID,
		IntentNo:               intentNo,
		TokenDigest:            digestValue(rawToken),
		BrowserNonceDigest:     digestValue("browser:" + intentNo),
		NormalizedUsernameHash: digestValue(username),
		ContactChannelCode:     "email",
		ContactHash:            digestValue("contact:" + intentNo),
		CustomerTypeCode:       string(customerType),
		StatusCode:             string(onboardingStatusVerified),
		ReturnPath:             "/account",
		HostRegistrationUserId: userID,
		VerifiedContactAt:      &verifiedAt,
		ExpiresAt:              &expiresAt,
		AttemptCount:           0,
		Version:                1,
	}).Insert(); err != nil {
		t.Fatalf("insert integration onboarding intent failed: %v", err)
	}
	return principalIntegrationIntent{IntentNo: intentNo, RawToken: rawToken, Username: username}
}

// newIntegrationUserID returns a high positive ID and refuses to reuse an
// identity already present in plugin-owned audience or binding tables.
func newIntegrationUserID(t *testing.T) int {
	t.Helper()
	for range 8 {
		var content [4]byte
		if _, err := rand.Read(content[:]); err != nil {
			t.Fatalf("generate integration user ID failed: %v", err)
		}
		candidate := 1_500_000_000 + int(binary.BigEndian.Uint32(content[:])%500_000_000)
		audienceCount, err := dao.PrincipalAudience.Ctx(context.Background()).
			Where(do.PrincipalAudience{TenantId: idccap.DefaultTenantID, SysUserId: candidate}).
			Count()
		if err != nil {
			t.Fatalf("check integration audience collision failed: %v", err)
		}
		bindingCount, err := dao.CustomerIdentityBinding.Ctx(context.Background()).
			Where(do.CustomerIdentityBinding{TenantId: idccap.DefaultTenantID, SysUserId: candidate}).
			Count()
		if err != nil {
			t.Fatalf("check integration binding collision failed: %v", err)
		}
		if audienceCount == 0 && bindingCount == 0 {
			return candidate
		}
	}
	t.Fatal("could not reserve an unused integration user ID")
	return 0
}

// assertPrincipalIntegrationCount verifies one table predicate without loading rows.
func assertPrincipalIntegrationCount(
	t *testing.T,
	ctx context.Context,
	table string,
	column string,
	value any,
	want int,
) {
	t.Helper()
	count, err := g.DB().Model(table).Ctx(ctx).Where(column, value).Count()
	if err != nil {
		t.Fatalf("count %s by %s failed: %v", table, column, err)
	}
	if count != want {
		t.Fatalf("count %s by %s=%v is %d, want %d", table, column, value, count, want)
	}
}

// cleanupPrincipalIntegrationRows removes only rows owned by the random test
// identity and optional intent number. No broad table cleanup is performed.
func cleanupPrincipalIntegrationRows(t *testing.T, ctx context.Context, userID int, intentNo string) {
	t.Helper()
	var customerNos []string
	if err := dao.CustomerIdentityBinding.Ctx(ctx).
		Fields(dao.CustomerIdentityBinding.Columns().CustomerNo).
		Where(do.CustomerIdentityBinding{TenantId: idccap.DefaultTenantID, SysUserId: userID}).
		Scan(&customerNos); err != nil {
		t.Errorf("load integration customer numbers for cleanup failed: %v", err)
	}
	if intentNo != "" {
		var intent *entity.CustomerOnboardingIntent
		if err := dao.CustomerOnboardingIntent.Ctx(ctx).
			Where(do.CustomerOnboardingIntent{TenantId: idccap.DefaultTenantID, IntentNo: intentNo}).
			Scan(&intent); err != nil {
			t.Errorf("load integration intent for cleanup failed: %v", err)
		}
		if intent != nil && intent.ResultCustomerNo != "" {
			customerNos = append(customerNos, intent.ResultCustomerNo)
		}
		if _, err := dao.BusinessAuditEvent.Ctx(ctx).
			Where(do.BusinessAuditEvent{TenantId: idccap.DefaultTenantID, CorrelationNo: intentNo}).
			Delete(); err != nil {
			t.Errorf("delete integration audit rows failed: %v", err)
		}
		if _, err := dao.BusinessNumber.Ctx(ctx).
			Where(do.BusinessNumber{
				TenantId:               idccap.DefaultTenantID,
				ReservationScopeDigest: digestValue("customer-onboarding:" + intentNo),
			}).Delete(); err != nil {
			t.Errorf("delete integration business numbers failed: %v", err)
		}
		if _, err := dao.CustomerOnboardingIntent.Ctx(ctx).
			Where(do.CustomerOnboardingIntent{TenantId: idccap.DefaultTenantID, IntentNo: intentNo}).
			Delete(); err != nil {
			t.Errorf("delete integration onboarding intent failed: %v", err)
		}
	}
	if _, err := dao.CustomerIdentityBinding.Ctx(ctx).
		Where(do.CustomerIdentityBinding{TenantId: idccap.DefaultTenantID, SysUserId: userID}).
		Delete(); err != nil {
		t.Errorf("delete integration identity binding failed: %v", err)
	}
	if _, err := dao.PrincipalAudience.Ctx(ctx).
		Where(do.PrincipalAudience{TenantId: idccap.DefaultTenantID, SysUserId: userID}).
		Delete(); err != nil {
		t.Errorf("delete integration audience failed: %v", err)
	}
	if len(customerNos) > 0 {
		if _, err := dao.CustomerIndividualProfile.Ctx(ctx).
			WhereIn(dao.CustomerIndividualProfile.Columns().CustomerNo, customerNos).
			Delete(); err != nil {
			t.Errorf("delete integration individual profiles failed: %v", err)
		}
		if _, err := dao.CustomerEnterpriseProfile.Ctx(ctx).
			WhereIn(dao.CustomerEnterpriseProfile.Columns().CustomerNo, customerNos).
			Delete(); err != nil {
			t.Errorf("delete integration enterprise profiles failed: %v", err)
		}
		if _, err := dao.CustomerSubject.Ctx(ctx).
			WhereIn(dao.CustomerSubject.Columns().CustomerNo, customerNos).
			Delete(); err != nil {
			t.Errorf("delete integration customer subjects failed: %v", err)
		}
	}
	verifyPrincipalIntegrationCleanup(t, ctx, userID, intentNo, customerNos)
}

// verifyPrincipalIntegrationCleanup proves test cleanup left no rows selected by
// its random identity, intent, or generated customer numbers.
func verifyPrincipalIntegrationCleanup(
	t *testing.T,
	ctx context.Context,
	userID int,
	intentNo string,
	customerNos []string,
) {
	t.Helper()
	type cleanupCheck struct {
		table  string
		column string
		value  any
	}
	checks := []cleanupCheck{
		{table: dao.PrincipalAudience.Table(), column: "sys_user_id", value: userID},
		{table: dao.CustomerIdentityBinding.Table(), column: "sys_user_id", value: userID},
	}
	if intentNo != "" {
		checks = append(checks,
			cleanupCheck{table: dao.CustomerOnboardingIntent.Table(), column: "intent_no", value: intentNo},
			cleanupCheck{table: dao.BusinessAuditEvent.Table(), column: "correlation_no", value: intentNo},
			cleanupCheck{
				table:  dao.BusinessNumber.Table(),
				column: "reservation_scope_digest",
				value:  digestValue("customer-onboarding:" + intentNo),
			},
		)
	}
	for _, check := range checks {
		count, err := g.DB().Model(check.table).Ctx(ctx).Where(check.column, check.value).Count()
		if err != nil {
			t.Errorf("verify cleanup for %s failed: %v", check.table, err)
			continue
		}
		if count != 0 {
			t.Errorf("cleanup left %d rows in %s for %s=%v", count, check.table, check.column, check.value)
		}
	}
	if len(customerNos) > 0 {
		count, err := dao.CustomerSubject.Ctx(ctx).
			WhereIn(dao.CustomerSubject.Columns().CustomerNo, customerNos).
			Count()
		if err != nil {
			t.Errorf("verify customer cleanup failed: %v", err)
		} else if count != 0 {
			t.Errorf("cleanup left %d customer subjects", count)
		}
	}
}
