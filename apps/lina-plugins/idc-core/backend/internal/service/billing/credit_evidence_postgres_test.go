package billing

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"lina-core/pkg/plugin/capability/storagecap"
)

type creditEvidenceStorageStub struct {
	storagecap.Service
	objects map[string][]byte
}

func (s *creditEvidenceStorageStub) Put(_ context.Context, input storagecap.PutInput) (*storagecap.PutOutput, error) {
	content, err := io.ReadAll(input.Body)
	if err != nil {
		return nil, err
	}
	if s.objects == nil {
		s.objects = make(map[string][]byte)
	}
	if _, exists := s.objects[input.Path]; exists && !input.Overwrite {
		return nil, errors.New("object exists")
	}
	s.objects[input.Path] = append([]byte(nil), content...)
	return &storagecap.PutOutput{Object: &storagecap.Object{
		Path: input.Path, Size: int64(len(content)), ContentType: input.ContentType,
		Visibility: storagecap.VisibilityPrivate,
	}}, nil
}

func (s *creditEvidenceStorageStub) Get(_ context.Context, input storagecap.GetInput) (*storagecap.GetOutput, error) {
	content, exists := s.objects[input.Path]
	if !exists {
		return &storagecap.GetOutput{Found: false}, nil
	}
	return &storagecap.GetOutput{
		Found:  true,
		Object: &storagecap.Object{Path: input.Path, Size: int64(len(content)), Visibility: storagecap.VisibilityPrivate},
		Body:   io.NopCloser(bytes.NewReader(content)),
	}, nil
}

func (s *creditEvidenceStorageStub) Delete(_ context.Context, input storagecap.DeleteInput) error {
	delete(s.objects, input.Path)
	return nil
}

func TestCreditEvidencePostgreSQLStaysPrivateAndSupportsExactReplay(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	reader := financialPrincipalStub{audience: activeCreditCustomer(77, "customer-ledger")}
	storage := &creditEvidenceStorageStub{}
	evidence, err := NewCreditEvidenceService(database, reader, managementCreditAuthorizerStub{}, storage)
	if err != nil {
		t.Fatal(err)
	}
	approval, err := NewCreditApprovalService(database)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = approval.ConfigureCreditPolicy(ctx, CreditPolicyInput{
		CurrencyCode: "CNY", MaximumLimitMinor: 50_000,
		DualReviewThresholdMinor: 5_000, BillingDay: 15,
		RepaymentPeriodDays: 10, GracePeriodDays: 3,
		DailyLateFeeBasisPoints: 10, MaximumLateFeeMinor: 5_000,
		ConfiguredBy: 101, IdempotencyKey: "credit-evidence-policy-0001",
	}); err != nil {
		t.Fatal(err)
	}
	usage, err := NewCreditUsageService(database)
	if err != nil {
		t.Fatal(err)
	}
	commands, err := NewAccountCreditCommandServiceWithEvidence(
		database, reader, &managementCommandAuditStub{}, &managementCommandRecentAuthStub{},
		approval, usage, evidence,
	)
	if err != nil {
		t.Fatal(err)
	}

	pdf := []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\n%%EOF")
	reservation, err := commands.ReserveCreditEvidence(ctx, "bank-statement.pdf", int64(len(pdf)))
	if err != nil || reservation.UploadNo == "" {
		t.Fatalf("reserve=%+v err=%v", reservation, err)
	}
	upload, err := commands.UploadCreditEvidence(ctx, reservation.UploadNo, "application/pdf", bytes.NewReader(pdf))
	if err != nil || upload.MediaType != "application/pdf" || len(storage.objects) != 1 {
		t.Fatalf("upload=%+v objects=%d err=%v", upload, len(storage.objects), err)
	}
	reference, evidenceDigest, err := evidence.PrepareAccountEvidence(ctx, []string{reservation.UploadNo})
	if err != nil || !strings.HasPrefix(reference, "private:credit/evidence/") || len(evidenceDigest) != 64 {
		t.Fatalf("prepare reference=%q digest=%q err=%v", reference, evidenceDigest, err)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	intent := AccountCreditApplicationIntent{
		CurrencyCode: "CNY", RequestedLimitMinor: 8_000,
		RequestedValidFrom: now, RequestedValidUntil: now.AddDate(1, 0, 0),
		UploadNos: []string{reservation.UploadNo}, IdempotencyKey: "credit-evidence-application-0001",
	}
	application, err := commands.SubmitCreditApplication(ctx, intent)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := commands.SubmitCreditApplication(ctx, intent)
	if err != nil || replayed.ApplicationNo != application.ApplicationNo {
		t.Fatalf("replay=%+v err=%v", replayed, err)
	}
	changed := intent
	changed.IdempotencyKey = "credit-evidence-application-0002"
	if result, changedErr := commands.SubmitCreditApplication(ctx, changed); result != nil || changedErr == nil {
		t.Fatalf("evidence was rebound to a different application: result=%+v err=%v", result, changedErr)
	}

	items, err := evidence.ListManagementEvidence(ctx, application.ApplicationNo)
	if err != nil || len(items) != 1 || items[0].UploadNo != reservation.UploadNo {
		t.Fatalf("items=%+v err=%v", items, err)
	}
	opened, err := evidence.OpenManagementEvidence(ctx, application.ApplicationNo, reservation.UploadNo)
	if err != nil || opened.MediaType != "application/pdf" {
		t.Fatalf("opened=%+v err=%v", opened, err)
	}
	content, readErr := io.ReadAll(opened.Body)
	_ = opened.Body.Close()
	if readErr != nil || !bytes.Equal(content, pdf) {
		t.Fatalf("opened content mismatch: %q err=%v", content, readErr)
	}
}

func TestCreditEvidenceRejectsUnsupportedOrSpoofedFiles(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name    string
		content []byte
	}{
		{name: "script.pdf", content: []byte("<script>alert(1)</script>")},
		{name: "evidence.exe", content: []byte("MZ")},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if mediaType, err := inspectCreditEvidence(testCase.name, testCase.content); err == nil || strings.TrimSpace(mediaType) != "" {
				t.Fatalf("mediaType=%q err=%v", mediaType, err)
			}
		})
	}
}
