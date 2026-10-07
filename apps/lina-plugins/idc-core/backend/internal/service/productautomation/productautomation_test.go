// This file unit-tests authorization ordering, bounded discovery, and safe projections.

package productautomation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

type automationTestAuthorizer struct {
	err   error
	calls []string
}

func (a *automationTestAuthorizer) authorize(action string) (Authorization, error) {
	a.calls = append(a.calls, action)
	if a.err != nil {
		return Authorization{}, a.err
	}
	return Authorization{ActorUserID: 101, RequiredPermission: "server-side-test-permission"}, nil
}

func (a *automationTestAuthorizer) AuthorizeProviderCandidates(context.Context) (Authorization, error) {
	return a.authorize("provider-candidates")
}
func (a *automationTestAuthorizer) AuthorizeInterfaceList(context.Context) (Authorization, error) {
	return a.authorize("interface-list")
}
func (a *automationTestAuthorizer) AuthorizeInterfaceCreate(context.Context) (Authorization, error) {
	return a.authorize("interface-create")
}
func (a *automationTestAuthorizer) AuthorizeInterfaceRename(context.Context, string) (Authorization, error) {
	return a.authorize("interface-rename")
}
func (a *automationTestAuthorizer) AuthorizeInterfaceDelete(context.Context, string) (Authorization, error) {
	return a.authorize("interface-delete")
}
func (a *automationTestAuthorizer) AuthorizeInterfaceStop(context.Context, string) (Authorization, error) {
	return a.authorize("interface-stop")
}
func (a *automationTestAuthorizer) AuthorizeInterfaceResume(context.Context, string) (Authorization, error) {
	return a.authorize("interface-resume")
}
func (a *automationTestAuthorizer) AuthorizeInterfaceConfiguration(context.Context, string) (Authorization, error) {
	return a.authorize("interface-configuration")
}
func (a *automationTestAuthorizer) AuthorizeBindingRead(context.Context, string) (Authorization, error) {
	return a.authorize("binding-read")
}
func (a *automationTestAuthorizer) AuthorizeBindingWrite(context.Context, string) (Authorization, error) {
	return a.authorize("binding-write")
}
func (a *automationTestAuthorizer) AuthorizeVersionConfiguration(context.Context, string) (Authorization, error) {
	return a.authorize("version-configuration")
}
func (a *automationTestAuthorizer) AuthorizePublicationAdmission(context.Context, string) (Authorization, error) {
	return a.authorize("publication-admission")
}

type automationTestExtensions struct {
	evidence        []*capmodel.BusinessExtensionProviderEvidence
	listCalls       int
	exactCalls      int
	exactBatchCalls int
	admissionCalls  int
	admissionErr    error
	listErr         error
	exactErr        error
	lastAdmission   pluginhost.BusinessExtensionAdmissionInput
	invokeCalls     int
	invocations     []pluginhost.BusinessExtensionMethodInvokeInput
	invoke          func(pluginhost.BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error)
}

type automationLockedContextKey struct{}

func (e *automationTestExtensions) ListProviders(context.Context, plugincap.BusinessExtensionProviderListInput) (*capmodel.PageResult[*capmodel.BusinessExtensionProviderEvidence], error) {
	e.listCalls++
	if e.listErr != nil {
		return nil, e.listErr
	}
	return &capmodel.PageResult[*capmodel.BusinessExtensionProviderEvidence]{Items: e.evidence, Total: len(e.evidence)}, nil
}

func (e *automationTestExtensions) GetProviderExact(context.Context, plugincap.BusinessExtensionProviderExactInput) (*capmodel.BusinessExtensionProviderEvidence, error) {
	e.exactCalls++
	if e.exactErr != nil || len(e.evidence) == 0 {
		return nil, e.exactErr
	}
	return cloneAutomationEvidence(e.evidence[0]), nil
}

