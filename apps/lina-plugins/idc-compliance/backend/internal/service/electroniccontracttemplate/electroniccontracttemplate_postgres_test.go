// This file proves template synchronization and local transitions against isolated PostgreSQL.

package electroniccontracttemplate

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/internal/localgatewayclient"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
)

const templatePostgreSQLDSNEnvironment = "LINAPRO_IDC_COMPLIANCE_TEST_DSN"

type templateAuthorizer struct {
	mu             sync.Mutex
	authorized     bool
	authorizations []managementcap.AuthorizationRequest
}

func (a *templateAuthorizer) Authorize(
	_ context.Context,
	request managementcap.AuthorizationRequest,
) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.authorizations = append(a.authorizations, request)
	if !a.authorized {
		return 0, errors.New("denied")
	}
	return 42, nil
}

type templateAudit struct {
	mu          sync.Mutex
	transitions []idccap.BusinessAuditTransition
}

func (a *templateAudit) AppendBusinessTransition(
	_ context.Context,
	transition idccap.BusinessAuditTransition,
) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.transitions = append(a.transitions, transition)
	return nil
}

func (*templateAudit) AppendBusinessBatchTransition(
	context.Context,
	idccap.BusinessAuditBatchTransition,
) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, nil
}

type templateGateway struct {
	mu       sync.Mutex
	snapshot localgatewayclient.TencentTemplateList
	err      error
	calls    int
}

func tencentTemplateFixture(ref, name, version, state, customerType string) localgatewayclient.TencentTemplate {
	customerOrder := int64(1)
	merchantOrder := int64(1)
	return localgatewayclient.TencentTemplate{
		TemplateRef: ref, Name: name, Version: version, ProviderState: state,
		CustomerSignatureCount: 1, MerchantSealCount: 1,
		Components: []localgatewayclient.TencentTemplateComponent{
			{ComponentRef: "contract-number", Name: "Contract number", TypeCode: "TEXT", Required: true},
			{ComponentRef: "service-details", Name: "Service details", TypeCode: "DYNAMIC_TABLE", Required: true},
			{ComponentRef: "customer-date", Name: "Customer signing date", TypeCode: "SIGN_DATE", Required: true, RecipientRef: "customer"},
			{ComponentRef: "merchant-date", Name: "Merchant signing date", TypeCode: "SIGN_DATE", Required: true, RecipientRef: "merchant"},
		},
		Recipients: []localgatewayclient.TencentTemplateRecipient{
			{RecipientRef: "customer", SequenceNo: 1, TypeCode: customerType, RoleName: "Customer", RequireSign: true, RoutingOrder: &customerOrder, SignComponentCount: 2},
			{RecipientRef: "merchant", SequenceNo: 2, TypeCode: ProviderRecipientPromoter, RoleName: "Merchant", RequireSign: true, RoutingOrder: &merchantOrder, IsPromoter: true, SignComponentCount: 2},
		},
	}
}

func saveTencentFixtureMapping(
	t *testing.T,
	service *serviceImpl,
	item *Item,
) *Item {
	t.Helper()
	detail, err := service.SaveProviderMapping(context.Background(), SaveProviderMappingInput{
		TemplateNo: item.TemplateNo, VersionNo: item.VersionNo,
		ExpectedVersion: item.LockVersion,
		IdempotencyKey:  "provider-mapping-" + item.TemplateNo + "-" + strconv.FormatInt(item.VersionNo, 10),
		FieldMappings: []ProviderFieldMapping{
			{ComponentRef: "contract-number", ValueCode: ProviderValueContractNo},
			{ComponentRef: "service-details", ValueCode: ProviderValueServiceLines},
			{ComponentRef: "customer-date", ValueCode: ProviderValueSigningDate},
			{ComponentRef: "merchant-date", ValueCode: ProviderValueSigningDate},
		},
		RecipientMappings: []ProviderRecipientMapping{
			{RecipientRef: "customer", PartyCode: PartyCustomer},
			{RecipientRef: "merchant", PartyCode: PartyMerchant},
		},
	})
	if err != nil || !detail.ProviderMappingSaved || !detail.EnableReady ||
		detail.StatusCode != StatusDisabled {
		t.Fatalf("save provider mapping detail=%+v err=%v", detail, err)
	}
	return &detail.Item
}

