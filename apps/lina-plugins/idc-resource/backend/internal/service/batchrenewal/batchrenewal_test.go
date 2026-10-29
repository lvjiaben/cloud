package batchrenewal

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
)

type fakeStore struct{ rows []Candidate }

func (s *fakeStore) List(context.Context, ListInput) (*CandidatePage, error) {
	return &CandidatePage{Items: s.rows}, nil
}
func (s *fakeStore) Load(_ context.Context, numbers []string) ([]Candidate, error) {
	wanted := map[string]bool{}
	for _, number := range numbers {
		wanted[number] = true
	}
	var result []Candidate
	for _, row := range s.rows {
		if wanted[row.ServerNo] {
			result = append(result, row)
		}
	}
	return result, nil
}

type fakeAuthorizer struct{ actor int }

func (a fakeAuthorizer) Authorize(context.Context, managementcap.AuthorizationRequest) (int, error) {
	return a.actor, nil
}

type permissionAuthorizer struct{ allowed map[string]bool }

func (a permissionAuthorizer) Authorize(_ context.Context, input managementcap.AuthorizationRequest) (int, error) {
	if a.allowed[input.Permission] {
		return 42, nil
	}
	return 0, errors.New("permission denied")
}

type fakeAudit struct{}

func (fakeAudit) AppendBusinessTransition(context.Context, idccap.BusinessAuditTransition) error {
	return nil
}
func (fakeAudit) AppendBusinessBatchTransition(context.Context, idccap.BusinessAuditBatchTransition) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, nil
}

type fakeRenewals struct {
	quoteFailures   map[string]bool
	confirmFailures map[string]bool
	confirms        []managementcap.StaffRenewalConfirmInput
	amountOffset    int64
}

func (f *fakeRenewals) QuoteForStaff(_ context.Context, input managementcap.StaffRenewalQuoteInput) (*managementcap.StaffRenewalQuote, error) {
	if f.quoteFailures[input.EntitlementNo] {
		return nil, errors.New("private core error")
	}
	now := time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)
	return &managementcap.StaffRenewalQuote{QuoteNo: "QUO_" + input.EntitlementNo, CurrencyCode: "CNY",
		RenewalCycles: input.RenewalCycles, OriginalExpiresAt: now,
		ProjectedExpiresAt: now.AddDate(0, input.RenewalCycles, 0), ExpiresAt: now.Add(time.Minute),
		TotalAmountMinor: int64(input.RenewalCycles)*1000 + f.amountOffset}, nil
}
func (f *fakeRenewals) ConfirmForStaff(_ context.Context, input managementcap.StaffRenewalConfirmInput) (*managementcap.StaffRenewalOperation, error) {
	f.confirms = append(f.confirms, input)
	if f.confirmFailures[input.QuoteNo] {
		return nil, errors.New("private confirmation error")
	}
	return &managementcap.StaffRenewalOperation{Status: "pending_payment"}, nil
}

func TestPreviewKeepsSelectionOrderAndSafeFailures(t *testing.T) {
	service, renewals := newTestService(t)
	preview, err := service.Preview(context.Background(), "request-key-001", 3, []string{serverB, serverA})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Items) != 2 || preview.Items[0].ServerNo != serverB || preview.Items[0].FailureCode != "quote_unavailable" {
		t.Fatalf("unexpected first item: %+v", preview.Items)
	}
	if !preview.Items[1].Eligible || preview.Items[1].TotalAmountMinor != 3000 || len(preview.ConfirmationDigest) != 64 {
		t.Fatalf("unexpected eligible preview: %+v", preview.Items[1])
	}
	if len(renewals.confirms) != 0 {
		t.Fatal("preview must not confirm")
	}
}

func TestConfirmContinuesAfterOneFailure(t *testing.T) {
	service, renewals := newTestService(t)
	renewals.quoteFailures = map[string]bool{}
	preview, err := service.Preview(context.Background(), "request-key-002", 6, []string{serverA, serverB})
	if err != nil {
		t.Fatal(err)
	}
	renewals.confirmFailures = map[string]bool{"QUO_ent_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb": true}
	confirmed, err := service.Confirm(context.Background(), "request-key-002", 6, []string{serverA, serverB}, preview.ConfirmationDigest)
	if err != nil {
		t.Fatal(err)
	}
	if len(renewals.confirms) != 2 || !confirmed.Items[0].Succeeded || confirmed.Items[1].FailureCode != "confirmation_failed" {
		t.Fatalf("independent results not preserved: %+v", confirmed.Items)
	}
}