func (e *automationTestExtensions) GetProvidersExact(_ context.Context, inputs []plugincap.BusinessExtensionProviderExactInput) ([]*capmodel.BusinessExtensionProviderEvidence, error) {
	e.exactBatchCalls++
	if e.exactErr != nil {
		return nil, e.exactErr
	}
	result := make([]*capmodel.BusinessExtensionProviderEvidence, len(inputs))
	for index, input := range inputs {
		for _, evidence := range e.evidence {
			if evidence != nil && evidence.PluginID == input.PluginID && evidence.ReleaseVersion == input.ReleaseVersion && evidence.Point == input.Point && evidence.ContractVersion == input.ContractVersion && evidence.ReleaseDigest == input.ReleaseDigest && evidence.DeclarationDigest == input.DeclarationDigest {
				result[index] = cloneAutomationEvidence(evidence)
				break
			}
		}
	}
	return result, nil
}

func (e *automationTestExtensions) WithNewBusinessAdmission(ctx context.Context, input pluginhost.BusinessExtensionAdmissionInput, action capmodel.BusinessExtensionAdmissionAction) error {
	e.admissionCalls++
	e.lastAdmission = input
	if e.admissionErr != nil {
		return e.admissionErr
	}
	if len(e.evidence) == 0 {
		return errors.New("no evidence")
	}
	return action(context.WithValue(ctx, automationLockedContextKey{}, true), cloneAutomationEvidence(e.evidence[0]))
}

func (*automationTestExtensions) WithConfidentialOutboundContext(
	context.Context,
	pluginhost.ConfidentialOutboundContextInput,
	pluginhost.ConfidentialOutboundAction,
) error {
	return errors.New("confidential outbound context is outside product automation tests")
}

func (e *automationTestExtensions) InvokeProviderMethod(
	_ context.Context,
	input pluginhost.BusinessExtensionMethodInvokeInput,
) (*capmodel.BusinessExtensionMethodResult, error) {
	e.invokeCalls++
	e.invocations = append(e.invocations, input)
	if e.invoke != nil {
		return e.invoke(input)
	}
	return nil, errors.New("provider method invocation is not configured in this test")
}

type automationTestCodec struct {
	evidence  *capmodel.BusinessExtensionProviderEvidence
	encodes   int
	decodes   int
	decodeErr error
}

func (c *automationTestCodec) EncodeProviderSelection(context.Context, *capmodel.BusinessExtensionProviderEvidence) (string, error) {
	c.encodes++
	return "opaque-purpose-bound-token", nil
}

func (c *automationTestCodec) DecodeProviderSelection(context.Context, string) (*capmodel.BusinessExtensionProviderEvidence, error) {
	c.decodes++
	return cloneAutomationEvidence(c.evidence), c.decodeErr
}

type automationTestAudit struct {
	mu          sync.Mutex
	transitions []idccap.BusinessAuditTransition
	err         error
	strict      bool
	seen        map[string]idccap.BusinessAuditTransition
	database    gdb.DB
}

func (a *automationTestAudit) AppendBusinessTransition(ctx context.Context, input idccap.BusinessAuditTransition) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.strict {
		key := input.IdempotencyScope + ":" + input.IdempotencyKey
		if a.seen == nil {
			a.seen = make(map[string]idccap.BusinessAuditTransition)
		}
		if previous, exists := a.seen[key]; exists && !reflect.DeepEqual(previous, input) {
			return errors.New("audit idempotency evidence conflict")
		}
		a.seen[key] = input
	}
	if a.database != nil {
		encoded, encodeErr := json.Marshal(input)
		if encodeErr != nil {
			return encodeErr
		}
		digest := fmt.Sprintf("%x", sha256.Sum256(encoded))
		if _, insertErr := a.database.Exec(ctx, `
INSERT INTO product_automation_test_audit (idempotency_scope,idempotency_key,evidence_digest)
VALUES (?,?,?) ON CONFLICT (idempotency_scope,idempotency_key) DO NOTHING`, input.IdempotencyScope, input.IdempotencyKey, digest); insertErr != nil {
			return insertErr
		}
		stored, loadErr := a.database.GetValue(ctx, `
SELECT evidence_digest FROM product_automation_test_audit
WHERE idempotency_scope=? AND idempotency_key=?`, input.IdempotencyScope, input.IdempotencyKey)
		if loadErr != nil {
			return loadErr
		}
		if stored.String() != digest {
			return errors.New("audit idempotency evidence conflict")
		}
	}
	a.transitions = append(a.transitions, input)
	return a.err
}