func (g *templateGateway) TencentTemplates(context.Context) (localgatewayclient.TencentTemplateList, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls++
	if g.err != nil {
		return localgatewayclient.TencentTemplateList{}, g.err
	}
	result := g.snapshot
	result.Items = append([]localgatewayclient.TencentTemplate(nil), g.snapshot.Items...)
	return result, nil
}

func TestElectronicContractTemplatePostgreSQLWorkflow(t *testing.T) {
	database, cleanup := newTemplateTestDatabase(t)
	defer cleanup()
	ctx := context.Background()
	authorizer := &templateAuthorizer{authorized: true}
	audit := &templateAudit{}
	gateway := &templateGateway{snapshot: localgatewayclient.TencentTemplateList{
		SyncedAt: time.Now().UTC().UnixMilli(),
		Items: []localgatewayclient.TencentTemplate{
			tencentTemplateFixture("provider-template-a", "Tencent A", "20260814001", ProviderStateActive, ProviderRecipientEnterprise),
			tencentTemplateFixture("provider-template-b", "Tencent B", "20260814001", ProviderStateDisabled, ProviderRecipientIndividual),
		},
	}}
	serviceValue, err := New(database, managementcap.Runtime{Authorizer: authorizer, Audit: audit}, gateway)
	if err != nil {
		t.Fatalf("construct template service: %v", err)
	}
	service := serviceValue.(*serviceImpl)
	service.now = func() time.Time { return time.Unix(1_786_665_600, 0).UTC() }

	created, err := service.Create(ctx, CreateInput{
		Name: "Local A", ScopeCode: ScopeEnterprise,
		SigningOrderCode: SigningOrderCustomerFirst,
		LanguageCode:     LanguageZhCN,
		IdempotencyKey:   "create-local-template-0001",
	})
	if err != nil || created.StatusCode != StatusDraft || created.OriginCode != OriginLocal {
		t.Fatalf("create local draft result=%+v err=%v", created, err)
	}
	replayed, err := service.Create(ctx, CreateInput{
		Name: "Local A", ScopeCode: ScopeEnterprise,
		SigningOrderCode: SigningOrderCustomerFirst,
		LanguageCode:     LanguageZhCN,
		IdempotencyKey:   "create-local-template-0001",
	})
	if err != nil || replayed.TemplateNo != created.TemplateNo {
		t.Fatalf("replay local draft result=%+v err=%v", replayed, err)
	}
	if _, err = service.Create(ctx, CreateInput{
		Name: "Changed", ScopeCode: ScopeEnterprise,
		SigningOrderCode: SigningOrderCustomerFirst,
		LanguageCode:     LanguageZhCN,
		IdempotencyKey:   "create-local-template-0001",
	}); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("changed create replay error=%v", err)
	}

	syncResult, err := service.SyncTencent(ctx)
	if err != nil || syncResult.ProviderTotal != 2 || syncResult.NewVersions != 2 {
		t.Fatalf("first sync result=%+v err=%v", syncResult, err)
	}
	page, err := service.List(ctx, ListInput{Page: 1})
	if err != nil || page.Total != 3 || len(page.Items) != 3 || page.PageSize != PageSize {
		t.Fatalf("list after sync result=%+v err=%v", page, err)
	}
	if gateway.calls != 1 {
		t.Fatalf("listing templates called Tencent implicitly: calls=%d", gateway.calls)
	}
	for _, pageNumber := range []int{1, 2, 3, 4, 100} {
		numbered, readErr := service.List(ctx, ListInput{Page: pageNumber, PageSize: 1})
		wantRows := 0
		if pageNumber <= 3 {
			wantRows = 1
		}
		if readErr != nil || numbered.Total != 3 || len(numbered.Items) != wantRows || numbered.PageSize != 1 {
			t.Fatalf("numbered template page %d: %+v err=%v", pageNumber, numbered, readErr)
		}
	}
	wide, readErr := service.List(ctx, ListInput{Page: 1, PageSize: 100})
	if readErr != nil || wide.Total != 3 || len(wide.Items) != 3 {
		t.Fatalf("wide template page: %+v err=%v", wide, readErr)
	}
	for index := 1; index < len(page.Items); index++ {
		if page.Items[index-1].UpdatedAt.Before(page.Items[index].UpdatedAt) {
			t.Fatalf("templates are not ordered by updated time descending: %+v", page.Items)
		}
	}
	namePage, err := service.List(ctx, ListInput{Name: "Tencent", Page: 1})
	if err != nil || namePage.Total != 2 || len(namePage.Items) != 2 {
		t.Fatalf("name filter result=%+v err=%v", namePage, err)
	}
	disabledPage, err := service.List(ctx, ListInput{StatusCode: StatusProviderDisabled, Page: 1})
	if err != nil || disabledPage.Total != 1 || len(disabledPage.Items) != 1 ||
		disabledPage.Items[0].Name != "Tencent B" {
		t.Fatalf("provider-disabled filter result=%+v err=%v", disabledPage, err)
	}
	if gateway.calls != 1 {
		t.Fatalf("filtered template lists called Tencent implicitly: calls=%d", gateway.calls)
	}
	providerA := requireTemplateByName(t, page.Items, "Tencent A")
	providerB := requireTemplateByName(t, page.Items, "Tencent B")
	if providerA.StatusCode != StatusNeedsConfiguration || providerA.ScopeCode != "" ||
		providerB.StatusCode != StatusProviderDisabled {
		t.Fatalf("unexpected synchronized states A=%+v B=%+v", providerA, providerB)
	}
	replayedSync, err := service.SyncTencent(ctx)
	if err != nil || replayedSync.NewVersions != 0 || replayedSync.MarkedOffline != 0 {
		t.Fatalf("unchanged provider snapshot created another version result=%+v err=%v", replayedSync, err)
	}
	replayedPage, err := service.List(ctx, ListInput{Page: 1})
	if err != nil {
		t.Fatalf("list after unchanged provider snapshot: %v", err)
	}
	replayedProviderA := requireTemplateByName(t, replayedPage.Items, "Tencent A")
	if replayedProviderA.TemplateNo != providerA.TemplateNo ||
		replayedProviderA.VersionNo != providerA.VersionNo {
		t.Fatalf("unchanged provider mapping moved from %+v to %+v", providerA, replayedProviderA)
	}

	providerAResult, err := service.Update(ctx, UpdateInput{
		TemplateNo: providerA.TemplateNo, VersionNo: providerA.VersionNo, Name: providerA.Name,
		ScopeCode: ScopeEnterprise, SigningOrderCode: SigningOrderParallel,
		LanguageCode:    LanguageZhCN,
		ExpectedVersion: providerA.LockVersion,
	})
	if err != nil || providerAResult.StatusCode != StatusNeedsConfiguration {
		t.Fatalf("complete provider settings result=%+v err=%v", providerAResult, err)
	}
	providerAResult = saveTencentFixtureMapping(t, service, providerAResult)
	providerAResult, err = service.Enable(ctx, StateInput{
		TemplateNo: providerAResult.TemplateNo, VersionNo: providerAResult.VersionNo,
		ExpectedVersion: providerAResult.LockVersion,
	})
	if err != nil || providerAResult.StatusCode != StatusEnabled {
		t.Fatalf("enable synchronized template result=%+v err=%v", providerAResult, err)
	}
	enterprisePage, err := service.List(ctx, ListInput{ScopeCode: ScopeEnterprise, Page: 1})
	if err != nil || enterprisePage.Total != 2 || len(enterprisePage.Items) != 2 {
		t.Fatalf("scope filter result=%+v err=%v", enterprisePage, err)
	}
	providerABeforeCopy := *providerAResult
	copied, err := service.Copy(ctx, CopyInput{
		SourceTemplateNo: providerAResult.TemplateNo,
		SourceVersionNo:  providerAResult.VersionNo,
		IdempotencyKey:   "copy-provider-template-0001",
	})
	if err != nil || copied.TemplateNo == providerAResult.TemplateNo || copied.VersionNo != 1 ||
		copied.OriginCode != OriginLocal || copied.StatusCode != StatusDraft ||
		copied.Name != providerAResult.Name || copied.ScopeCode != providerAResult.ScopeCode ||
		copied.SigningOrderCode != providerAResult.SigningOrderCode || copied.ProviderState != "" ||
		copied.ProviderVersion != "" || copied.UsedCount != 0 {
		t.Fatalf("copy did not create an isolated local draft result=%+v err=%v", copied, err)
	}
	replayedCopy, err := service.Copy(ctx, CopyInput{
		SourceTemplateNo: providerAResult.TemplateNo,
		SourceVersionNo:  providerAResult.VersionNo,
		IdempotencyKey:   "copy-provider-template-0001",
	})
	if err != nil || replayedCopy.TemplateNo != copied.TemplateNo {
		t.Fatalf("copy replay result=%+v err=%v", replayedCopy, err)
	}
	providerAAfterCopy, err := service.Get(ctx, providerAResult.TemplateNo, providerAResult.VersionNo)
	if err != nil || !reflect.DeepEqual(providerAAfterCopy, &providerABeforeCopy) {
		t.Fatalf("copy changed source before=%+v after=%+v err=%v", providerABeforeCopy, providerAAfterCopy, err)
	}
	copiedVersionCount, err := database.GetValue(ctx, `
SELECT count(*)
FROM plugin_idc_compliance_contract_template_version
WHERE template_no = ?`, copied.TemplateNo)
	if err != nil || copiedVersionCount.Int() != 1 {
		t.Fatalf("copied logical template version count=%d err=%v", copiedVersionCount.Int(), err)
	}
	providerBResult, err := service.Update(ctx, UpdateInput{
		TemplateNo: providerB.TemplateNo, VersionNo: providerB.VersionNo, Name: providerB.Name,
		ScopeCode: ScopePersonal, SigningOrderCode: SigningOrderCustomerFirst,
		LanguageCode:    LanguageEnUS,
		ExpectedVersion: providerB.LockVersion,
	})
	if err != nil || providerBResult.StatusCode != StatusProviderDisabled {
		t.Fatalf("complete disabled provider settings result=%+v err=%v", providerBResult, err)
	}
	if _, err = service.Enable(ctx, StateInput{
		TemplateNo: providerBResult.TemplateNo, VersionNo: providerBResult.VersionNo,
		ExpectedVersion: providerBResult.LockVersion,
	}); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("disabled Tencent template enable error=%v", err)
	}

	gateway.mu.Lock()
	gateway.snapshot.Items[0].Name = "Tencent A revised"
	gateway.snapshot.Items[0].Version = "20260814002"
	gateway.snapshot.Items[0].ProviderTimeMS = 1_786_665_601_000
	gateway.mu.Unlock()
	versionSync, err := service.SyncTencent(ctx)
	if err != nil || versionSync.NewVersions != 1 || versionSync.MarkedOffline != 0 {
		t.Fatalf("changed provider snapshot result=%+v err=%v", versionSync, err)
	}
	providerANext, err := service.Get(ctx, providerA.TemplateNo, providerA.VersionNo+1)
	if err != nil || providerANext.TemplateNo != providerA.TemplateNo ||
		providerANext.VersionNo != providerA.VersionNo+1 || providerANext.Name != "Tencent A revised" ||
		providerANext.ProviderVersion != "20260814002" || providerANext.StatusCode != StatusNeedsConfiguration {
		t.Fatalf("changed provider snapshot did not append an immutable local version result=%+v err=%v", providerANext, err)
	}
	providerAOld, err := service.Get(ctx, providerA.TemplateNo, providerA.VersionNo)
	if err != nil || providerAOld.StatusCode != StatusDisabled || providerAOld.Name != providerABeforeCopy.Name ||
		providerAOld.ProviderVersion != providerABeforeCopy.ProviderVersion {
		t.Fatalf("new provider mapping changed old version result=%+v err=%v", providerAOld, err)
	}
	providerAVersionCount, err := database.GetValue(ctx, `
SELECT count(*)
FROM plugin_idc_compliance_contract_template_version
WHERE template_no = ?`, providerA.TemplateNo)
	if err != nil || providerAVersionCount.Int() != 2 {
		t.Fatalf("provider logical template version count=%d err=%v", providerAVersionCount.Int(), err)
	}

	gateway.mu.Lock()
	gateway.snapshot.Items = gateway.snapshot.Items[:1]
	gateway.mu.Unlock()
	syncResult, err = service.SyncTencent(ctx)
	if err != nil || syncResult.MarkedOffline != 1 {
		t.Fatalf("offline sync result=%+v err=%v", syncResult, err)
	}
	providerBResult, err = service.Get(ctx, providerB.TemplateNo, providerB.VersionNo)
	if err != nil || providerBResult.StatusCode != StatusProviderOffline {
		t.Fatalf("offline provider projection=%+v err=%v", providerBResult, err)
	}
	offlinePage, err := service.List(ctx, ListInput{StatusCode: StatusProviderOffline, Page: 1})
	if err != nil || offlinePage.Total != 1 || len(offlinePage.Items) != 1 ||
		offlinePage.Items[0].TemplateNo != providerB.TemplateNo {
		t.Fatalf("provider-offline filter result=%+v err=%v", offlinePage, err)
	}

	beforeFailure, err := service.List(ctx, ListInput{})
	if err != nil {
		t.Fatalf("list before failed sync: %v", err)
	}
	gateway.mu.Lock()
	gateway.err = errors.New("provider unavailable")
	gateway.mu.Unlock()
	if _, err = service.SyncTencent(ctx); !bizerr.Is(err, CodeProviderUnavailable) {
		t.Fatalf("failed sync error=%v", err)
	}
	afterFailure, err := service.List(ctx, ListInput{})
	if err != nil || !reflect.DeepEqual(afterFailure, beforeFailure) {
		t.Fatalf("failed sync changed projection before=%+v after=%+v err=%v", beforeFailure, afterFailure, err)
	}

	authorizer.mu.Lock()
	authorizer.authorized = false
	authorizer.mu.Unlock()
	callsBeforeDeniedRequests := gateway.calls
	if _, err = service.List(ctx, ListInput{Page: 1}); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("list without query permission error=%v", err)
	}
	if _, err = service.SyncTencent(ctx); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("sync without management permission error=%v", err)
	}
	if gateway.calls != callsBeforeDeniedRequests {
		t.Fatalf("denied sync reached Tencent: before=%d after=%d", callsBeforeDeniedRequests, gateway.calls)
	}
	authorizer.mu.Lock()
	authorizations := append([]managementcap.AuthorizationRequest(nil), authorizer.authorizations...)
	authorizer.mu.Unlock()
	if !hasTemplateAuthorization(authorizations, PermissionQuery, templateDirectoryTarget) ||
		!hasTemplateAuthorization(authorizations, PermissionManage, templateDirectoryTarget) {
		t.Fatalf("query/manage permissions were not kept separate: %+v", authorizations)
	}

	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_contract (template_no, template_version)
