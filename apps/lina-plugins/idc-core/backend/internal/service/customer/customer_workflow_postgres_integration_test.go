// This file verifies customer workflow transaction invariants against real PostgreSQL.

package customer

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/asynctask"
	"lina-plugin-idc-core/backend/internal/service/reauth"
)

func TestDAOLegalChangeConvergesWithoutOverwritingVerifiedProjection(t *testing.T) {
	ctx := context.Background()
	configureCustomerIntegrationDB(t, ctx)
	insertCustomerIntegrationProfile(t, ctx, testCustomerNo, TypeIndividual, StatusActive)
	verifiedAt := time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC)
	seedVerifiedIndividualProfile(t, ctx, testCustomerNo, verifiedAt)

	writer := &transactionalAuditWriter{}
	persistence := &daoPersistence{audit: writer, now: func() time.Time {
		return time.Date(2026, 8, 7, 1, 0, 0, 0, time.UTC)
	}, random: rand.Reader}
	input, err := normalizeLegalChange(RequestOwnLegalChangeInput{
		CustomerNo: testCustomerNo, Fields: []LegalChangeField{LegalChangeFieldName},
		Reason: LegalChangeReasonNameChange, ExpectedProfileVersion: 1,
		IdempotencyKey: "legal-concurrent-1",
	})
	if err != nil {
		t.Fatalf("normalize legal change: %v", err)
	}

	const callers = 8
	results := make([]*LegalChangeRequest, callers)
	errorsOut := make([]error, callers)
	start := make(chan struct{})
	var waitGroup sync.WaitGroup
	for index := range callers {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			results[index], errorsOut[index] = persistence.requestOwnLegalChange(ctx, 81, input)
		}()
	}
	close(start)
	waitGroup.Wait()
	requestNo := ""
	for index := range callers {
		if errorsOut[index] != nil || results[index] == nil {
			t.Fatalf("legal replay %d result=%+v err=%v", index, results[index], errorsOut[index])
		}
		if requestNo == "" {
			requestNo = results[index].RequestNo
		} else if results[index].RequestNo != requestNo {
			t.Fatalf("legal replay diverged: %q != %q", results[index].RequestNo, requestNo)
		}
	}
	assertCustomerIntegrationCount(t, ctx, dao.CustomerLegalChangeRequest.Table(), "request_no", requestNo, 1)
	current, currentErr := persistence.currentOwnLegalChange(ctx, testCustomerNo)
	if currentErr != nil || current == nil || current.RequestNo != requestNo ||
		current.Status != LegalChangeStatusSubmitted {
		t.Fatalf("current legal change=%+v err=%v", current, currentErr)
	}
	if transitions := writer.transitionSnapshot(); len(transitions) != 1 {
		t.Fatalf("legal audit transitions=%d want=1", len(transitions))
	}

	var profile *entity.CustomerIndividualProfile
	if err = dao.CustomerIndividualProfile.Ctx(ctx).Where(do.CustomerIndividualProfile{
		TenantId: idccap.DefaultTenantID, CustomerNo: testCustomerNo,
	}).Scan(&profile); err != nil {
		t.Fatalf("load verified profile after legal request: %v", err)
	}
	if profile == nil || profile.LegalName != "Verified Person" ||
		profile.IdentityIdentifierMasked != "110***********001" ||
		profile.IdentityStatusCode != string(VerificationVerified) ||
		profile.IdentityCaseNo != "CASE_VERIFIED_INDIVIDUAL" || profile.Version != 1 {
		t.Fatalf("legal request overwrote certification projection: %+v", profile)
	}
	encoded, err := json.Marshal(writer.transitionSnapshot()[0])
	if err != nil {
		t.Fatalf("marshal legal audit: %v", err)
	}
	if strings.Contains(string(encoded), "Verified Person") || strings.Contains(string(encoded), "110***********001") {
		t.Fatalf("legal audit contains identity values: %s", encoded)
	}
}