func (a *automationTestAudit) snapshotTransitions() []idccap.BusinessAuditTransition {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]idccap.BusinessAuditTransition(nil), a.transitions...)
}

func (*automationTestAudit) AppendBusinessBatchTransition(context.Context, idccap.BusinessAuditBatchTransition) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, nil
}

func TestListProviderCandidatesIsAuthorizedBoundedAndSafe(t *testing.T) {
	ctx := context.Background()
	evidence := automationEvidence(nil)
	authorizer := &automationTestAuthorizer{}
	extensions := &automationTestExtensions{evidence: []*capmodel.BusinessExtensionProviderEvidence{evidence}}
	codec := &automationTestCodec{evidence: evidence}
	service := newAutomationUnitService(t, extensions, authorizer, codec)

	page, err := service.ListProviderCandidates(ctx, PageInput{Page: 1, PageSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	if extensions.listCalls != 1 || codec.encodes != 1 || len(page.Items) != 1 || page.Total != 1 {
		t.Fatalf("unexpected bounded discovery calls=%d encodes=%d page=%+v", extensions.listCalls, codec.encodes, page)
	}
	if page.Items[0].FulfillmentCapability == nil || page.Items[0].FulfillmentCapability.ZhCN != "自动开通" {
		t.Fatalf("safe candidate omitted the user-readable fulfillment capability: %+v", page.Items[0])
	}
	projection := fmt.Sprintf("%+v", page.Items[0])
	for _, forbidden := range []string{evidence.PluginID, evidence.ReleaseDigest, evidence.DeclarationDigest, "provision"} {
		if strings.Contains(projection, forbidden) {
			t.Fatalf("safe candidate leaked %q: %s", forbidden, projection)
		}
	}
	if _, err = service.ListProviderCandidates(ctx, PageInput{Page: 1, PageSize: maximumPageSize + 1}); err == nil {
		t.Fatal("expected oversized provider page to be rejected")
	}
	if extensions.listCalls != 1 {
		t.Fatalf("invalid page reached host discovery: calls=%d", extensions.listCalls)
	}
}

func TestProviderDiscoveryAuthorizationRunsBeforeHost(t *testing.T) {
	denied := errors.New("denied")
	authorizer := &automationTestAuthorizer{err: denied}
	extensions := &automationTestExtensions{evidence: []*capmodel.BusinessExtensionProviderEvidence{automationEvidence(nil)}}
	service := newAutomationUnitService(t, extensions, authorizer, &automationTestCodec{})
	if _, err := service.ListProviderCandidates(context.Background(), PageInput{}); !errors.Is(err, denied) {
		t.Fatalf("expected authorization error, got %v", err)
	}
	if extensions.listCalls != 0 {
		t.Fatal("host was called before authorization succeeded")
	}
}

func TestAllOperationsAuthorizeBeforeDependencies(t *testing.T) {
	operations := []struct {
		name string
		run  func(Service) error
	}{
		{name: "provider candidates", run: func(service Service) error {
			_, err := service.ListProviderCandidates(context.Background(), PageInput{})
			return err
		}},
		{name: "interface list", run: func(service Service) error {
			_, err := service.ListInterfaces(context.Background(), PageInput{})
			return err
		}},
		{name: "interface create", run: func(service Service) error {
			_, err := service.CreateInterface(context.Background(), CreateInterfaceInput{})
			return err
		}},
		{name: "interface stop", run: func(service Service) error { return service.StopNewBusiness(context.Background(), "bad") }},
		{name: "interface resume", run: func(service Service) error { return service.ResumeNewBusiness(context.Background(), "bad") }},
		{name: "interface configuration read", run: func(service Service) error {
			_, err := service.GetInterfaceConfiguration(context.Background(), "bad")
			return err
		}},
		{name: "interface configuration evaluate", run: func(service Service) error {
			_, err := service.EvaluateInterfaceConfiguration(context.Background(), EvaluateInterfaceConfigurationInput{})
			return err
		}},
		{name: "interface configuration write", run: func(service Service) error {
			_, err := service.UpdateInterfaceConfiguration(context.Background(), UpdateInterfaceConfigurationInput{})
			return err
		}},
		{name: "binding read", run: func(service Service) error { _, err := service.GetBinding(context.Background(), "bad"); return err }},
		{name: "fulfillment contract read", run: func(service Service) error {
			_, err := service.GetFrozenFulfillmentContract(context.Background(), "bad")
			return err
		}},
		{name: "fulfillment policy write admission", run: func(service Service) error {
			_, err := service.AuthorizeFulfillmentPolicyWrite(context.Background(), "bad")
			return err
		}},
		{name: "binding write", run: func(service Service) error { _, err := service.Bind(context.Background(), BindInput{}); return err }},
		{name: "binding remove", run: func(service Service) error { return service.Unbind(context.Background(), "bad") }},
		{name: "version configuration read", run: func(service Service) error {
			_, err := service.GetVersionConfiguration(context.Background(), "bad")
			return err
		}},
		{name: "version configuration evaluate", run: func(service Service) error {
			_, err := service.EvaluateVersionConfiguration(context.Background(), EvaluateVersionConfigurationInput{})
			return err
		}},
		{name: "publication readiness", run: func(service Service) error {
			_, err := service.CheckPublicationReadiness(context.Background(), "bad")
			return err
		}},
		{name: "publication", run: func(service Service) error { return service.WithPublicationAdmission(context.Background(), "bad", nil) }},
	}
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			denied := errors.New("denied")
			evidence := automationEvidence(nil)
			authorizer := &automationTestAuthorizer{err: denied}
			extensions := &automationTestExtensions{evidence: []*capmodel.BusinessExtensionProviderEvidence{evidence}}
			codec := &automationTestCodec{evidence: evidence}
			service := newAutomationUnitService(t, extensions, authorizer, codec)
			if err := operation.run(service); !errors.Is(err, denied) {
				t.Fatalf("expected authorization error, got %v", err)
			}
			if extensions.listCalls != 0 || extensions.exactCalls != 0 || extensions.admissionCalls != 0 || codec.encodes != 0 || codec.decodes != 0 {
				t.Fatalf("dependency called before authorization: extensions=%+v codec=%+v", extensions, codec)
			}
		})
	}
}

