// This file verifies encrypted enterprise submission against isolated PostgreSQL schemas.

package enterpriseverification

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/storagecap"
	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-compliance/backend/internal/model/do"
	"lina-plugin-idc-compliance/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

const enterpriseVerificationIntegrationDSNEnv = "LINAPRO_IDC_COMPLIANCE_TEST_DSN"

type enterpriseIdentityFake struct {
	subject   identitycap.EnterpriseSubject
	projected []identitycap.EnterpriseProjectionInput
	duplicate bool
}

func (*enterpriseIdentityFake) CurrentPersonal(context.Context) (*identitycap.PersonalSubject, error) {
	return nil, errors.New("not used")
}

func (*enterpriseIdentityFake) RecordPersonalConsent(
	context.Context,
	identitycap.PersonalConsentInput,
) (*identitycap.PersonalConsent, error) {
	return nil, errors.New("not used")
}

func (*enterpriseIdentityFake) PreparePersonal(context.Context, string) (*identitycap.PersonalSubject, error) {
	return nil, errors.New("not used")
}

func (*enterpriseIdentityFake) ProjectPersonal(
	context.Context,
	identitycap.PersonalProjectionInput,
) (*identitycap.PersonalProjection, error) {
	return nil, errors.New("not used")
}

func (f *enterpriseIdentityFake) CurrentEnterprise(context.Context) (*identitycap.EnterpriseSubject, error) {
	copyValue := f.subject
	return &copyValue, nil
}

func (*enterpriseIdentityFake) RecordEnterpriseConsent(
	context.Context,
	identitycap.EnterpriseConsentInput,
) (*identitycap.EnterpriseConsent, error) {
	return nil, errors.New("not used")
}

func (f *enterpriseIdentityFake) PrepareEnterprise(
	_ context.Context,
	consentNo string,
	legalChangeRequestNo string,
) (*identitycap.EnterprisePreparation, error) {
	if consentNo != "CNS_0123456789abcdef0123456789abcdef" || legalChangeRequestNo != "" {
		return nil, errors.New("consent denied")
	}
	return &identitycap.EnterprisePreparation{Subject: f.subject}, nil
}

func (f *enterpriseIdentityFake) ProjectEnterprise(
	_ context.Context,
	input identitycap.EnterpriseProjectionInput,
) (*identitycap.EnterpriseProjection, error) {
	defer clearBytes(input.LegalName)
	defer clearBytes(input.RegistrationIdentifier)
	f.projected = append(f.projected, input)
	if input.Status == identitycap.StatusVerified && f.duplicate {
		return &identitycap.EnterpriseProjection{
			Status: identitycap.StatusPending, Version: 2, DuplicateRisk: true,
			PreservedCurrent: input.LegalChangeRequestNo != "",
		}, nil
	}
	if input.Status == identitycap.StatusRejected && input.LegalChangeRequestNo != "" {
		return &identitycap.EnterpriseProjection{
			Status: identitycap.StatusVerified, Version: 2, PreservedCurrent: true,
		}, nil
	}
	result := &identitycap.EnterpriseProjection{Status: input.Status, Version: 2}
	if input.Status == identitycap.StatusVerified {
		result.RegistrationLookupDigest = strings.Repeat("a", 64)
	}
	return result, nil
}

type enterpriseManagementAuthorizerFake struct {
	actor       int
	permissions map[string]bool
	requests    []managementcap.AuthorizationRequest
}

func (f *enterpriseManagementAuthorizerFake) Authorize(
	_ context.Context,
	request managementcap.AuthorizationRequest,
) (int, error) {
	f.requests = append(f.requests, request)
	if !f.permissions[request.Permission] {
		return 0, errors.New("permission denied")
	}
	return f.actor, nil
}

type enterpriseSensitiveAccessFake struct {
	issued   managementcap.SensitiveAccessBinding
	consumed map[string]bool
}

func (f *enterpriseSensitiveAccessFake) IssueSensitiveAccess(
	_ context.Context,
	binding managementcap.SensitiveAccessBinding,
) (*managementcap.SensitiveAccessContext, error) {
	f.issued = binding
	return &managementcap.SensitiveAccessContext{
		Token:     "signed-one-time-sensitive-access-context",
		ExpiresAt: time.Now().UTC().Add(time.Minute),
	}, nil
}

func (f *enterpriseSensitiveAccessFake) ConsumeSensitiveAccess(
	_ context.Context,
	token string,
	binding managementcap.SensitiveAccessBinding,
) (*managementcap.SensitiveAccessProof, error) {
	if token != "signed-one-time-sensitive-access-context" || binding != f.issued || f.consumed[token] {
		return nil, errors.New("proof rejected")
	}
	f.consumed[token] = true
	return &managementcap.SensitiveAccessProof{
		ActorUserID: 8101, ExpiresAt: time.Now().UTC().Add(time.Minute),
	}, nil
}

type enterpriseSecretsFake struct {
	secretcap.Manager
	handle secretcap.Handle
	value  []byte
}