VALUES (?, ?)`, providerA.TemplateNo, providerA.VersionNo); err != nil {
		t.Fatalf("insert contract template use: %v", err)
	}
	directoryValue, err := NewDirectorySource(database)
	if err != nil {
		t.Fatalf("construct template directory: %v", err)
	}
	locked, err := directoryValue.OrderContractExists(ctx, "ORDER_TEMPLATE_USE")
	if err != nil || !locked {
		t.Fatalf("order contract existence locked=%t err=%v", locked, err)
	}
	unlocked, err := directoryValue.OrderContractExists(ctx, "ORDER_WITHOUT_CONTRACT")
	if err != nil || unlocked {
		t.Fatalf("empty order contract existence locked=%t err=%v", unlocked, err)
	}
	if _, err = database.Exec(ctx, `
DELETE FROM plugin_idc_compliance_contract_template_version
WHERE template_no = ? AND version_no = ?`, providerA.TemplateNo, providerA.VersionNo); err == nil {
		t.Fatal("used template version was deletable")
	}
	if len(audit.transitions) < 6 {
		t.Fatalf("audit transition count=%d", len(audit.transitions))
	}
}

func TestElectronicContractTemplateClosedSettingsVocabulary(t *testing.T) {
	acceptedScopes := []string{ScopePersonal, ScopeEnterprise, ScopeAllCustomers}
	acceptedOrders := []string{
		SigningOrderCustomerFirst,
		SigningOrderMerchantFirst,
		SigningOrderParallel,
	}
	acceptedLanguages := []string{LanguageZhCN, LanguageEnUS}
	for _, scope := range acceptedScopes {
		if !validScope(scope) {
			t.Fatalf("configured scope was rejected: %q", scope)
		}
		for _, order := range acceptedOrders {
			for _, language := range acceptedLanguages {
				if !validRequiredSettings(scope, order, language) {
					t.Fatalf(
						"configured scope/order/language settings were rejected: %q/%q/%q",
						scope, order, language,
					)
				}
			}
		}
	}
	for _, scope := range []string{"", "individual", "company", "all"} {
		if validScope(scope) {
			t.Fatalf("unexpected scope was accepted: %q", scope)
		}
	}
	for _, order := range []string{"", "sequential", "merchant_and_customer", "any"} {
		if validSigningOrder(order) {
			t.Fatalf("unexpected signing order was accepted: %q", order)
		}
	}
	for _, language := range []string{"", "zh", "en", "zh-TW"} {
		if validLanguage(language) {
			t.Fatalf("unexpected contract language was accepted: %q", language)
		}
	}
}

func hasTemplateAuthorization(
	authorizations []managementcap.AuthorizationRequest,
	permission string,
	target string,
) bool {
	for _, authorization := range authorizations {
		if authorization.Permission == permission && authorization.TargetNo == target {
			return true
		}
	}
	return false
}

func TestElectronicContractTemplatePostgreSQLConcurrentEnableKeepsOneVersion(t *testing.T) {
	database, cleanup := newTemplateTestDatabase(t)
	defer cleanup()
	ctx := context.Background()
	authorizer := &templateAuthorizer{authorized: true}
	audit := &templateAudit{}
	gateway := &templateGateway{snapshot: localgatewayclient.TencentTemplateList{
		SyncedAt: time.Now().UTC().UnixMilli(),
		Items: []localgatewayclient.TencentTemplate{
			tencentTemplateFixture("provider-template-concurrent", "Concurrent", "1", ProviderStateActive, ProviderRecipientEnterprise),
		},
	}}
	serviceValue, err := New(database, managementcap.Runtime{Authorizer: authorizer, Audit: audit}, gateway)
	if err != nil {
		t.Fatal(err)
	}
	service := serviceValue.(*serviceImpl)
	if _, err = service.SyncTencent(ctx); err != nil {
		t.Fatal(err)
	}
	firstPage, err := service.List(ctx, ListInput{})
	if err != nil || len(firstPage.Items) != 1 {
		t.Fatalf("first page=%+v err=%v", firstPage, err)
	}
	first, err := service.Update(ctx, UpdateInput{
		TemplateNo: firstPage.Items[0].TemplateNo, VersionNo: 1, Name: "Concurrent",
		ScopeCode: ScopeEnterprise, SigningOrderCode: SigningOrderParallel,
		LanguageCode:    LanguageZhCN,
		ExpectedVersion: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	first = saveTencentFixtureMapping(t, service, first)
	gateway.mu.Lock()
	gateway.snapshot.Items[0].Version = "2"
	gateway.mu.Unlock()
	if _, err = service.SyncTencent(ctx); err != nil {
		t.Fatal(err)
	}
	second, err := service.Get(ctx, first.TemplateNo, 2)
	if err != nil {
		t.Fatal(err)
	}
	second, err = service.Update(ctx, UpdateInput{
		TemplateNo: second.TemplateNo, VersionNo: second.VersionNo, Name: second.Name,
		ScopeCode: ScopeEnterprise, SigningOrderCode: SigningOrderParallel,
		LanguageCode:    LanguageZhCN,
		ExpectedVersion: second.LockVersion,
	})
	if err != nil {
		t.Fatal(err)
	}
	second = saveTencentFixtureMapping(t, service, second)
	authorizer.mu.Lock()
	authorizer.authorized = false
	authorizer.mu.Unlock()
	if _, err = service.Enable(ctx, StateInput{
		TemplateNo: first.TemplateNo, VersionNo: first.VersionNo, ExpectedVersion: first.LockVersion,
	}); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("enable without management permission error=%v", err)
	}
	_, err = service.Get(ctx, first.TemplateNo, first.VersionNo)
	if !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("detail without query permission error=%v", err)
	}
	authorizer.mu.Lock()
	authorizer.authorized = true
	authorizer.mu.Unlock()
	firstAfterDenied, err := service.Get(ctx, first.TemplateNo, first.VersionNo)
	if err != nil || firstAfterDenied.StatusCode != StatusDisabled || firstAfterDenied.LockVersion != first.LockVersion {
		t.Fatalf("denied enable changed version result=%+v err=%v", firstAfterDenied, err)
	}

	firstEnabled, err := service.Enable(ctx, StateInput{
		TemplateNo: first.TemplateNo, VersionNo: first.VersionNo, ExpectedVersion: first.LockVersion,
	})
	if err != nil || firstEnabled.StatusCode != StatusEnabled {
		t.Fatalf("enable first version result=%+v err=%v", firstEnabled, err)
	}
	secondEnabled, err := service.Enable(ctx, StateInput{
		TemplateNo: second.TemplateNo, VersionNo: second.VersionNo, ExpectedVersion: second.LockVersion,
	})
	if err != nil || secondEnabled.StatusCode != StatusEnabled {
		t.Fatalf("enable replacement version result=%+v err=%v", secondEnabled, err)
	}
	firstAutomaticallyDisabled, err := service.Get(ctx, first.TemplateNo, first.VersionNo)
	if err != nil || firstAutomaticallyDisabled.StatusCode != StatusDisabled ||
		firstAutomaticallyDisabled.LockVersion != firstEnabled.LockVersion+1 {
		t.Fatalf("replacement did not atomically disable old version result=%+v err=%v", firstAutomaticallyDisabled, err)
	}
	secondDisabled, err := service.Disable(ctx, StateInput{
		TemplateNo: secondEnabled.TemplateNo, VersionNo: secondEnabled.VersionNo,
		ExpectedVersion: secondEnabled.LockVersion,
	})
	if err != nil || secondDisabled.StatusCode != StatusDisabled {
		t.Fatalf("prepare concurrent enable state result=%+v err=%v", secondDisabled, err)
	}

	inputs := []StateInput{
		{
			TemplateNo: firstAutomaticallyDisabled.TemplateNo, VersionNo: firstAutomaticallyDisabled.VersionNo,
			ExpectedVersion: firstAutomaticallyDisabled.LockVersion,
		},
		{
			TemplateNo: secondDisabled.TemplateNo, VersionNo: secondDisabled.VersionNo,
			ExpectedVersion: secondDisabled.LockVersion,
		},
	}
	var wait sync.WaitGroup
	results := make(chan error, len(inputs))
	wait.Add(len(inputs))
	for _, input := range inputs {
		input := input
		go func() {
			defer wait.Done()
			_, enableErr := service.Enable(ctx, input)
			results <- enableErr
		}()
	}
	wait.Wait()
	close(results)
	for enableErr := range results {
		if enableErr != nil {
			t.Fatalf("concurrent enable failed: %v", enableErr)
		}
	}
	count, err := database.GetValue(ctx, `