func TestProviderEvidenceRejectsMissingFulfillmentMethods(t *testing.T) {
	nilMethods := automationEvidence([]string{})
	nilMethods.AuthorizedMethods = nil
	if validProviderEvidence(nilMethods) {
		t.Fatal("nil authorized methods must not be accepted as the canonical empty method set")
	}
	emptyMethods := automationEvidence([]string{})
	if validProviderEvidence(emptyMethods) {
		t.Fatal("product fulfillment evidence without role methods must be rejected")
	}
}

func TestProviderEvidenceRequiresMultipleCardinalityForInterfaceConfiguration(t *testing.T) {
	evidence := automationEvidence(nil)
	evidence.InterfaceConfiguration = &capmodel.ConfigurationSchemaDeclaration{
		Version: "v1",
		Fields: []*capmodel.ConfigurationFieldDeclaration{{
			Key: "endpoint", Type: capmodel.ConfigurationFieldSingleLine,
			Label: &capmodel.LocalizedText{EnUS: "Endpoint", ZhCN: "接口地址"},
		}},
	}
	if validProviderEvidence(evidence) {
		t.Fatal("interface configuration without explicit cardinality must be rejected")
	}
	evidence.InterfaceConfigurationCardinality = capmodel.InterfaceConfigurationCardinalitySingleton
	if validProviderEvidence(evidence) {
		t.Fatal("product automation owner must reject singleton supplier interfaces")
	}
	evidence.InterfaceConfigurationCardinality = capmodel.InterfaceConfigurationCardinalityMultiple
	if !validProviderEvidence(evidence) {
		t.Fatal("multiple named supplier interfaces must be accepted")
	}
}