func newEnterpriseSecretsFake(t *testing.T) *enterpriseSecretsFake {
	t.Helper()
	handle, err := secretcap.NewHandle("SEC_0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	key := bytes.Repeat([]byte{0x53}, 32)
	encoded := make([]byte, base64.RawURLEncoding.EncodedLen(len(key)))
	base64.RawURLEncoding.Encode(encoded, key)
	clearBytes(key)
	return &enterpriseSecretsFake{handle: handle, value: encoded}
}

func (f *enterpriseSecretsFake) CreateGenerated(
	context.Context,
	string,
	string,
) (secretcap.CreateResult, error) {
	return secretcap.CreateResult{Handle: f.handle}, nil
}

func (f *enterpriseSecretsFake) Access(
	_ context.Context,
	request secretcap.AccessRequest,
	consume secretcap.Consumer,
) error {
	if !request.Handle.Equal(f.handle) {
		return errors.New("secret unavailable")
	}
	copyValue := append([]byte(nil), f.value...)
	defer clearBytes(copyValue)
	return consume(copyValue)
}

type enterpriseStorageFake struct {
	storagecap.Service
	objects map[string][]byte
}

func newEnterpriseStorageFake() *enterpriseStorageFake {
	return &enterpriseStorageFake{objects: make(map[string][]byte)}
}

func (f *enterpriseStorageFake) Put(
	_ context.Context,
	input storagecap.PutInput,
) (*storagecap.PutOutput, error) {
	if input.Overwrite {
		return nil, errors.New("overwrite forbidden")
	}
	if _, exists := f.objects[input.Path]; exists {
		return nil, errors.New("object exists")
	}
	content, err := io.ReadAll(input.Body)
	if err != nil || int64(len(content)) != input.Size || input.ContentType != "application/octet-stream" {
		return nil, errors.New("invalid private object write")
	}
	f.objects[input.Path] = append([]byte(nil), content...)
	return &storagecap.PutOutput{Object: enterpriseStorageObject(input.Path, content)}, nil
}

func (f *enterpriseStorageFake) Get(
	_ context.Context,
	input storagecap.GetInput,
) (*storagecap.GetOutput, error) {
	content, found := f.objects[input.Path]
	if !found {
		return &storagecap.GetOutput{Found: false}, nil
	}
	copyValue := append([]byte(nil), content...)
	return &storagecap.GetOutput{
		Found: true, Object: enterpriseStorageObject(input.Path, copyValue),
		Body: io.NopCloser(bytes.NewReader(copyValue)),
	}, nil
}

func enterpriseStorageObject(path string, content []byte) *storagecap.Object {
	return &storagecap.Object{
		Path: path, Size: int64(len(content)), ContentType: "application/octet-stream",
		Visibility: storagecap.VisibilityPrivate,
	}
}

type enterpriseAuditFake struct {
	mu          sync.Mutex
	transitions []idccap.BusinessAuditTransition
}

func (f *enterpriseAuditFake) AppendBusinessTransition(
	ctx context.Context,
	input idccap.BusinessAuditTransition,
) error {
	if gdb.TXFromCtx(ctx, dao.VerificationCase.Group()) == nil {
		return errors.New("audit is outside authoritative transaction")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.transitions = append(f.transitions, input)
	return nil
}

func (*enterpriseAuditFake) AppendBusinessBatchTransition(
	context.Context,
	idccap.BusinessAuditBatchTransition,
) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, errors.New("not used")
}

func TestEnterpriseSubmissionPostgreSQLEncryptsAndReplays(t *testing.T) {
	ctx := context.Background()
	database := enterpriseVerificationTestDatabase(t, ctx)
	identity := &enterpriseIdentityFake{subject: identitycap.EnterpriseSubject{
		CustomerNo: "cust_enterprise_0001", PrincipalReference: "usr:7101", ActorUserID: 7101,
	}}
	storage := newEnterpriseStorageFake()
	secrets := newEnterpriseSecretsFake(t)
	audit := &enterpriseAuditFake{}
	service, err := New(database, identity, storage, secrets, secrets, audit)
	if err != nil {
		t.Fatal(err)
	}
	draft, err := service.CreateDraft(ctx, CreateDraftInput{
		ConsentNo:      "CNS_0123456789abcdef0123456789abcdef",
		IdempotencyKey: "enterprise-draft-idempotency-0001",
	})
	if err != nil || draft == nil || draft.StatusCode != caseStatusDraft {
		t.Fatalf("draft=%+v err=%v", draft, err)
	}
	licenseReservation, err := service.ReserveMaterial(ctx, ReserveMaterialInput{
		CaseNo: draft.CaseNo, MaterialType: MaterialBusinessLicense,
		IdempotencyKey: "enterprise-material-license-0001",
	})
	if err != nil || licenseReservation == nil || licenseReservation.Reused {
		t.Fatalf("license reservation=%+v err=%v", licenseReservation, err)
	}
	representativeReservation, err := service.ReserveMaterial(ctx, ReserveMaterialInput{
		CaseNo: draft.CaseNo, MaterialType: MaterialLegalRepresentativeID,
		IdempotencyKey: "enterprise-material-representative-0001",
	})
	if err != nil || representativeReservation == nil || representativeReservation.Reused {
		t.Fatalf("representative reservation=%+v err=%v", representativeReservation, err)
	}
	replayedReservation, err := service.ReserveMaterial(ctx, ReserveMaterialInput{
		CaseNo: draft.CaseNo, MaterialType: MaterialBusinessLicense,
		IdempotencyKey: "enterprise-material-license-0001",
	})
	if err != nil || replayedReservation == nil || !replayedReservation.Reused ||
		replayedReservation.MaterialNo != licenseReservation.MaterialNo {
		t.Fatalf("reservation replay=%+v err=%v", replayedReservation, err)
	}
	pdf := []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\n%%EOF")
	license, err := service.UploadMaterial(ctx, UploadMaterialInput{
		CaseNo: draft.CaseNo, MaterialNo: licenseReservation.MaterialNo,
		DeclaredMediaType: "application/pdf", Body: bytes.NewReader(pdf),
	})
	if err != nil {
		t.Fatal(err)
	}
	representative, err := service.UploadMaterial(ctx, UploadMaterialInput{
		CaseNo: draft.CaseNo, MaterialNo: representativeReservation.MaterialNo,
		DeclaredMediaType: "application/pdf", Body: bytes.NewReader(pdf),
	})
	if err != nil {
		t.Fatal(err)
	}
	replayedMaterial, err := service.UploadMaterial(ctx, UploadMaterialInput{
		CaseNo: draft.CaseNo, MaterialNo: licenseReservation.MaterialNo,
		DeclaredMediaType: "application/pdf", Body: bytes.NewReader(pdf),
	})
	if err != nil || !replayedMaterial.Reused || replayedMaterial.MaterialNo != license.MaterialNo {
		t.Fatalf("material replay=%+v err=%v", replayedMaterial, err)
	}
	name := []byte("示例科技有限公司")
	registration := []byte("91350211M000100Y46")
	representativeName := []byte("测试法人")
	representativeIdentity := []byte("110101199001010015")
	result, err := service.Submit(ctx, SubmitInput{
		CaseNo: draft.CaseNo, LegalName: name, RegistrationCountryCode: "CN",
		RegistrationIdentifier:            registration,
		LegalRepresentativeName:           representativeName,
		LegalRepresentativeIdentityNumber: representativeIdentity,
		IndustryCode:                      "IDC",
		ApplicantRole:                     ApplicantLegalRepresentative,
		MaterialNos:                       []string{representative.MaterialNo, license.MaterialNo},
		IdempotencyKey:                    "enterprise-submission-idempotency-0001",
	})
	if err != nil || result == nil || result.StatusCode != caseStatusPending || result.Reused {
		t.Fatalf("submission=%+v err=%v", result, err)
	}
	current, err := service.Current(ctx)
	if err != nil || current == nil || current.CaseNo != draft.CaseNo ||
		current.StatusCode != caseStatusPending || current.SubmittedAt == nil || current.Version != 2 ||
		len(current.Materials) != 2 || current.Materials[0].MaterialNo == "" ||
		current.Materials[0].MediaType != "application/pdf" {
		t.Fatalf("current enterprise case=%+v err=%v", current, err)
	}
	if !allZero(name) || !allZero(registration) || !allZero(representativeName) ||
		!allZero(representativeIdentity) {
		t.Fatal("caller-owned legal buffers were not cleared")
	}
	if len(identity.projected) != 1 || len(identity.projected[0].LegalName) != 0 ||
		len(identity.projected[0].RegistrationIdentifier) != 0 {
		t.Fatalf("unsafe pending projection=%+v", identity.projected)
	}
	if len(storage.objects) != 3 {
		t.Fatalf("private object count=%d want=3", len(storage.objects))
	}
	for path, encrypted := range storage.objects {
		if !strings.HasPrefix(path, "enterprise/cases/"+draft.CaseNo+"/") ||
			bytes.Contains(encrypted, []byte("示例科技有限公司")) ||
			bytes.Contains(encrypted, []byte("91350211M000100Y46")) ||
			bytes.Contains(encrypted, []byte("测试法人")) ||
			bytes.Contains(encrypted, []byte("110101199001010015")) ||
			bytes.Contains(encrypted, pdf) {
			t.Fatalf("private object %q contains plaintext or escaped its case namespace", path)
		}
	}
	assertNoEnterprisePlaintext(
		t, ctx, database, "示例科技有限公司", "91350211M000100Y46",
		"测试法人", "110101199001010015", "enterprise-material-license-0001",
		"enterprise-material-representative-0001",
	)
	assertAuditIsSafe(
		t, audit.transitions, "示例科技有限公司", "91350211M000100Y46",
		"测试法人", "110101199001010015", "enterprise/cases/",
		"enterprise-material-license-0001", "enterprise-material-representative-0001",
	)
	name = []byte("示例科技有限公司")
	registration = []byte("91350211M000100Y46")
	representativeName = []byte("测试法人")
	representativeIdentity = []byte("110101199001010015")
	replay, err := service.Submit(ctx, SubmitInput{
		CaseNo: draft.CaseNo, LegalName: name, RegistrationCountryCode: "CN",
		RegistrationIdentifier:            registration,
		LegalRepresentativeName:           representativeName,
		LegalRepresentativeIdentityNumber: representativeIdentity,
		IndustryCode:                      "IDC",
		ApplicantRole:                     ApplicantLegalRepresentative,
		MaterialNos:                       []string{license.MaterialNo, representative.MaterialNo},
		IdempotencyKey:                    "enterprise-submission-idempotency-0001",
	})
	if err != nil || replay == nil || !replay.Reused || len(identity.projected) != 1 ||
		len(audit.transitions) != 6 {
		t.Fatalf("submission replay=%+v projections=%d audits=%d err=%v", replay, len(identity.projected), len(audit.transitions), err)
	}
}

func TestEnterpriseManagementPostgreSQLReviewAndOneTimeMaterialAccess(t *testing.T) {
	ctx := context.Background()
	database := enterpriseVerificationTestDatabase(t, ctx)
	identity := &enterpriseIdentityFake{subject: identitycap.EnterpriseSubject{
		CustomerNo:         "cust_enterprise_review_0001",
		PrincipalReference: "usr:8100", ActorUserID: 8100,
	}}
	storage := newEnterpriseStorageFake()
	secrets := newEnterpriseSecretsFake(t)
	audit := &enterpriseAuditFake{}
	caseNo, materialNos, original := prepareEnterpriseManagementCase(
		t, ctx, database, identity, storage, secrets, audit,
		"management-review-0001",
	)
	authorizer := &enterpriseManagementAuthorizerFake{
		actor: 8101,
		permissions: map[string]bool{
			permissionEnterpriseCaseList: true, permissionEnterpriseCaseQuery: true,
			permissionEnterpriseCaseReview: true, permissionEnterpriseCaseApprove: true,
			permissionEnterpriseCaseReject: true, permissionEnterpriseMaterialRead: true,
		},
	}
	sensitive := &enterpriseSensitiveAccessFake{consumed: make(map[string]bool)}
	management, err := NewManagement(
		database, identity, storage, secrets, secrets, managementcap.Runtime{
			Authorizer: authorizer, SensitiveAccess: sensitive, Audit: audit,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	page, err := management.ListCases(ctx, CaseListInput{PageSize: 1})
	if err != nil || page == nil || len(page.Items) != 1 || page.Items[0].CaseNo != caseNo ||
		page.Items[0].MaskedRegistrationID != "91************0Y46" || page.NextCursor != "" {
		t.Fatalf("management page=%+v err=%v", page, err)
	}
	detail, err := management.GetCase(ctx, caseNo)
	if err != nil || detail == nil || len(detail.Materials) != 2 ||
		detail.LegalProfile.RegistrationIdentifier != "91************0Y46" ||
		detail.LegalProfile.LegalRepresentativeID != "**************0015" ||
		strings.Contains(detail.LegalProfile.LegalName, "Example IDC") {
		t.Fatalf("masked detail=%+v err=%v", detail, err)
	}
	access, err := management.IssueMaterialAccess(ctx, caseNo, materialNos[0])
	if err != nil || access == nil || access.AccessContext == "" || access.ExpiresAt.IsZero() {
		t.Fatalf("access=%+v err=%v", access, err)
	}
	content, err := management.OpenMaterial(
		ctx, caseNo, materialNos[0], access.AccessContext,
	)
	if err != nil || content == nil || !bytes.Equal(content.Content, original) ||
		content.MediaType != "application/pdf" {
		t.Fatalf("content=%+v err=%v", content, err)
	}
	content.Clear()
	replayedContent, err := management.OpenMaterial(
		ctx, caseNo, materialNos[0], access.AccessContext,
	)
	if replayedContent != nil || err == nil {
		t.Fatalf("replayed content=%+v err=%v", replayedContent, err)
	}
	started, err := management.StartReview(ctx, ReviewInput{
		CaseNo: caseNo, ExpectedVersion: 2,
		IdempotencyKey: "management-review-start-0001",
	})
	if err != nil || started == nil || started.StatusCode != caseStatusVerifying || started.Version != 3 {
		t.Fatalf("started=%+v err=%v", started, err)
	}
	replayedStart, err := management.StartReview(ctx, ReviewInput{
		CaseNo: caseNo, ExpectedVersion: 2,
		IdempotencyKey: "management-review-start-0001",
	})
	if err != nil || replayedStart == nil || !replayedStart.Reused || replayedStart.Version != 3 {
		t.Fatalf("replayed start=%+v err=%v", replayedStart, err)
	}
	approved, err := management.Approve(ctx, DecisionInput{
		CaseNo: caseNo, ExpectedVersion: 3,
		IdempotencyKey: "management-approval-0001",
	})
	if err != nil || approved == nil || approved.StatusCode != caseStatusVerified ||
		approved.DecisionReasonCode != decisionReasonApproved || approved.Version != 4 {
		t.Fatalf("approved=%+v err=%v", approved, err)
	}
	replayedApproval, err := management.Approve(ctx, DecisionInput{
		CaseNo: caseNo, ExpectedVersion: 3,
		IdempotencyKey: "management-approval-0001",
	})
	if err != nil || replayedApproval == nil || !replayedApproval.Reused ||
		replayedApproval.StatusCode != caseStatusVerified {
		t.Fatalf("replayed approval=%+v err=%v", replayedApproval, err)
	}
	var row *entity.VerificationCase
	if err = dao.VerificationCase.Ctx(ctx).Where(do.VerificationCase{CaseNo: caseNo}).Scan(&row); err != nil || row == nil || row.StatusCode != caseStatusVerified ||
		row.DecisionActionCode != decisionActionApproved || row.IdentityLookupDigest == nil ||
		*row.IdentityLookupDigest != strings.Repeat("a", 64) {
		t.Fatalf("approved row=%+v err=%v", row, err)
	}
	assertManagementAuditSafe(t, audit.transitions)
}

func TestEnterpriseManagementPostgreSQLRejectsAndFlagsDuplicate(t *testing.T) {
	ctx := context.Background()
	database := enterpriseVerificationTestDatabase(t, ctx)
	storage := newEnterpriseStorageFake()
	secrets := newEnterpriseSecretsFake(t)
	audit := &enterpriseAuditFake{}
	authorizer := &enterpriseManagementAuthorizerFake{
		actor: 8101,
		permissions: map[string]bool{
			permissionEnterpriseCaseList:   true,
			permissionEnterpriseCaseReview: true, permissionEnterpriseCaseApprove: true,
			permissionEnterpriseCaseReject: true,
		},
	}
	sensitive := &enterpriseSensitiveAccessFake{consumed: make(map[string]bool)}

	rejectedIdentity := &enterpriseIdentityFake{subject: identitycap.EnterpriseSubject{
		CustomerNo:         "cust_enterprise_reject_0001",
		PrincipalReference: "usr:8200", ActorUserID: 8200,
	}}
	rejectedCase, _, _ := prepareEnterpriseManagementCase(
		t, ctx, database, rejectedIdentity, storage, secrets, audit, "management-reject-0001",
	)
	rejectedService, err := NewManagement(
		database, rejectedIdentity, storage, secrets, secrets, managementcap.Runtime{
			Authorizer: authorizer, SensitiveAccess: sensitive, Audit: audit,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	started, err := rejectedService.StartReview(ctx, ReviewInput{
		CaseNo: rejectedCase, ExpectedVersion: 2,
		IdempotencyKey: "management-reject-start-0001",
	})
	if err != nil {
		t.Fatal(err)
	}
	rejected, err := rejectedService.Reject(ctx, DecisionInput{
		CaseNo: rejectedCase, ExpectedVersion: started.Version,
		ReasonCode: "materials_incomplete", IdempotencyKey: "management-rejection-0001",
	})
	if err != nil || rejected == nil || rejected.StatusCode != caseStatusRejected ||
		rejected.DecisionReasonCode != "materials_incomplete" {
		t.Fatalf("rejected=%+v err=%v", rejected, err)
	}

	duplicateIdentity := &enterpriseIdentityFake{subject: identitycap.EnterpriseSubject{
		CustomerNo:         "cust_enterprise_duplicate_0001",
		PrincipalReference: "usr:8300", ActorUserID: 8300,
	}, duplicate: true}
	duplicateCase, _, _ := prepareEnterpriseManagementCase(
		t, ctx, database, duplicateIdentity, storage, secrets, audit, "management-duplicate-0001",
	)
	duplicateService, err := NewManagement(
		database, duplicateIdentity, storage, secrets, secrets, managementcap.Runtime{
			Authorizer: authorizer, SensitiveAccess: sensitive, Audit: audit,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	started, err = duplicateService.StartReview(ctx, ReviewInput{
		CaseNo: duplicateCase, ExpectedVersion: 2,
		IdempotencyKey: "management-duplicate-start-0001",
	})
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := duplicateService.Approve(ctx, DecisionInput{
		CaseNo: duplicateCase, ExpectedVersion: started.Version,
		IdempotencyKey: "management-duplicate-approve-0001",
	})
	if err != nil || duplicate == nil || duplicate.StatusCode != caseStatusReviewRequired ||
		duplicate.DecisionReasonCode != decisionReasonDuplicate {
		t.Fatalf("duplicate=%+v err=%v", duplicate, err)
	}
	firstPage, err := duplicateService.ListCases(ctx, CaseListInput{PageSize: 1})
	for _, pageNumber := range []int{1, 2, 3} {
		numbered, readErr := duplicateService.ListCases(ctx, CaseListInput{Page: pageNumber, PageSize: 1})
		wantRows := 0
		if pageNumber <= 2 {
			wantRows = 1
		}
		if readErr != nil || numbered.Total != 2 || len(numbered.Items) != wantRows {
			t.Fatalf("enterprise page %d: %+v err=%v", pageNumber, numbered, readErr)
		}
	}
	if err != nil || firstPage == nil || len(firstPage.Items) != 1 || firstPage.NextCursor == "" {
		t.Fatalf("first keyset page=%+v err=%v", firstPage, err)
	}
	secondPage, err := duplicateService.ListCases(ctx, CaseListInput{
		PageSize: 1, Cursor: firstPage.NextCursor,
	})
	if err != nil || secondPage == nil || len(secondPage.Items) != 1 ||
		secondPage.Items[0].CaseNo == firstPage.Items[0].CaseNo {
		t.Fatalf("second keyset page=%+v err=%v", secondPage, err)
	}
	tamperedSuffix := "A"
	if strings.HasSuffix(firstPage.NextCursor, tamperedSuffix) {
		tamperedSuffix = "B"
	}
	tampered := firstPage.NextCursor[:len(firstPage.NextCursor)-1] + tamperedSuffix
	if invalid, invalidErr := duplicateService.ListCases(ctx, CaseListInput{
		PageSize: 1, Cursor: tampered,
	}); invalid != nil || !bizerr.Is(invalidErr, CodeInvalid) {
		t.Fatalf("tampered cursor result=%+v err=%v", invalid, invalidErr)
	}
	if invalid, invalidErr := duplicateService.ListCases(ctx, CaseListInput{
		PageSize: 1, StatusCode: caseStatusVerified, Cursor: firstPage.NextCursor,
	}); invalid != nil || !bizerr.Is(invalidErr, CodeInvalid) {
		t.Fatalf("cross-filter cursor result=%+v err=%v", invalid, invalidErr)
	}
	restarted, err := duplicateService.StartReview(ctx, ReviewInput{
		CaseNo: duplicateCase, ExpectedVersion: duplicate.Version,
		IdempotencyKey: "management-duplicate-restart-0001",
	})
	if err != nil || restarted == nil || restarted.StatusCode != caseStatusVerifying {
		t.Fatalf("duplicate review restart=%+v err=%v", restarted, err)
	}
	var restartedRow *entity.VerificationCase
	if err = dao.VerificationCase.Ctx(ctx).Where(do.VerificationCase{
		CaseNo: duplicateCase,
	}).Scan(&restartedRow); err != nil || restartedRow == nil ||
		restartedRow.DecisionReasonCode != "" || restartedRow.DecisionActionCode != "" ||
		restartedRow.DecidedByPrincipalNo == nil || *restartedRow.DecidedByPrincipalNo != "" {
		t.Fatalf("restarted duplicate row=%+v err=%v", restartedRow, err)
	}
}

func prepareEnterpriseManagementCase(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	identity *enterpriseIdentityFake,
	storage *enterpriseStorageFake,
	secrets *enterpriseSecretsFake,
	audit *enterpriseAuditFake,
	suffix string,
) (string, []string, []byte) {
	t.Helper()
	service, err := New(database, identity, storage, secrets, secrets, audit)
	if err != nil {
		t.Fatal(err)
	}
	draft, err := service.CreateDraft(ctx, CreateDraftInput{
		ConsentNo:      "CNS_0123456789abcdef0123456789abcdef",
		IdempotencyKey: "draft-" + suffix,
	})
	if err != nil {
		t.Fatal(err)
	}
	reservations := make([]*MaterialReservation, 0, 2)
	for index, materialType := range []MaterialType{
		MaterialBusinessLicense, MaterialLegalRepresentativeID,
	} {
		reservation, reserveErr := service.ReserveMaterial(ctx, ReserveMaterialInput{
			CaseNo: draft.CaseNo, MaterialType: materialType,
			IdempotencyKey: "material-" + strconv.Itoa(index) + "-" + suffix,
		})
		if reserveErr != nil {
			t.Fatal(reserveErr)
		}
		reservations = append(reservations, reservation)
	}
	original := []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\n%%EOF")
	materialNos := make([]string, 0, len(reservations))
	for _, reservation := range reservations {
		uploaded, uploadErr := service.UploadMaterial(ctx, UploadMaterialInput{
			CaseNo: draft.CaseNo, MaterialNo: reservation.MaterialNo,
			DeclaredMediaType: "application/pdf", Body: bytes.NewReader(original),
		})
		if uploadErr != nil {
			t.Fatal(uploadErr)
		}
		materialNos = append(materialNos, uploaded.MaterialNo)
	}
	_, err = service.Submit(ctx, SubmitInput{
		CaseNo: draft.CaseNo, LegalName: []byte("Example IDC Limited"),
		RegistrationCountryCode:           "CN",
		RegistrationIdentifier:            []byte("91350211M000100Y46"),
		LegalRepresentativeName:           []byte("Review Person"),
		LegalRepresentativeIdentityNumber: []byte("110101199001010015"),
		IndustryCode:                      "IDC", ApplicantRole: ApplicantLegalRepresentative,
		MaterialNos: materialNos, IdempotencyKey: "submission-" + suffix,
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(materialNos)
	return draft.CaseNo, materialNos, original
}

func assertManagementAuditSafe(t *testing.T, transitions []idccap.BusinessAuditTransition) {
	t.Helper()
	encoded, err := json.Marshal(transitions)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, forbidden := range []string{
		"Example IDC Limited", "91350211M000100Y46", "110101199001010015",
		"signed-one-time-sensitive-access-context", "enterprise/cases/", "SEC_",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("management audit contains forbidden value %q", forbidden)
		}
	}
}

func TestEnterpriseMaterialReservationRejectsChangedSemanticReplay(t *testing.T) {
	ctx := context.Background()
	database := enterpriseVerificationTestDatabase(t, ctx)
	identity := &enterpriseIdentityFake{subject: identitycap.EnterpriseSubject{
		CustomerNo: "cust_enterprise_0002", PrincipalReference: "usr:7102", ActorUserID: 7102,
	}}
	storage := newEnterpriseStorageFake()
	secrets := newEnterpriseSecretsFake(t)
	audit := &enterpriseAuditFake{}
	service, err := New(database, identity, storage, secrets, secrets, audit)
	if err != nil {
		t.Fatal(err)
	}
	draft, err := service.CreateDraft(ctx, CreateDraftInput{
		ConsentNo:      "CNS_0123456789abcdef0123456789abcdef",
		IdempotencyKey: "enterprise-draft-idempotency-0002",
	})
	if err != nil {
		t.Fatal(err)
	}
	reservation, err := service.ReserveMaterial(ctx, ReserveMaterialInput{
		CaseNo: draft.CaseNo, MaterialType: MaterialBusinessLicense,
		IdempotencyKey: "enterprise-material-conflict-0001",
	})
	if err != nil || reservation == nil {
		t.Fatalf("reservation=%+v err=%v", reservation, err)
	}
	conflict, err := service.ReserveMaterial(ctx, ReserveMaterialInput{
		CaseNo: draft.CaseNo, MaterialType: MaterialLegalRepresentativeID,
		IdempotencyKey: "enterprise-material-conflict-0001",
	})
	if conflict != nil || !bizerr.Is(err, CodeConflict) || len(audit.transitions) != 2 {
		t.Fatalf("conflict=%+v audits=%d err=%v", conflict, len(audit.transitions), err)
	}
}

func TestEnterpriseMaterialReservationConvergesUnderConcurrency(t *testing.T) {
	ctx := context.Background()
	database := enterpriseVerificationTestDatabase(t, ctx)
	identity := &enterpriseIdentityFake{subject: identitycap.EnterpriseSubject{
		CustomerNo: "cust_enterprise_0004", PrincipalReference: "usr:7104", ActorUserID: 7104,
	}}
	storage := newEnterpriseStorageFake()
	secrets := newEnterpriseSecretsFake(t)
	audit := &enterpriseAuditFake{}
	service, err := New(database, identity, storage, secrets, secrets, audit)
	if err != nil {
		t.Fatal(err)
	}
	draft, err := service.CreateDraft(ctx, CreateDraftInput{
		ConsentNo:      "CNS_0123456789abcdef0123456789abcdef",
		IdempotencyKey: "enterprise-draft-idempotency-0004",
	})
	if err != nil {
		t.Fatal(err)
	}
	const workers = 8
	start := make(chan struct{})
	results := make(chan *MaterialReservation, workers)
	errors := make(chan error, workers)
	var wait sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			result, reserveErr := service.ReserveMaterial(ctx, ReserveMaterialInput{
				CaseNo: draft.CaseNo, MaterialType: MaterialBusinessLicense,
				IdempotencyKey: "enterprise-material-concurrent-0001",
			})
			results <- result
			errors <- reserveErr
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	close(errors)
	for reserveErr := range errors {
		if reserveErr != nil {
			t.Fatal(reserveErr)
		}
	}
	materialNo := ""
	for result := range results {
		if result == nil {
			t.Fatal("concurrent reservation returned nil")
		}
		if materialNo == "" {
			materialNo = result.MaterialNo
		}
		if result.MaterialNo != materialNo {
			t.Fatalf("concurrent reservation diverged: %q != %q", result.MaterialNo, materialNo)
		}
	}
	count, err := dao.EnterpriseMaterialUpload.Ctx(ctx).Where(do.EnterpriseMaterialUpload{
		TenantId: idccap.DefaultTenantID, CaseNo: draft.CaseNo,
	}).Count()
	if err != nil || count != 1 || len(audit.transitions) != 2 {
		t.Fatalf("reservations=%d audits=%d err=%v", count, len(audit.transitions), err)
	}
}

func TestEnterpriseMaterialUploadRequiresOwnedReservationAndStableContent(t *testing.T) {
	ctx := context.Background()
	database := enterpriseVerificationTestDatabase(t, ctx)
	identity := &enterpriseIdentityFake{subject: identitycap.EnterpriseSubject{
		CustomerNo: "cust_enterprise_0003", PrincipalReference: "usr:7103", ActorUserID: 7103,
	}}
	storage := newEnterpriseStorageFake()
	secrets := newEnterpriseSecretsFake(t)
	audit := &enterpriseAuditFake{}
	service, err := New(database, identity, storage, secrets, secrets, audit)
	if err != nil {
		t.Fatal(err)
	}
	draft, err := service.CreateDraft(ctx, CreateDraftInput{
		ConsentNo:      "CNS_0123456789abcdef0123456789abcdef",
		IdempotencyKey: "enterprise-draft-idempotency-0003",
	})
	if err != nil {
		t.Fatal(err)
	}
	missing, err := service.UploadMaterial(ctx, UploadMaterialInput{
		CaseNo: draft.CaseNo, MaterialNo: "mat_0123456789abcdef0123456789abcdef",
		DeclaredMediaType: "application/pdf", Body: bytes.NewReader([]byte("%PDF-1.4\n%%EOF")),
	})
	if missing != nil || !bizerr.Is(err, CodeNotAccessible) || len(storage.objects) != 0 {
		t.Fatalf("missing reservation=%+v objects=%d err=%v", missing, len(storage.objects), err)
	}
	reservation, err := service.ReserveMaterial(ctx, ReserveMaterialInput{
		CaseNo: draft.CaseNo, MaterialType: MaterialBusinessLicense,
		IdempotencyKey: "enterprise-material-stable-content-0001",
	})
	if err != nil {
		t.Fatal(err)
	}
	original := []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\n%%EOF")
	uploaded, err := service.UploadMaterial(ctx, UploadMaterialInput{
		CaseNo: draft.CaseNo, MaterialNo: reservation.MaterialNo,
		DeclaredMediaType: "application/pdf", Body: bytes.NewReader(original),
	})
	if err != nil || uploaded == nil || uploaded.Reused {
		t.Fatalf("uploaded=%+v err=%v", uploaded, err)
	}
	changed, err := service.UploadMaterial(ctx, UploadMaterialInput{
		CaseNo: draft.CaseNo, MaterialNo: reservation.MaterialNo,
		DeclaredMediaType: "application/pdf", Body: bytes.NewReader([]byte("%PDF-1.4\nchanged\n%%EOF")),
	})
	if changed != nil || !bizerr.Is(err, CodeConflict) || len(storage.objects) != 1 ||
		len(audit.transitions) != 3 {
		t.Fatalf("changed=%+v objects=%d audits=%d err=%v", changed, len(storage.objects), len(audit.transitions), err)
	}
}

func assertNoEnterprisePlaintext(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	values ...string,
) {
	t.Helper()
	for _, value := range values {
		for _, table := range []string{
			"plugin_idc_compliance_verification_case",
			"plugin_idc_compliance_enterprise_material_upload",
			"plugin_idc_compliance_enterprise_material",
		} {
			count, err := database.GetValue(ctx,
				"SELECT count(*) FROM "+table+" AS row_value WHERE row_to_json(row_value)::text LIKE ?",
				"%"+value+"%",
			)
			if err != nil || count.Int() != 0 {
				t.Fatalf("plaintext found in %s: count=%d err=%v", table, count.Int(), err)
			}
		}
	}
}

func assertAuditIsSafe(t *testing.T, values []idccap.BusinessAuditTransition, forbidden ...string) {
	t.Helper()
	if len(values) != 6 {
		t.Fatalf("audit count=%d want=6", len(values))
	}
	for _, transition := range values {
		before, err := json.Marshal(transition.BeforeSnapshot)
		if err != nil {
			t.Fatalf("encode audit before snapshot: %v", err)
		}
		after, err := json.Marshal(transition.AfterSnapshot)
		if err != nil {
			t.Fatalf("encode audit after snapshot: %v", err)
		}
		text := strings.Join([]string{
			transition.IdempotencyScope, transition.IdempotencyKey, transition.OperationCode,
			transition.TargetNo, transition.CorrelationNo, string(before), string(after),
		}, "\x00")
		for _, value := range forbidden {
			if strings.Contains(text, value) {
				t.Fatalf("audit routing metadata contains forbidden value %q", value)
			}
		}
	}
}

func allZero(value []byte) bool {
	for _, character := range value {
		if character != 0 {
			return false
		}
	}
	return true
}

func enterpriseVerificationTestDatabase(t *testing.T, ctx context.Context) gdb.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(enterpriseVerificationIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run enterprise verification PostgreSQL tests", enterpriseVerificationIntegrationDSNEnv)
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	schema := "idc_enterprise_verification_test_" + randomEnterpriseTestHex(t, 8)
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	originalConfig := gdb.GetAllConfig()
	var database gdb.DB
	t.Cleanup(func() {
		if database != nil {
			if closeErr := database.Close(ctx); closeErr != nil {
				t.Errorf("close enterprise verification database: %v", closeErr)
			}
		}
		if configErr := gdb.SetConfig(originalConfig); configErr != nil {
			t.Errorf("restore enterprise verification database config: %v", configErr)
		}
		if _, dropErr := admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`); dropErr != nil {
			t.Errorf("drop enterprise verification schema: %v", dropErr)
		}
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Errorf("close enterprise verification admin database: %v", closeErr)
		}
	})
	if err = gdb.SetConfig(gdb.Config{
		gdb.DefaultGroupName: gdb.ConfigGroup{{
			Link: appendEnterpriseTestDSNParameter(dsn, "search_path="+schema),
		}},
	}); err != nil {
		t.Fatal(err)
	}
	database = g.DB()
	createEnterpriseDictionaryFixtures(t, ctx, database)
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "manifest", "sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"001-compliance-foundation.sql", "003-enterprise-verification-submission.sql",
		"004-enterprise-verification-management.sql", "024-privacy-governance.sql",
	} {
		content, readErr := os.ReadFile(filepath.Join(root, name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, readErr = database.Exec(ctx, string(content)); readErr != nil {
			t.Fatalf("execute %s: %v", name, readErr)
		}
	}
	return database
}

func createEnterpriseDictionaryFixtures(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	if _, err := database.Exec(ctx, `
CREATE TABLE sys_dict_type (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0, name VARCHAR(128) NOT NULL DEFAULT '',
    type VARCHAR(128) NOT NULL, status SMALLINT NOT NULL DEFAULT 1,
    is_builtin SMALLINT NOT NULL DEFAULT 0, allow_tenant_override BOOLEAN NOT NULL DEFAULT FALSE,
    remark VARCHAR(512) NOT NULL DEFAULT '', created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_enterprise_test_dict_type UNIQUE (tenant_id, type)
);
CREATE TABLE sys_dict_data (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0, dict_type VARCHAR(128) NOT NULL,
    label VARCHAR(128) NOT NULL DEFAULT '', value VARCHAR(128) NOT NULL,
    sort INT NOT NULL DEFAULT 0, tag_style VARCHAR(32) NOT NULL DEFAULT 'default',
    status SMALLINT NOT NULL DEFAULT 1, is_builtin SMALLINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_enterprise_test_dict_data UNIQUE (tenant_id, dict_type, value)
)`); err != nil {
		t.Fatal(err)
	}
}

func randomEnterpriseTestHex(t *testing.T, count int) string {
	t.Helper()
	buffer := make([]byte, count)
	if _, err := rand.Read(buffer); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(buffer)
}

func appendEnterpriseTestDSNParameter(dsn, parameter string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + parameter
}