SELECT count(*)
FROM plugin_idc_compliance_contract_template_version
WHERE template_no = ? AND status_code = 'enabled'`, first.TemplateNo)
	if err != nil || count.Int() != 1 {
		t.Fatalf("enabled count=%d err=%v", count.Int(), err)
	}
	if _, err = service.Enable(ctx, inputs[0]); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("stale enable was not rejected: %v", err)
	}
	audit.mu.Lock()
	transitions := append([]idccap.BusinessAuditTransition(nil), audit.transitions...)
	audit.mu.Unlock()
	enableAudits := 0
	disableAudits := 0
	for _, transition := range transitions {
		switch transition.OperationCode {
		case "compliance.contract-template.enable":
			enableAudits++
		case "compliance.contract-template.disable":
			disableAudits++
		default:
			continue
		}
		if transition.TargetNo != first.TemplateNo || transition.ResultCode != "succeeded" ||
			transition.RequiredPermission != PermissionManage || transition.BeforeSnapshot == nil ||
			transition.AfterSnapshot == nil {
			t.Fatalf("unsafe template state audit: %+v", transition)
		}
	}
	if enableAudits != 4 || disableAudits != 1 {
		t.Fatalf("state audit counts enable=%d disable=%d transitions=%+v", enableAudits, disableAudits, transitions)
	}
}

func requireTemplateByName(t *testing.T, items []Item, name string) Item {
	t.Helper()
	for _, item := range items {
		if item.Name == name {
			return item
		}
	}
	t.Fatalf("template %q missing from %+v", name, items)
	return Item{}
}

func newTemplateTestDatabase(t *testing.T) (gdb.DB, func()) {
	t.Helper()
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(templatePostgreSQLDSNEnvironment))
	if dsn == "" {
		t.Skipf("set %s to run template PostgreSQL tests", templatePostgreSQLDSNEnvironment)
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("open template test admin database: %v", err)
	}
	var random [8]byte
	if _, err = rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	schema := "idc_contract_template_test_" + hex.EncodeToString(random[:])
	quoted := `"` + schema + `"`
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	scopedDSN := dsn + "?search_path=" + schema
	if strings.Contains(dsn, "?") {
		scopedDSN = dsn + "&search_path=" + schema
	}
	database, err := gdb.New(gdb.ConfigNode{Link: scopedDSN})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `
