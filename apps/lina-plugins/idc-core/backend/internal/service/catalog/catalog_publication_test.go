package catalog

import (
	"context"
	"errors"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

type publicationTestAuthorizer struct {
	actor int
	err   error
	calls int
}

func (a *publicationTestAuthorizer) AuthorizeProductPublication(context.Context, string) (int, error) {
	a.calls++
	return a.actor, a.err
}

type publicationTestAuditWriter struct {
	transition *idccap.BusinessAuditTransition
}

func (a *publicationTestAuditWriter) AppendBusinessTransition(_ context.Context, transition idccap.BusinessAuditTransition) error {
	a.transition = &transition
	return nil
}

// Staff publication must carry the permission and both immutable snapshots
// required by the real audit writer, rather than relying on a permissive fake.
func TestPublicationAuditCarriesStaffAuthorization(t *testing.T) {
	audit := &publicationTestAuditWriter{}
	service := &publicationServiceImpl{audit: audit}
	err := service.appendPublicationAudit(context.Background(), &entity.CatalogProductVersion{
		VersionNo: "ver_11111111111111111111111111111111", ProductNo: "prd_11111111111111111111111111111111",
		StatusCode: catalogStatusDraft, SourceCode: catalogSourceManual,
	}, &PublicationResult{})
	if err != nil || audit.transition == nil {
		t.Fatalf("publication audit missing: %v", err)
	}
	if audit.transition.RequiredPermission != PermissionCatalogProductPublish || audit.transition.BeforeSnapshot == nil || audit.transition.AfterSnapshot == nil {
		t.Fatalf("publication audit cannot authorize staff or freeze states: %+v", audit.transition)
	}
}

func (*publicationTestAuditWriter) AppendBusinessBatchTransition(context.Context, idccap.BusinessAuditBatchTransition) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, nil
}

type publicationTestAutomationAdmission struct {
	err    error
	calls  int
	action bool
}

func (g *publicationTestAutomationAdmission) WithPublicationAdmission(
	ctx context.Context,
	_ string,
	action func(context.Context) error,
) error {
	g.calls++
	if g.err != nil {
		return g.err
	}
	g.action = true
	return action(ctx)
}

func TestPublicationAuthorizesBeforeAutomationAdmission(t *testing.T) {
	authorizationErr := bizerr.NewCode(CodePublicationNotAccessible)
	authorizer := &publicationTestAuthorizer{err: authorizationErr}
	automation := &publicationTestAutomationAdmission{err: errors.New("must not run")}
	service, err := NewPublication(
		&publicationTestAuditWriter{}, authorizer, automation,
	)
	if err != nil {
		t.Fatalf("construct publication: %v", err)
	}
	if _, err = service.PublishVersion(context.Background(), "invalid target"); !bizerr.Is(err, CodePublicationNotAccessible) {
		t.Fatalf("authorization error=%v", err)
	}
	if authorizer.calls != 1 || automation.calls != 0 {
		t.Fatalf("authorizer calls=%d automation calls=%d", authorizer.calls, automation.calls)
	}
}

func TestPublicationHidesAutomationManagementAndInfrastructureErrors(t *testing.T) {
	tests := []struct {
		name string
		gate error
		want *bizerr.Code
	}{
		{name: "stopped or changed evidence", gate: bizerr.NewCode(CodeManagementConflict), want: CodePublicationIncomplete},
		{name: "binding not accessible", gate: bizerr.NewCode(CodeManagementNotAccessible), want: CodePublicationIncomplete},
		{name: "host unavailable", gate: errors.New("secret database or host failure"), want: CodePublicationUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			automation := &publicationTestAutomationAdmission{err: test.gate}
			service, err := NewPublication(
				&publicationTestAuditWriter{}, &publicationTestAuthorizer{actor: 7},
				automation,
			)
			if err != nil {
				t.Fatalf("construct publication: %v", err)
			}
			_, err = service.PublishVersion(context.Background(), "ver_0123456789abcdef0123456789abcdef")
			if !bizerr.Is(err, test.want) {
				t.Fatalf("publication error=%v want=%s", err, test.want.RuntimeCode())
			}
			if bizerr.Is(err, CodeManagementConflict) || bizerr.Is(err, CodeManagementNotAccessible) {
				t.Fatalf("publication leaked gate error: %v", err)
			}
			if automation.action {
				t.Fatal("publication action ran after gate rejection")
			}
		})
	}
}

func stringPointer(value string) *string { return &value }

func TestPublicationSalesCopyIsIndependentOfProductKind(t *testing.T) {
	for _, product := range []*entity.CatalogProduct{
		{NameZhCn: "SSL 证书", NameEnUs: "SSL Certificate", SummaryZhCn: "证书服务", SummaryEnUs: "Certificate service"},
		{NameZhCn: "手机", NameEnUs: "Phone", SummaryZhCn: "实物商品", SummaryEnUs: "Physical goods"},
		{NameZhCn: "人工服务", NameEnUs: "Manual Service", SummaryZhCn: "人工交付", SummaryEnUs: "Manual delivery"},
	} {
		if !completePublicationProductCopy(product) {
			t.Fatalf("generic sales copy rejected: %+v", product)
		}
	}
}

func TestPublicationChecksContainOnlyGenericCommercialDimensions(t *testing.T) {
	want := []PublicationCheckCode{PublicationCheckSalesCopy, PublicationCheckPricing, PublicationCheckTax}
	checks, err := evaluateCatalogPublicationCompleteness(context.Background(), nil, nil)
	if err != nil || len(checks) != len(want) {
		t.Fatalf("checks=%+v err=%v", checks, err)
	}
	for i := range want {
		if checks[i].Code != want[i] {
			t.Fatalf("check %d=%q want %q", i, checks[i].Code, want[i])
		}
	}
}