func TestDAOAccountTransitionIsAtomicAndMatrixBounded(t *testing.T) {
	ctx := context.Background()
	configureCustomerIntegrationDB(t, ctx)
	insertCustomerIntegrationProfile(t, ctx, testCustomerNo, TypeIndividual, StatusActive)
	writer := &transactionalAuditWriter{}
	persistence := &daoPersistence{audit: writer, now: func() time.Time {
		return time.Date(2026, 8, 7, 2, 0, 0, 0, time.UTC)
	}, random: rand.Reader}
	input, err := normalizeAccountTransition(TransitionAccountInput{
		CustomerNo: testCustomerNo, ToStatus: StatusRestricted,
		Reason: AccountReasonRiskRestriction, ExpectedVersion: 1,
		IdempotencyKey: "state-transition-1",
	})
	if err != nil {
		t.Fatalf("normalize account transition: %v", err)
	}
	transition, err := persistence.transitionAccount(ctx, 99, input)
	if err != nil || transition == nil || transition.FromStatus != StatusActive ||
		transition.ToStatus != StatusRestricted || transition.SubjectVersion != 2 {
		t.Fatalf("account transition result=%+v err=%v", transition, err)
	}
	var subject *entity.CustomerSubject
	if err = dao.CustomerSubject.Ctx(ctx).Where(do.CustomerSubject{
		TenantId: idccap.DefaultTenantID, CustomerNo: testCustomerNo,
	}).Scan(&subject); err != nil {
		t.Fatalf("load transitioned subject: %v", err)
	}
	if subject == nil || subject.StatusCode != string(StatusRestricted) || subject.Version != 2 ||
		subject.StatusReasonCode != string(AccountReasonRiskRestriction) || subject.StatusChangedBy != 99 {
		t.Fatalf("unexpected transitioned subject: %+v", subject)
	}
	assertCustomerIntegrationCount(t, ctx, dao.CustomerAccountStateHistory.Table(), "transition_no", transition.TransitionNo, 1)
	invalidTime := time.Date(2026, 8, 7, 2, 1, 0, 0, time.UTC)
	if _, err = dao.CustomerAccountStateHistory.Ctx(ctx).Data(do.CustomerAccountStateHistory{
		TenantId:     idccap.DefaultTenantID,
		TransitionNo: "CAT_ffffffffffffffffffffffffffffffff",
		CustomerNo:   testCustomerNo, FromStatusCode: string(StatusRestricted),
		ToStatusCode: string(StatusAnonymized), ReasonCode: string(AccountReasonManualDisable),
		SourceCode: "staff", ActorUserId: 99, SubjectVersion: 3,
		IdempotencyKeyDigest: strings.Repeat("d", 64), RequestFingerprint: strings.Repeat("e", 64),
		OccurredAt: &invalidTime,
	}).Insert(); err == nil {
		t.Fatal("database state matrix accepted a generic anonymized transition")
	}

	invalid := normalizedAccountTransition{
		customerNo: testCustomerNo, toStatus: StatusAnonymized,
		reason: AccountReasonManualDisable, expectedVersion: 2, idempotencyKey: "state-transition-2",
	}
	if _, err = persistence.transitionAccount(ctx, 99, invalid); !bizerr.Is(err, CodeStateDenied) {
		t.Fatalf("generic anonymized transition returned %v", err)
	}
}

func TestDAOAccountTransitionRollsBackWhenAuditFails(t *testing.T) {
	ctx := context.Background()
	configureCustomerIntegrationDB(t, ctx)
	insertCustomerIntegrationProfile(t, ctx, testCustomerNo, TypeIndividual, StatusActive)
	cause := errors.New("audit unavailable")
	persistence := &daoPersistence{
		audit: &transactionalAuditWriter{err: cause}, now: time.Now, random: rand.Reader,
	}
	input, err := normalizeAccountTransition(TransitionAccountInput{
		CustomerNo: testCustomerNo, ToStatus: StatusRestricted,
		Reason: AccountReasonRiskRestriction, ExpectedVersion: 1,
		IdempotencyKey: "state-rollback",
	})
	if err != nil {
		t.Fatalf("normalize rollback transition: %v", err)
	}
	if _, err = persistence.transitionAccount(ctx, 99, input); !errors.Is(err, cause) {
		t.Fatalf("account transition rollback returned %v", err)
	}
	var subject *entity.CustomerSubject
	if err = dao.CustomerSubject.Ctx(ctx).Where(do.CustomerSubject{
		TenantId: idccap.DefaultTenantID, CustomerNo: testCustomerNo,
	}).Scan(&subject); err != nil {
		t.Fatalf("load rolled-back account subject: %v", err)
	}
	if subject == nil || subject.StatusCode != string(StatusActive) || subject.Version != 1 {
		t.Fatalf("account transition escaped audit rollback: %+v", subject)
	}
	assertCustomerIntegrationCount(t, ctx, dao.CustomerAccountStateHistory.Table(), "customer_no", testCustomerNo, 0)
}

