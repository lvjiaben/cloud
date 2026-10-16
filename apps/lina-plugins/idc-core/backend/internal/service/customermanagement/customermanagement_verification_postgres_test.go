// This file verifies staff identity commands against PostgreSQL and the real sealed audit owner.
package customermanagement

import (
	"bytes"
	"context"
	"github.com/gogf/gf/v2/frame/g"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/audit"
	"lina-plugin-idc-core/backend/internal/service/customer"
	"lina-plugin-idc-core/backend/internal/service/identityprojection"
	"strings"
	"testing"
	"time"
)

func TestCustomerManualVerificationPostgreSQL(t *testing.T) {
	ctx := context.Background()
	configureManagementIntegrationDB(t, ctx)
	current := managementTestBizCtx{current: bizctxcap.CurrentContext{
		UserID: 71, TenantID: 0, DataScope: 2,
	}}
	principalSvc := &managementTestPrincipal{audience: activeStaffAudience()}
	authorization := &managementTestAuthz{grants: map[authz.PermissionKey]bool{
		authz.PermissionKey(permissionCustomerUpdate): true,
		authz.PermissionKey("idc:admin:audit:list"):   true,
	}}
	auditService, err := audit.New(
		current,
		principalSvc,
		authorization,
		audit.Config{
			ReservationLookupKey:  audit.IntegrityKey{ID: "lookup.v1", Secret: bytes.Repeat([]byte{1}, 32)},
			ActiveReplayDigestKey: audit.IntegrityKey{ID: "replay.v1", Secret: bytes.Repeat([]byte{2}, 32)},
			ActiveIntegrityKey:    audit.IntegrityKey{ID: "integrity.v1", Secret: bytes.Repeat([]byte{3}, 32)},
			MaximumSnapshotBytes:  32 * 1024,
			MaximumSnapshotDepth:  8,
			MaximumSnapshotNodes:  512,
			MaximumPageSize:       100,
			MaximumResponseBytes:  512 * 1024,
		},
	)
	if err != nil {
		t.Fatalf("create batch tag audit service: %v", err)
	}
	auditWriter, err := audit.NewBusinessWriterAdapter(auditService)
	if err != nil {
		t.Fatalf("create batch tag audit adapter: %v", err)
	}
	service, err := NewMutation(current, principalSvc, authorization, auditWriter)
	if err != nil {
		t.Fatalf("create batch tag mutation service: %v", err)
	}

	commands := service.(VerificationService)
	for _, kind := range []customer.Type{customer.TypeIndividual, customer.TypeEnterprise} {
		number := "CUS_55555555555555555555555555555555"
		if kind == customer.TypeEnterprise {
			number = "CUS_66666666666666666666666666666666"
		}
		insertManagementCustomer(t, ctx, number, kind, customer.VerificationUnverified, "Manual identity", time.Now())
		input := VerificationInput{CustomerNo: number, Action: VerificationApprove, ExpectedVersion: 1, IdempotencyKey: "manual-approve-test-" + string(kind)}
		if err = commands.UpdateVerification(ctx, input); err != nil {
			t.Fatal(err)
		}
		if err = commands.UpdateVerification(ctx, input); err != nil {
			t.Fatalf("replay failed: %v", err)
		}
		changed := input
		changed.Action = VerificationClear
		if err = commands.UpdateVerification(ctx, changed); err == nil {
			t.Fatal("changed retry accepted")
		}
		var subject *entity.CustomerSubject
		if err = dao.CustomerSubject.Ctx(ctx).Where(do.CustomerSubject{CustomerNo: number}).Scan(&subject); err != nil {
			t.Fatal(err)
		}
		if subject.Version != 2 || subject.CustomerTypeCode != string(kind) {
			t.Fatalf("subject changed incorrectly: %+v", subject)
		}
		var personal *entity.CustomerIndividualProfile
		var enterprise *entity.CustomerEnterpriseProfile
		if kind == customer.TypeIndividual {
			if err = dao.CustomerIndividualProfile.Ctx(ctx).Where(do.CustomerIndividualProfile{CustomerNo: number}).Scan(&personal); err != nil {
				t.Fatal(err)
			}
			if !personal.ManualVerification || personal.IdentityStatusCode != "verified" || personal.IdentityVerifiedAt == nil || personal.IdentityCaseNo != "" || strings.TrimSpace(personal.IdentityLookupDigest) != "" || personal.LegalName != "" {
				t.Fatalf("approval fabricated evidence: %+v", personal)
			}
		} else {
			if err = dao.CustomerEnterpriseProfile.Ctx(ctx).Where(do.CustomerEnterpriseProfile{CustomerNo: number}).Scan(&enterprise); err != nil {
				t.Fatal(err)
			}
			if !enterprise.ManualVerification || enterprise.VerificationStatusCode != "verified" || enterprise.VerifiedAt == nil || enterprise.VerificationCaseNo != "" || enterprise.RegistrationIdentifier != "" {
				t.Fatalf("enterprise approval fabricated evidence: %+v", enterprise)
			}
		}
		stale := input
		stale.IdempotencyKey = "manual-stale-test-" + string(kind)
		if err = commands.UpdateVerification(ctx, stale); !bizerr.Is(err, customer.CodeVersionConflict) {
			t.Fatalf("stale version accepted: %v", err)
		}
		authorization.grants[authz.PermissionKey(permissionCustomerUpdate)] = false
		if err = commands.UpdateVerification(ctx, VerificationInput{CustomerNo: number, Action: VerificationClear, ExpectedVersion: 2, IdempotencyKey: "manual-denied-test-" + string(kind)}); err == nil {
			t.Fatal("missing permission accepted")
		}
		authorization.grants[authz.PermissionKey(permissionCustomerUpdate)] = true
		// Seed representative current evidence; clearing must remove actual fields, including SQL NULL timestamps.
		now := time.Now().UTC()
		if kind == customer.TypeIndividual {
			_, err = dao.CustomerIndividualProfile.Ctx(ctx).Where(do.CustomerIndividualProfile{CustomerNo: number}).Data(do.CustomerIndividualProfile{LegalName: "Test person", LegalNameMasked: "T***", IdentityIdentifierMasked: "110***********0019", IdentityLookupDigest: strings.Repeat("a", 64), IdentityCaseNo: "case_reset_old_001", IdentityExpiresAt: &now}).Update()
		} else {
			_, err = dao.CustomerEnterpriseProfile.Ctx(ctx).Where(do.CustomerEnterpriseProfile{CustomerNo: number}).Data(do.CustomerEnterpriseProfile{LegalName: "Test company", RegistrationIdentifier: "91110000MA0099999X", RegistrationIdentifierMasked: "9111**********999X", RegistrationLookupDigest: strings.Repeat("b", 64), VerificationCaseNo: "case_reset_old_002", ExpiresAt: &now}).Update()
		}
		if err != nil {
			t.Fatal(err)
		}
		clearInput := VerificationInput{CustomerNo: number, Action: VerificationClear, ExpectedVersion: 2, IdempotencyKey: "manual-clear-test-" + string(kind)}
		if err = commands.UpdateVerification(ctx, clearInput); err != nil {
			t.Fatal(err)
		}
		if err = commands.UpdateVerification(ctx, clearInput); err != nil {
			t.Fatal(err)
		}
		if kind == customer.TypeIndividual {
			if err = dao.CustomerIndividualProfile.Ctx(ctx).Where(do.CustomerIndividualProfile{CustomerNo: number}).Scan(&personal); err != nil {
				t.Fatal(err)
			}
			if personal.IdentityStatusCode != "unverified" || personal.LegalName != "" || personal.LegalNameMasked != "" || personal.IdentityIdentifierMasked != "" || strings.TrimSpace(personal.IdentityLookupDigest) != "" || personal.IdentityCaseNo != "" || personal.IdentityVerifiedAt != nil || personal.IdentityExpiresAt != nil {
				t.Fatalf("personal data survived reset: %+v", personal)
			}
			projection, createErr := identityprojection.New(g.DB(), principalSvc, bytes.Repeat([]byte{4}, 32))
			if createErr != nil {
				t.Fatal(createErr)
			}
			if _, err = projection.ProjectPersonal(ctx, identitycap.PersonalProjectionInput{CustomerNo: number, CaseNo: "case_reset_old_001", Status: identitycap.StatusVerified, LegalName: []byte("Test person"), MaskedName: "T***", IdentityNumber: []byte("110101199003070019"), MaskedIdentityNo: "110***********0019", VerifiedAt: &now}); err == nil {
				t.Fatal("old completion restored cleared identity")
			}
			if _, err = projection.ProjectPersonal(ctx, identitycap.PersonalProjectionInput{CustomerNo: number, CaseNo: "case_reset_new_001", Status: identitycap.StatusPending}); err != nil {
				t.Fatalf("new case blocked: %v", err)
			}
		} else {
			if err = dao.CustomerEnterpriseProfile.Ctx(ctx).Where(do.CustomerEnterpriseProfile{CustomerNo: number}).Scan(&enterprise); err != nil {
				t.Fatal(err)
			}
			if enterprise.VerificationStatusCode != "unverified" || enterprise.LegalName != "" || enterprise.RegistrationIdentifier != "" || enterprise.RegistrationIdentifierMasked != "" || strings.TrimSpace(enterprise.RegistrationLookupDigest) != "" || enterprise.VerificationCaseNo != "" || enterprise.VerifiedAt != nil || enterprise.ExpiresAt != nil {
				t.Fatalf("enterprise data survived reset: %+v", enterprise)
			}
		}
		count, countErr := dao.BusinessAuditEvent.Ctx(ctx).Where("target_no", number).Count()
		if countErr != nil || count != 2 {
			t.Fatalf("audit count=%d err=%v", count, countErr)
		}
	}
}
