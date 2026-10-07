package backend

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/storagecap"
	"lina-plugin-idc-compliance/backend/internal/service/privacygovernance"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

type privacyTestAuthorizer struct {
	permissions []string
}

func (a *privacyTestAuthorizer) Authorize(
	_ context.Context,
	request managementcap.AuthorizationRequest,
) (int, error) {
	if err := managementcap.ValidateAuthorizationRequest(request); err != nil {
		return 0, err
	}
	a.permissions = append(a.permissions, request.Permission)
	return 87, nil
}

type privacyTestAudit struct {
	transitions []idccap.BusinessAuditTransition
}

type privacyStorageStub struct{ storagecap.Service }
type privacySecretManagerStub struct{ secretcap.Manager }
type privacyCustomerReaderStub struct {
	subject *identitycap.CustomerSubject
}
type privacyConsentServiceStub struct {
	decisions []identitycap.PrivacyConsentDecision
}

func (s *privacyCustomerReaderStub) CurrentCustomer(context.Context) (*identitycap.CustomerSubject, error) {
	if s.subject == nil {
		return nil, bizerr.NewCode(privacygovernance.CodeNotAccessible)
	}
	copyValue := *s.subject
	return &copyValue, nil
}

func (s *privacyConsentServiceStub) ListCurrentPrivacyConsents(context.Context) ([]identitycap.PrivacyConsentDecision, error) {
	return append([]identitycap.PrivacyConsentDecision(nil), s.decisions...), nil
}

func (s *privacyConsentServiceStub) WithdrawCurrentPrivacyConsent(
	_ context.Context,
	input identitycap.PrivacyConsentWithdrawalInput,
) (*identitycap.PrivacyConsentDecision, error) {
	decision := identitycap.PrivacyConsentDecision{
		ConsentNo: "CNS_WITHDRAWN_TEST_0001", ConsentType: input.ConsentType,
		PolicyReference: "idc://compliance/privacy-test", PolicyVersion: "v1",
		ActionCode: "withdrawn", Locale: "zh-CN",
	}
	s.decisions = []identitycap.PrivacyConsentDecision{decision}
	return &decision, nil
}

func (a *privacyTestAudit) AppendBusinessTransition(
	_ context.Context,
	transition idccap.BusinessAuditTransition,
) error {
	a.transitions = append(a.transitions, transition)
	return nil
}

func (*privacyTestAudit) AppendBusinessBatchTransition(
	context.Context,
	idccap.BusinessAuditBatchTransition,
) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, nil
}