func TestProductContextRejectsForeignPreviewAndConfirmation(t *testing.T) {
	service, renewals := newTestService(t)
	ctx := context.Background()
	product := "ent_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, err := service.Preview(ctx, "request-context-001", 1, []string{serverB}, product); err == nil {
		t.Fatal("foreign product preview accepted")
	}
	preview, err := service.Preview(ctx, "request-context-001", 1, []string{serverA}, product)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Confirm(ctx, "request-context-001", 1, []string{serverA}, preview.ConfirmationDigest, "ent_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"); err == nil {
		t.Fatal("foreign product confirmation accepted")
	}
	if len(renewals.confirms) != 0 {
		t.Fatal("cross-product context reached writes")
	}
	confirmed, err := service.Confirm(ctx, "request-context-001", 1, []string{serverA}, preview.ConfirmationDigest, product)
	if err != nil || len(confirmed.Items) != 1 || !confirmed.Items[0].Succeeded || len(renewals.confirms) != 1 {
		t.Fatalf("own product confirmation failed: %+v / %v", confirmed, err)
	}
}

func TestConfirmRejectsChangedProjectionBeforeAnyWrite(t *testing.T) {
	service, renewals := newTestService(t)
	preview, err := service.Preview(context.Background(), "request-key-003", 1, []string{serverA})
	if err != nil {
		t.Fatal(err)
	}
	service.(*serviceImpl).store.(*fakeStore).rows[0].ProjectionVersion++
	if _, err = service.Confirm(context.Background(), "request-key-003", 1, []string{serverA}, preview.ConfirmationDigest); err == nil {
		t.Fatal("changed projection must invalidate confirmation")
	}
	if len(renewals.confirms) != 0 {
		t.Fatal("conflict must be detected before writes")
	}
}

func TestConfirmRejectsChangedQuoteBeforeAnyWrite(t *testing.T) {
	service, renewals := newTestService(t)
	preview, err := service.Preview(context.Background(), "request-key-quote", 1, []string{serverA})
	if err != nil {
		t.Fatal(err)
	}
	renewals.amountOffset = 1
	if _, err = service.Confirm(context.Background(), "request-key-quote", 1, []string{serverA}, preview.ConfirmationDigest); err == nil {
		t.Fatal("changed amount must invalidate confirmation")
	}
	if len(renewals.confirms) != 0 {
		t.Fatal("changed quote must be detected before writes")
	}
}

func TestPreviewRejectsMoreThanTwentyServers(t *testing.T) {
	service, _ := newTestService(t)
	numbers := make([]string, 21)
	for index := range numbers {
		numbers[index] = fmt.Sprintf("SVR-%032X", index+1)
	}
	if _, err := service.Preview(context.Background(), "request-key-004", 1, numbers); err == nil {
		t.Fatal("batch must be bounded")
	}
}

func TestPreviewRequiresBothListAndSafePermissions(t *testing.T) {
	for _, allowed := range []map[string]bool{
		{PermissionView: true},
		{PermissionSafe: true},
	} {
		renewals := &fakeRenewals{quoteFailures: map[string]bool{}, confirmFailures: map[string]bool{}}
		service, err := NewWithStore(&fakeStore{rows: []Candidate{{
			ServerNo: serverA, EntitlementNo: "ent_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			CustomerNo: "CUS_A", ProductNameZhCN: "标准服务器", ProductNameEnUS: "Standard Server", ProjectionVersion: 3,
		}}}, managementcap.Runtime{Authorizer: permissionAuthorizer{allowed: allowed}, Audit: fakeAudit{}, Renewals: renewals})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = service.Preview(context.Background(), "request-key-permission", 1, []string{serverA}); err == nil {
			t.Fatalf("preview accepted incomplete permissions: %#v", allowed)
		}
		if len(renewals.confirms) != 0 {
			t.Fatal("permission rejection reached renewal writes")
		}
	}
}

const (
	serverA = "SVR-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	serverB = "SVR-BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"
)

func newTestService(t *testing.T) (Service, *fakeRenewals) {
	t.Helper()
	renewals := &fakeRenewals{quoteFailures: map[string]bool{"ent_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb": true}, confirmFailures: map[string]bool{}}
	store := &fakeStore{rows: []Candidate{
		{ServerNo: serverA, EntitlementNo: "ent_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", CustomerNo: "CUS_A", ProductNameZhCN: "标准服务器", ProductNameEnUS: "Standard Server", ProjectionVersion: 3},
		{ServerNo: serverB, EntitlementNo: "ent_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", CustomerNo: "CUS_B", ProductNameZhCN: "高性能服务器", ProductNameEnUS: "Performance Server", ProjectionVersion: 4},
	}}
	service, err := NewWithStore(store, managementcap.Runtime{Authorizer: fakeAuthorizer{actor: 42}, Audit: fakeAudit{}, Renewals: renewals})
	if err != nil {
		t.Fatal(err)
	}
	return service, renewals
}