func TestDAOClosurePersistsConsentAndRetentionOnlyAfterClearPrecheck(t *testing.T) {
	ctx := context.Background()
	configureCustomerIntegrationDB(t, ctx)
	insertCustomerIntegrationProfile(t, ctx, testCustomerNo, TypeIndividual, StatusActive)
	now := time.Date(2026, 8, 7, 3, 0, 0, 0, time.UTC)
	persistence := &daoPersistence{
		audit: &transactionalAuditWriter{}, now: func() time.Time { return now }, random: rand.Reader,
	}
	input, err := normalizeClosureRequest(RequestOwnClosureInput{
		CustomerNo: testCustomerNo, ExpectedVersion: 1,
		Confirmed: true, IdempotencyKey: "closure-request-1",
	})
	if err != nil {
		t.Fatalf("normalize closure request: %v", err)
	}
	precheck := ClosurePrecheckResult{
		CustomerNo: testCustomerNo, ContractVersion: closurePrecheckContractVersion,
		CheckedAt: now.Add(-time.Second),
		Findings:  []ClosureFinding{{Code: ClosureFindingLegalRetention, Count: 2}},
	}
	request, err := persistence.requestOwnClosure(ctx, 81, input, precheck)
	if err != nil || request == nil || request.Status != ClosureStatusPendingReview ||
		!validConsentNo(request.ConsentNo) || request.RetentionPolicyCode != closureRetentionPolicy {
		t.Fatalf("closure request=%+v err=%v", request, err)
	}
	var consent *entity.CustomerConsent
	if err = dao.CustomerConsent.Ctx(ctx).Where(do.CustomerConsent{
		TenantId: idccap.DefaultTenantID, ConsentNo: request.ConsentNo,
		CustomerNo: testCustomerNo,
	}).Scan(&consent); err != nil {
		t.Fatalf("load closure consent: %v", err)
	}
	if consent == nil || consent.ConsentTypeCode != closureConsentType || consent.ActionCode != "granted" ||
		consent.PolicyReference != closurePolicyReference || consent.PolicyVersion != customerWorkflowPolicyVersion ||
		!validDigest(consent.EvidenceDigest) {
		t.Fatalf("unexpected closure consent: %+v", consent)
	}
	var row *entity.CustomerClosureRequest
	if err = dao.CustomerClosureRequest.Ctx(ctx).Where(do.CustomerClosureRequest{
		TenantId: idccap.DefaultTenantID, RequestNo: request.RequestNo,
	}).Scan(&row); err != nil {
		t.Fatalf("load closure row: %v", err)
	}
	if row == nil || row.LegalRetentionCount != 2 || row.ActiveServiceCount != 0 ||
		row.ConsentNo != request.ConsentNo || row.StatusCode != string(ClosureStatusPendingReview) {
		t.Fatalf("unexpected closure row: %+v", row)
	}
	if _, err = dao.CustomerClosureRequest.Ctx(ctx).Where(do.CustomerClosureRequest{
		TenantId: idccap.DefaultTenantID, RequestNo: request.RequestNo,
	}).Data(do.CustomerClosureRequest{ActiveServiceCount: 1}).Update(); err == nil {
		t.Fatal("database accepted a retained closure request with a hard blocker")
	}
}