func TestPrivacyGovernancePostgreSQLAssessmentRetentionAndGate(t *testing.T) {
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(complianceIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL privacy governance tests", complianceIntegrationDSNEnv)
	}
	adminDB, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create privacy governance admin database: %v", err)
	}
	defer adminDB.Close(ctx)
	schema := newComplianceIntegrationSchema(t)
	if _, err = adminDB.Exec(ctx, "CREATE SCHEMA "+quoteCompliancePostgreSQLIdentifier(schema)); err != nil {
		t.Fatalf("create privacy governance schema: %v", err)
	}
	defer func() {
		if !strings.HasPrefix(schema, "idc_compliance_test_") {
			t.Errorf("refusing unexpected privacy governance schema %q", schema)
			return
		}
		if _, dropErr := adminDB.Exec(ctx, "DROP SCHEMA "+quoteCompliancePostgreSQLIdentifier(schema)+" CASCADE"); dropErr != nil {
			t.Errorf("drop privacy governance schema: %v", dropErr)
		}
	}()
	database, err := gdb.New(gdb.ConfigNode{Link: appendComplianceDSNParameter(dsn, "search_path="+schema)})
	if err != nil {
		t.Fatalf("create privacy governance scoped database: %v", err)
	}
	defer database.Close(ctx)
	createComplianceDictionaryFixtures(t, ctx, database)
	executeComplianceMigrations(t, ctx, database)

	authorizer := new(privacyTestAuthorizer)
	audit := new(privacyTestAudit)
	customerReader := new(privacyCustomerReaderStub)
	consentService := new(privacyConsentServiceStub)
	service, err := privacygovernance.NewComplete(
		database,
		managementcap.Runtime{Authorizer: authorizer, Audit: audit},
		customerReader,
		consentService,
		audit,
		new(privacyStorageStub),
		new(privacySecretManagerStub),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err = service.RequireCollection(ctx, privacygovernance.AssessmentPersonalVerification, privacygovernance.DataPersonalVerificationMaterial); !bizerr.Is(err, privacygovernance.CodePolicyRequired) {
		t.Fatalf("missing policy gate error = %v", err)
	}
	draft, err := service.SaveAssessmentDraft(ctx, completePrivacyDraft())
	if err != nil || draft == nil || draft.Status != privacygovernance.AssessmentDraft || draft.RowVersion != 1 || draft.CrossBorderTransfer {
		t.Fatalf("SaveAssessmentDraft() = %#v error=%v", draft, err)
	}
	published, err := service.PublishAssessment(ctx, privacygovernance.PublishAssessmentInput{
		AssessmentNo: draft.AssessmentNo, ExpectedRowVersion: draft.RowVersion,
		Confirmed: true, IdempotencyKey: "privacy-publish-postgres-0001",
	})
	if err != nil || published == nil || published.Status != privacygovernance.AssessmentPublished || published.ConfirmedByUserID != 87 || published.RowVersion != 2 {
		t.Fatalf("PublishAssessment() = %#v error=%v", published, err)
	}
	replayed, err := service.PublishAssessment(ctx, privacygovernance.PublishAssessmentInput{
		AssessmentNo: draft.AssessmentNo, ExpectedRowVersion: draft.RowVersion,
		Confirmed: true, IdempotencyKey: "privacy-publish-postgres-0001",
	})
	if err != nil || replayed.AssessmentNo != published.AssessmentNo || replayed.Status != privacygovernance.AssessmentPublished {
		t.Fatalf("PublishAssessment(replay) = %#v error=%v", replayed, err)
	}
	policy, err := service.SetRetentionPolicy(ctx, privacygovernance.RetentionPolicyInput{
		DataType:      privacygovernance.DataPersonalVerificationMaterial,
		RetentionDays: 420, IdempotencyKey: "privacy-retention-postgres-0001",
	})
	if err != nil || policy == nil || policy.RetentionDays != 420 || policy.StatusCode != "active" {
		t.Fatalf("SetRetentionPolicy() = %#v error=%v", policy, err)
	}
	decision, err := service.RequireCollection(ctx, privacygovernance.AssessmentPersonalVerification, privacygovernance.DataPersonalVerificationMaterial)
	if err != nil || decision == nil || decision.AssessmentNo != published.AssessmentNo || decision.PolicyNo != policy.PolicyNo || decision.RetentionUntil.IsZero() {
		t.Fatalf("RequireCollection() = %#v error=%v", decision, err)
	}
	initialGeneration, blocked, err := service.CurrentSensitiveAccessGeneration(ctx)
	if err != nil || blocked || initialGeneration != 1 {
		t.Fatalf("initial sensitive access generation = %d blocked=%v error=%v", initialGeneration, blocked, err)
	}
	incident, err := service.CreateIncident(ctx, privacygovernance.IncidentCreateInput{
		SeverityCode: "high", SourceCode: "bulk_download",
		SafeSummary:    "A bounded local audit rule detected an unusual download pattern.",
		IdempotencyKey: "privacy-incident-postgres-0001",
	})
	if err != nil || incident == nil || incident.StatusCode != "discovered" || !incident.ExportBlocked {
		t.Fatalf("CreateIncident() = %#v error=%v", incident, err)
	}
	rotatedGeneration, blocked, err := service.CurrentSensitiveAccessGeneration(ctx)
	if err != nil || !blocked || rotatedGeneration <= initialGeneration {
		t.Fatalf("rotated sensitive access generation = %d blocked=%v error=%v", rotatedGeneration, blocked, err)
	}
	for index, stage := range []string{"contained", "assessed", "notification_pending", "notification_confirmed", "remediating", "resolved"} {
		incident, err = service.AdvanceIncident(ctx, privacygovernance.IncidentAdvanceInput{
			IncidentNo: incident.IncidentNo, ExpectedVersion: incident.Version,
			NextStageCode: stage, SafeNote: "Completed the local " + stage + " response step.",
			ConfirmExternalNotification: stage == "notification_confirmed",
			IdempotencyKey:              "privacy-advance-postgres-000" + string(rune('1'+index)),
		})
		if err != nil || incident.StatusCode != stage {
			t.Fatalf("AdvanceIncident(%s) = %#v error=%v", stage, incident, err)
		}
	}
	_, blocked, err = service.CurrentSensitiveAccessGeneration(ctx)
	if err != nil || blocked {
		t.Fatalf("resolved sensitive access blocked=%v error=%v", blocked, err)
	}
	aiDraftInput := completePrivacyDraft()
	aiDraftInput.Type = privacygovernance.AssessmentAISafeProjection
	aiDraftInput.Purpose = "Project only explicitly approved de-identified status and aggregate fields."
	aiDraftInput.AIAllowedFields = []string{"personal_verification_status", "aggregate_case_count"}
	aiDraft, err := service.SaveAssessmentDraft(ctx, aiDraftInput)
	if err != nil {
		t.Fatalf("SaveAssessmentDraft(AI) error = %v", err)
	}
	if _, err = service.PublishAssessment(ctx, privacygovernance.PublishAssessmentInput{
		AssessmentNo: aiDraft.AssessmentNo, ExpectedRowVersion: aiDraft.RowVersion,
		Confirmed: true, IdempotencyKey: "privacy-ai-publish-postgres-0001",
	}); err != nil {
		t.Fatalf("PublishAssessment(AI) error = %v", err)
	}
	if _, err = service.ProjectAISafeFields(ctx, map[string]any{
		"personal_verification_status": "verified",
	}); !bizerr.Is(err, privacygovernance.CodePolicyRequired) {
		t.Fatalf("ProjectAISafeFields(missing retention policy) error = %v", err)
	}
	if _, err = service.SetRetentionPolicy(ctx, privacygovernance.RetentionPolicyInput{
		DataType: privacygovernance.DataSensitiveAccessAudit, RetentionDays: 365,
		IdempotencyKey: "privacy-ai-retention-postgres-0001",
	}); err != nil {
		t.Fatalf("SetRetentionPolicy(AI safe projection) error = %v", err)
	}
	projected, err := service.ProjectAISafeFields(ctx, map[string]any{
		"personal_verification_status": "verified",
		"aggregate_case_count":         int64(12),
	})
	if err != nil || projected["personal_verification_status"] != "verified" || projected["aggregate_case_count"] != int64(12) {
		t.Fatalf("ProjectAISafeFields(allowed) = %#v error=%v", projected, err)
	}
	if _, err = service.ProjectAISafeFields(ctx, map[string]any{"identity_card_number": "110101199001011234"}); !bizerr.Is(err, privacygovernance.CodeNotAccessible) {
		t.Fatalf("ProjectAISafeFields(raw identity) error = %v", err)
	}
	if _, err = service.ProjectAISafeFields(ctx, map[string]any{"personal_verification_status": "customer/123"}); !bizerr.Is(err, privacygovernance.CodeNotAccessible) {
		t.Fatalf("ProjectAISafeFields(re-identifiable status) error = %v", err)
	}

	exportDraftInput := completePrivacyDraft()
	exportDraftInput.Type = privacygovernance.AssessmentLocalSensitiveExport
	exportDraftInput.Purpose = "Generate a minimal redacted package for a separately agreed local support handoff."
	exportDraft, err := service.SaveAssessmentDraft(ctx, exportDraftInput)
	if err != nil {
		t.Fatalf("SaveAssessmentDraft(local export) error = %v", err)
	}
	if _, err = service.PublishAssessment(ctx, privacygovernance.PublishAssessmentInput{
		AssessmentNo: exportDraft.AssessmentNo, ExpectedRowVersion: exportDraft.RowVersion,
		Confirmed: true, IdempotencyKey: "privacy-export-publish-postgres-0001",
	}); err != nil {
		t.Fatalf("PublishAssessment(local export) error = %v", err)
	}
	if _, err = service.SetRetentionPolicy(ctx, privacygovernance.RetentionPolicyInput{
		DataType: privacygovernance.DataLocalSensitiveExport, RetentionDays: 30,
		IdempotencyKey: "privacy-export-retention-postgres-0001",
	}); err != nil {
		t.Fatalf("SetRetentionPolicy(local export) error = %v", err)
	}
	exported, err := service.GenerateSupportExport(ctx, privacygovernance.SupportExportInput{
		SupportAgreementConfirmed: true,
		IdempotencyKey:            "privacy-support-export-postgres-0001",
	})
	if err != nil || exported == nil || len(exported.Content) == 0 {
		t.Fatalf("GenerateSupportExport() = %#v error=%v", exported, err)
	}
	replayedExport, err := service.GenerateSupportExport(ctx, privacygovernance.SupportExportInput{
		SupportAgreementConfirmed: true,
		IdempotencyKey:            "privacy-support-export-postgres-0001",
	})
	if err != nil || replayedExport.ExportNo != exported.ExportNo || string(replayedExport.Content) != string(exported.Content) || replayedExport.ContentSHA256 != exported.ContentSHA256 {
		t.Fatalf("GenerateSupportExport(replay) = %#v error=%v", replayedExport, err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_compliance_privacy_support_export SET retention_until = CURRENT_TIMESTAMP - INTERVAL '1 minute' WHERE export_no = ?`, exported.ExportNo); err != nil {
		t.Fatalf("expire local export: %v", err)
	}
	created, err := service.GenerateDueDeletionTasks(ctx, 10)
	if err != nil || created != 1 {
		t.Fatalf("GenerateDueDeletionTasks() created=%d error=%v", created, err)
	}
	deletionTasks, err := service.ListDeletionTasks(ctx, 10)
	if err != nil || len(deletionTasks) != 1 || deletionTasks[0].TargetNo != exported.ExportNo {
		t.Fatalf("ListDeletionTasks() = %#v error=%v", deletionTasks, err)
	}
	task := deletionTasks[0]
	if err = service.PlaceRetentionHold(ctx, privacygovernance.RetentionHoldInput{
		TargetTypeCode: task.TargetTypeCode, TargetNo: task.TargetNo, ReasonCode: "audit",
		OwnerCode: "compliance.audit", IdempotencyKey: "privacy-hold-postgres-0001",
	}); err != nil {
		t.Fatalf("PlaceRetentionHold() error = %v", err)
	}
	if _, err = service.ConfirmDeletion(ctx, privacygovernance.DeletionConfirmInput{
		TaskNo: task.TaskNo, ExpectedVersion: task.Version, Confirmed: true,
		IdempotencyKey: "privacy-delete-postgres-0001",
	}); !bizerr.Is(err, privacygovernance.CodeConflict) {
		t.Fatalf("ConfirmDeletion(active hold) error = %v", err)
	}
	deletionTasks, err = service.ListDeletionTasks(ctx, 10)
	if err != nil || deletionTasks[0].StatusCode != "blocked" || deletionTasks[0].HoldReasonCode != "audit" {
		t.Fatalf("blocked deletion task = %#v error=%v", deletionTasks, err)
	}
	if err = service.ReleaseRetentionHold(ctx, privacygovernance.RetentionHoldReleaseInput{
		TargetTypeCode: task.TargetTypeCode, TargetNo: task.TargetNo, ReasonCode: "audit",
		OwnerCode: "compliance.audit", IdempotencyKey: "privacy-release-postgres-0001",
	}); err != nil {
		t.Fatalf("ReleaseRetentionHold() error = %v", err)
	}
	deletionTasks, err = service.ListDeletionTasks(ctx, 10)
	if err != nil || deletionTasks[0].StatusCode != "pending" {
		t.Fatalf("released deletion task = %#v error=%v", deletionTasks, err)
	}
	completed, err := service.ConfirmDeletion(ctx, privacygovernance.DeletionConfirmInput{
		TaskNo: deletionTasks[0].TaskNo, ExpectedVersion: deletionTasks[0].Version, Confirmed: true,
		IdempotencyKey: "privacy-delete-postgres-0002",
	})
	if err != nil || completed.StatusCode != "completed" {
		t.Fatalf("ConfirmDeletion() = %#v error=%v", completed, err)
	}
	var disposed []struct {
		StatusCode string `json:"status_code"`
		Content    []byte `json:"content_bytes"`
	}
	if err = database.GetScan(ctx, &disposed, `SELECT status_code, content_bytes FROM plugin_idc_compliance_privacy_support_export WHERE export_no = ?`, exported.ExportNo); err != nil || len(disposed) != 1 || disposed[0].StatusCode != "deleted" || string(disposed[0].Content) != "{}" {
		t.Fatalf("disposed local export = %#v error=%v", disposed, err)
	}

	customerReader.subject = &identitycap.CustomerSubject{
		CustomerNo: "CUS_PRIVACY_ALPHA_0001", CustomerTypeCode: "personal",
		PrincipalReference: "usr:9101", ActorUserID: 9101,
	}
	consentService.decisions = []identitycap.PrivacyConsentDecision{{
		ConsentNo: "CNS_PRIVACY_ALPHA_0001", ConsentType: identitycap.PrivacyConsentSensitivePersonal,
		PolicyReference: identitycap.PersonalConsentPolicyReference,
		PolicyVersion:   identitycap.PersonalConsentPolicyVersion,
		ActionCode:      "granted", Locale: "zh-CN",
	}}
	alphaRequest, err := service.CreateCurrentCustomerRequest(ctx, privacygovernance.CustomerPrivacyRequestInput{
		RequestType: "export", IdempotencyKey: "privacy-customer-request-shared-0001",
	})
	if err != nil || alphaRequest == nil {
		t.Fatalf("CreateCurrentCustomerRequest(alpha) = %#v error=%v", alphaRequest, err)
	}
	alphaReplay, err := service.CreateCurrentCustomerRequest(ctx, privacygovernance.CustomerPrivacyRequestInput{
		RequestType: "export", IdempotencyKey: "privacy-customer-request-shared-0001",
	})
	if err != nil || alphaReplay.RequestNo != alphaRequest.RequestNo {
		t.Fatalf("CreateCurrentCustomerRequest(alpha replay) = %#v error=%v", alphaReplay, err)
	}

	customerReader.subject = &identitycap.CustomerSubject{
		CustomerNo: "CUS_PRIVACY_BRAVO_0001", CustomerTypeCode: "enterprise",
		PrincipalReference: "usr:9102", ActorUserID: 9102,
	}
	consentService.decisions = []identitycap.PrivacyConsentDecision{{
		ConsentNo: "CNS_PRIVACY_BRAVO_0001", ConsentType: identitycap.PrivacyConsentEnterprise,
		PolicyReference: identitycap.EnterpriseConsentPolicyReference,
		PolicyVersion:   identitycap.EnterpriseConsentPolicyVersion,
		ActionCode:      "granted", Locale: "en-US",
	}}
	bravoRequest, err := service.CreateCurrentCustomerRequest(ctx, privacygovernance.CustomerPrivacyRequestInput{
		RequestType: "export", IdempotencyKey: "privacy-customer-request-shared-0001",
	})
	if err != nil || bravoRequest == nil || bravoRequest.RequestNo == alphaRequest.RequestNo {
		t.Fatalf("CreateCurrentCustomerRequest(bravo) = %#v error=%v", bravoRequest, err)
	}
	bravoOverview, err := service.CurrentCustomerPrivacy(ctx)
	if err != nil || len(bravoOverview.Requests) != 1 || bravoOverview.Requests[0].RequestNo != bravoRequest.RequestNo ||
		len(bravoOverview.Consents) != 1 || bravoOverview.Consents[0].ConsentType != identitycap.PrivacyConsentEnterprise {
		t.Fatalf("CurrentCustomerPrivacy(bravo) = %#v error=%v", bravoOverview, err)
	}

	customerReader.subject = &identitycap.CustomerSubject{
		CustomerNo: "CUS_PRIVACY_ALPHA_0001", CustomerTypeCode: "personal",
		PrincipalReference: "usr:9101", ActorUserID: 9101,
	}
	consentService.decisions = []identitycap.PrivacyConsentDecision{{
		ConsentNo: "CNS_PRIVACY_ALPHA_0001", ConsentType: identitycap.PrivacyConsentSensitivePersonal,
		PolicyReference: identitycap.PersonalConsentPolicyReference,
		PolicyVersion:   identitycap.PersonalConsentPolicyVersion,
		ActionCode:      "granted", Locale: "zh-CN",
	}}
	alphaOverview, err := service.CurrentCustomerPrivacy(ctx)
	if err != nil || len(alphaOverview.Requests) != 1 || alphaOverview.Requests[0].RequestNo != alphaRequest.RequestNo ||
		len(alphaOverview.Consents) != 1 || alphaOverview.Consents[0].ConsentType != identitycap.PrivacyConsentSensitivePersonal ||
		len(alphaOverview.Notices) == 0 || alphaOverview.Notices[0].RetentionDays != 420 {
		t.Fatalf("CurrentCustomerPrivacy(alpha) = %#v error=%v", alphaOverview, err)
	}
	requestCount, err := database.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_compliance_privacy_customer_request WHERE customer_no IN (?, ?)`, "CUS_PRIVACY_ALPHA_0001", "CUS_PRIVACY_BRAVO_0001")
	if err != nil || requestCount.Int() != 2 {
		t.Fatalf("isolated customer request count=%d error=%v", requestCount.Int(), err)
	}
	if len(authorizer.permissions) < 10 || len(audit.transitions) < 10 {
		t.Fatalf("permissions=%v audits=%d", authorizer.permissions, len(audit.transitions))
	}
}

func completePrivacyDraft() privacygovernance.AssessmentDraftInput {
	return privacygovernance.AssessmentDraftInput{
		Type:                     privacygovernance.AssessmentPersonalVerification,
		Purpose:                  "Verify the account holder before first payment.",
		Necessity:                "Meet the merchant's local identity obligations.",
		Impact:                   "The submitted identity information is checked for this request.",
		Recipients:               "The current IDC merchant and its configured verification supplier.",
		StorageLocation:          "This installation and the merchant-configured local supplier.",
		RetentionDescription:     "Uses the active personal verification material policy.",
		RightsEntry:              "Privacy and authorization page in the customer center.",
		MerchantRole:             "Personal information processor.",
		ThirdPartyRole:           "Entrusted verification processor.",
		ProcessingInstruction:    "Process only the submitted verification request.",
		SubcontractingBoundary:   "No subcontracting outside the configured supplier contract.",
		AuditEvidenceDescription: "Consent, request, result, access, and deletion audit records.",
	}
}