func TestFrozenBindingEvidenceMatchesExactProviderWithoutPersistingMetadata(t *testing.T) {
	exact := automationEvidence(nil)
	frozen := cloneAutomationEvidence(exact)
	frozen.ProviderMetadata = nil
	if !validFrozenEvidence(frozen) {
		t.Fatal("persisted binding identity must remain valid without duplicating provider metadata")
	}
	if validProviderEvidence(frozen) {
		t.Fatal("persisted binding identity must not be accepted as exact provider evidence")
	}
	frozen.DisplayName = "商家自定义接口名称"
	if !sameExactEvidence(frozen, exact) {
		t.Fatal("frozen binding identity with a merchant-owned display name did not match its exact provider release")
	}
	exact.ProviderMetadata = nil
	if sameExactEvidence(frozen, exact) {
		t.Fatal("exact provider evidence without owner metadata was accepted")
	}
}

func TestProviderEvidenceRejectsUnsafeFulfillmentContract(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		mutate func(*productfulfillmentcap.ProviderDeclaration)
	}{
		{name: "untrimmed capability", mutate: func(value *productfulfillmentcap.ProviderDeclaration) {
			value.Capability.EnUS = " Automated fulfillment"
		}},
		{name: "oversized capability", mutate: func(value *productfulfillmentcap.ProviderDeclaration) {
			value.Capability.ZhCN = strings.Repeat("履", 257)
		}},
		{name: "unknown granularity", mutate: func(value *productfulfillmentcap.ProviderDeclaration) { value.Granularity = "per_request" }},
		{name: "missing query role", mutate: func(value *productfulfillmentcap.ProviderDeclaration) { value.MethodRoles.Query = "" }},
		{name: "duplicate roles", mutate: func(value *productfulfillmentcap.ProviderDeclaration) {
			value.MethodRoles.Query = value.MethodRoles.Provision
		}},
		{name: "unauthorized role", mutate: func(value *productfulfillmentcap.ProviderDeclaration) { value.MethodRoles.Provision = "submit" }},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			evidence := automationEvidence(nil)
			declaration := testFulfillmentDeclaration()
			testCase.mutate(declaration)
			evidence.ProviderMetadata = mustTestProviderMetadata(t, declaration)
			if validProviderEvidence(evidence) {
				t.Fatal("unsafe fulfillment contract was accepted")
			}
		})
	}
}

func TestCreateInterfaceClassifiesOnlyInvalidSelectionSentinelAsInvalid(t *testing.T) {
	for _, testCase := range []struct {
		name string
		err  error
		code *bizerr.Code
	}{
		{name: "invalid token", err: fmt.Errorf("decode: %w", ErrInvalidSelectionToken), code: catalog.CodeManagementInvalid},
		{name: "codec unavailable", err: errors.New("key service unavailable"), code: catalog.CodeManagementUnavailable},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			evidence := automationEvidence(nil)
			service := newAutomationUnitService(t,
				&automationTestExtensions{evidence: []*capmodel.BusinessExtensionProviderEvidence{evidence}},
				&automationTestAuthorizer{},
				&automationTestCodec{evidence: evidence, decodeErr: testCase.err},
			)
			_, err := service.CreateInterface(context.Background(), CreateInterfaceInput{DisplayName: "接口 A", SelectionToken: "opaque"})
			if err == nil || !bizerr.Is(err, testCase.code) {
				t.Fatalf("expected %s classification, got %v", testCase.code, err)
			}
		})
	}
}