func TestDAODataExportCreatesConsentTaskRequestAndAuditInOneTransaction(t *testing.T) {
	ctx := context.Background()
	configureCustomerIntegrationDB(t, ctx)
	insertCustomerIntegrationProfile(t, ctx, testCustomerNo, TypeIndividual, StatusActive)
	now := time.Date(2026, 8, 7, 4, 0, 0, 0, time.UTC)
	taskSvc, err := asynctask.New(asynctask.Config{})
	if err != nil {
		t.Fatalf("create async task service: %v", err)
	}
	writer := &transactionalAuditWriter{}
	persistence := &daoPersistence{
		audit: writer, tasks: taskSvc, now: func() time.Time { return now }, random: rand.Reader,
	}
	input, err := normalizeDataExport(OwnDataExportIntent{
		CustomerNo: testCustomerNo, Scope: DataExportScopeOwnDeliverable,
		Format: DataExportFormatJSONZip, ExpectedVersion: 1,
		IdempotencyKey: "data-export-1", Confirmed: true,
	})
	if err != nil {
		t.Fatalf("normalize data export: %v", err)
	}
	proof := &reauth.Proof{
		Purpose: reauth.PurposeSensitiveExport, TargetDigest: strings.Repeat("b", 64),
		UserID: 81, TenantID: idccap.DefaultTenantID,
		LoginAt: now.Add(-time.Minute), IssuedAt: now, ExpiresAt: now.Add(time.Minute),
		SessionDigest: strings.Repeat("c", 64),
	}
	request, err := persistence.requestOwnDataExport(ctx, 81, input, proof)
	if err != nil || request == nil || !validConsentNo(request.ConsentNo) ||
		!validWorkflowNo(request.TaskNo, "TSK_") {
		t.Fatalf("data export request=%+v err=%v", request, err)
	}
	var task *entity.AsyncTask
	if err = dao.AsyncTask.Ctx(ctx).Where(do.AsyncTask{
		TenantId: idccap.DefaultTenantID, TaskNo: request.TaskNo,
	}).Scan(&task); err != nil {
		t.Fatalf("load data export task: %v", err)
	}
	if task == nil || task.TaskType != dataExportTaskType || task.SubjectType != dataExportTaskSubjectType ||
		task.SubjectNo != testCustomerNo || task.CustomerNo != testCustomerNo ||
		strings.Contains(string(task.Payload), proof.SessionDigest) ||
		strings.Contains(string(task.Payload), "proof") {
		t.Fatalf("unexpected data export task: %+v", task)
	}
	var row *entity.CustomerDataExportRequest
	if err = dao.CustomerDataExportRequest.Ctx(ctx).Where(do.CustomerDataExportRequest{
		TenantId: idccap.DefaultTenantID, RequestNo: request.RequestNo,
	}).Scan(&row); err != nil {
		t.Fatalf("load data export workflow row: %v", err)
	}
	if row == nil || row.TaskNo != task.TaskNo || row.ConsentNo != request.ConsentNo ||
		row.ScopeCode != string(DataExportScopeOwnDeliverable) || row.FormatCode != string(DataExportFormatJSONZip) {
		t.Fatalf("unexpected data export workflow row: %+v", row)
	}
	if transitions := writer.transitionSnapshot(); len(transitions) != 1 ||
		transitions[0].CorrelationNo != task.TaskNo {
		t.Fatalf("unexpected data export audit transitions: %+v", transitions)
	}
}

func TestDAODataExportRollsBackTaskConsentAndRequestWhenAuditFails(t *testing.T) {
	ctx := context.Background()
	configureCustomerIntegrationDB(t, ctx)
	insertCustomerIntegrationProfile(t, ctx, testCustomerNo, TypeIndividual, StatusActive)
	now := time.Date(2026, 8, 7, 5, 0, 0, 0, time.UTC)
	taskSvc, err := asynctask.New(asynctask.Config{})
	if err != nil {
		t.Fatalf("create async task service: %v", err)
	}
	cause := errors.New("audit unavailable")
	persistence := &daoPersistence{
		audit: &transactionalAuditWriter{err: cause}, tasks: taskSvc,
		now: func() time.Time { return now }, random: rand.Reader,
	}
	input, err := normalizeDataExport(OwnDataExportIntent{
		CustomerNo: testCustomerNo, Scope: DataExportScopeOwnDeliverable,
		Format: DataExportFormatJSONZip, ExpectedVersion: 1,
		IdempotencyKey: "data-export-rollback", Confirmed: true,
	})
	if err != nil {
		t.Fatalf("normalize rollback export: %v", err)
	}
	proof := &reauth.Proof{IssuedAt: now}
	if _, err = persistence.requestOwnDataExport(ctx, 81, input, proof); !errors.Is(err, cause) {
		t.Fatalf("data export rollback returned %v", err)
	}
	assertCustomerIntegrationCount(t, ctx, dao.CustomerDataExportRequest.Table(), "customer_no", testCustomerNo, 0)
	assertCustomerIntegrationCount(t, ctx, dao.AsyncTask.Table(), "customer_no", testCustomerNo, 0)
	assertCustomerIntegrationCount(t, ctx, dao.CustomerConsent.Table(), "customer_no", testCustomerNo, 0)
}