CREATE TABLE plugin_idc_compliance_contract (
    contract_no BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0,
    order_no VARCHAR(40) NOT NULL DEFAULT 'ORDER_TEMPLATE_USE',
    template_no VARCHAR(64) NOT NULL,
    template_version BIGINT NOT NULL
)`); err != nil {
		t.Fatal(err)
	}
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve template migration path")
	}
	migrationDirectory := filepath.Join(
		filepath.Dir(sourceFile), "..", "..", "..", "..", "manifest", "sql",
	)
	for _, migrationName := range []string{
		"009-electronic-contract-template.sql",
		"011-electronic-contract-provider-mapping.sql",
	} {
		migration, readErr := os.ReadFile(filepath.Join(migrationDirectory, migrationName))
		if readErr != nil {
			t.Fatalf("read template migration %s: %v", migrationName, readErr)
		}
		if _, err = database.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("apply template migration %s: %v", migrationName, err)
		}
	}
	cleanup := func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Errorf("close scoped template database: %v", closeErr)
		}
		if !strings.HasPrefix(schema, "idc_contract_template_test_") {
			t.Errorf("refusing unexpected schema %q", schema)
		} else if _, dropErr := admin.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE"); dropErr != nil {
			t.Errorf("drop template test schema: %v", dropErr)
		}
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Errorf("close template admin database: %v", closeErr)
		}
	}
	return database, cleanup
}