func TestStateUpdateAuthorizesBeforeValidatingCallerInput(t *testing.T) {
	denied := bizerr.NewCode(catalog.CodeManagementNotAccessible)
	authorizer := &automationTestAuthorizer{err: denied}
	service := newAutomationUnitService(t, &automationTestExtensions{}, authorizer, &automationTestCodec{})
	_, err := service.UpdateNewBusinessState(context.Background(), InterfaceStateInput{InterfaceInstanceNo: "bad", NewBusinessState: "bad"})
	if !bizerr.Is(err, catalog.CodeManagementNotAccessible) {
		t.Fatalf("invalid unauthorized state update leaked validation: %v", err)
	}
	if len(authorizer.calls) != 1 || authorizer.calls[0] != "interface-rename" {
		t.Fatalf("authorization order=%v", authorizer.calls)
	}
}

func newAutomationUnitService(t *testing.T, extensions *automationTestExtensions, authorizer *automationTestAuthorizer, codec *automationTestCodec) Service {
	t.Helper()
	database, err := gdb.New(gdb.ConfigNode{Link: "pgsql:postgres:unused@tcp(127.0.0.1:1)/unused?sslmode=disable"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(context.Background()); closeErr != nil {
			t.Errorf("close unit database: %v", closeErr)
		}
	})
	service, err := newWithRandom(database, extensions, authorizer, &automationTestAudit{}, codec, bytes.NewReader(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func automationEvidence(methods []string) *capmodel.BusinessExtensionProviderEvidence {
	if methods == nil {
		methods = []string{"provision", "query.status"}
	}
	return &capmodel.BusinessExtensionProviderEvidence{
		PluginID: "server-automation", DisplayName: "Provider A", ReleaseVersion: "2026.8.0",
		Point: capmodel.BusinessExtensionProductFulfillment, ContractVersion: "v1",
		AuthorizedMethods: methods, ReleaseDigest: strings.Repeat("1", 64), DeclarationDigest: strings.Repeat("a", 64),
		ProviderMetadata: testProviderMetadata(),
	}
}

func testFulfillmentDeclaration() *productfulfillmentcap.ProviderDeclaration {
	return &productfulfillmentcap.ProviderDeclaration{
		Capability:       productfulfillmentcap.ProviderCapability{EnUS: "Automated provisioning", ZhCN: "自动开通"},
		Granularity:      productfulfillmentcap.GranularityPerOrderItem,
		RenewalSupport:   productfulfillmentcap.RenewalSupportUnsupported,
		ExpiryManagement: productfulfillmentcap.ExpiryManagementUnsupported,
		MethodRoles:      productfulfillmentcap.ProviderMethodRoles{Provision: "provision", Query: "query.status"},
	}
}

func testProviderMetadata() *capmodel.BusinessExtensionProviderMetadata {
	metadata, err := productfulfillmentcap.NewProviderMetadata(testFulfillmentDeclaration())
	if err != nil {
		panic(err)
	}
	return metadata
}

func mustTestProviderMetadata(t *testing.T, declaration *productfulfillmentcap.ProviderDeclaration) *capmodel.BusinessExtensionProviderMetadata {
	t.Helper()
	metadata, err := productfulfillmentcap.NewProviderMetadata(declaration)
	if err != nil {
		t.Fatal(err)
	}
	return metadata
}

func cloneAutomationEvidence(value *capmodel.BusinessExtensionProviderEvidence) *capmodel.BusinessExtensionProviderEvidence {
	if value == nil {
		return nil
	}
	clone := *value
	clone.AuthorizedMethods = append([]string{}, value.AuthorizedMethods...)
	if value.ProviderMetadata != nil {
		metadata := *value.ProviderMetadata
		metadata.Values = make(map[string]any, len(value.ProviderMetadata.Values))
		for key, item := range value.ProviderMetadata.Values {
			metadata.Values[key] = item
		}
		clone.ProviderMetadata = &metadata
	}
	return &clone
}