func TestCustomerWorkflowDictionaryMirrorsBothLocales(t *testing.T) {
	ctx := context.Background()
	configureCustomerIntegrationDB(t, ctx)
	currentSchema, err := g.DB().GetValue(ctx, "SELECT current_schema()")
	if err != nil {
		t.Fatalf("load workflow test schema: %v", err)
	}
	workflowTables := []string{
		dao.CustomerLegalChangeRequest.Table(), dao.CustomerAccountStateHistory.Table(),
		dao.CustomerClosureRequest.Table(), dao.CustomerDataExportRequest.Table(),
	}
	deletedAtCount, err := g.DB().Model("information_schema.columns").Ctx(ctx).
		Where("table_schema", currentSchema.String()).WhereIn("table_name", workflowTables).
		Where("column_name", "deleted_at").Count()
	if err != nil || deletedAtCount != 0 {
		t.Fatalf("workflow soft-delete columns=%d want=0 err=%v", deletedAtCount, err)
	}
	types := []string{
		"idc_customer_account_status_reason",
		"idc_customer_account_transition_source",
		"idc_customer_closure_finding",
		"idc_customer_closure_status",
		"idc_customer_data_export_format",
		"idc_customer_data_export_scope",
		"idc_customer_legal_change_reason",
		"idc_customer_legal_change_status",
	}
	type dictionaryRow struct {
		DictType string `orm:"dict_type"`
		Value    string `orm:"value"`
	}
	var rows []dictionaryRow
	if err = g.DB().Model("sys_dict_data").Ctx(ctx).
		Fields("dict_type", "value").WhereIn("dict_type", types).Scan(&rows); err != nil {
		t.Fatalf("load workflow dictionary rows: %v", err)
	}
	databaseValues := make(map[string][]string, len(types))
	for _, row := range rows {
		databaseValues[row.DictType] = append(databaseValues[row.DictType], row.Value)
	}
	for _, values := range databaseValues {
		sort.Strings(values)
	}

	manifestDir := filepath.Dir(customerMigrationPath(t, "007-customer-account-workflows.sql"))
	for _, locale := range []string{"en-US", "zh-CN"} {
		content, err := os.ReadFile(filepath.Join(manifestDir, "..", "i18n", locale, "dict.json"))
		if err != nil {
			t.Fatalf("read %s workflow dictionary: %v", locale, err)
		}
		var document struct {
			Dict map[string]map[string]json.RawMessage `json:"dict"`
		}
		if err = json.Unmarshal(content, &document); err != nil {
			t.Fatalf("decode %s workflow dictionary: %v", locale, err)
		}
		for _, dictionaryType := range types {
			entries, exists := document.Dict[dictionaryType]
			if !exists {
				t.Fatalf("%s dictionary is missing type %s", locale, dictionaryType)
			}
			localizedValues := make([]string, 0, len(entries)-2)
			for key := range entries {
				if key != "name" && key != "remark" {
					localizedValues = append(localizedValues, key)
				}
			}
			sort.Strings(localizedValues)
			if strings.Join(localizedValues, "\x00") != strings.Join(databaseValues[dictionaryType], "\x00") {
				t.Fatalf(
					"%s dictionary %s values=%v database=%v",
					locale, dictionaryType, localizedValues, databaseValues[dictionaryType],
				)
			}
		}
	}
}

func seedVerifiedIndividualProfile(
	t *testing.T,
	ctx context.Context,
	customerNo string,
	verifiedAt time.Time,
) {
	t.Helper()
	if _, err := dao.CustomerIndividualProfile.Ctx(ctx).Where(do.CustomerIndividualProfile{
		TenantId: idccap.DefaultTenantID, CustomerNo: customerNo,
	}).Data(do.CustomerIndividualProfile{
		LegalName: "Verified Person", LegalNameMasked: "V*** P*****",
		IdentityIdentifierMasked: "110***********001",
		IdentityLookupDigest:     strings.Repeat("a", 64),
		IdentityStatusCode:       string(VerificationVerified),
		IdentityCaseNo:           "CASE_VERIFIED_INDIVIDUAL", IdentityVerifiedAt: &verifiedAt,
	}).Update(); err != nil {
		t.Fatalf("seed verified individual profile: %v", err)
	}
}
