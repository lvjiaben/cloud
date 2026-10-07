// This file verifies contract preparation and guarded flow operations against PostgreSQL.

package electroniccontractpreparation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-platform/idc-compliance-gateway/pkg/processauth"
	"lina-plugin-idc-compliance/backend/internal/service/contractlifecycle"

	"lina-plugin-idc-core/backend/cap/contractcap"
	"lina-plugin-idc-core/backend/cap/contracttemplatecap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

const preparationPostgreSQLDSNEnvironment = "LINAPRO_IDC_COMPLIANCE_TEST_DSN"

type preparationOrders struct {
	mu            sync.Mutex
	items         map[string]*contractcap.OrderSnapshot
	prepareCalls  int
	optionalCalls int
	matchCalls    int
}

func (s *preparationOrders) PrepareOptionalOrder(
	_ context.Context,
	orderNo string,
) (*contractcap.OrderSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.optionalCalls++
	item := s.items[orderNo]
	if item == nil || item.RequiresContract {
		return nil, errors.New("optional order unavailable")
	}
	return cloneOrderSnapshot(item), nil
}

func (s *preparationOrders) PrepareOrder(
	_ context.Context,
	orderNo string,
) (*contractcap.OrderSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prepareCalls++
	item := s.items[orderNo]
	if item == nil {
		return nil, errors.New("order unavailable")
	}
	return cloneOrderSnapshot(item), nil
}

func (s *preparationOrders) MatchCustomerSigner(
	_ context.Context,
	identity contractcap.VerifiedIdentity,
	observed *contractcap.ObservedCustomerSigner,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.matchCalls++
	if identity.CustomerNo != "CUSTOMER_001" || observed == nil ||
		string(observed.LegalName) != "示例客户" {
		return errors.New("signer mismatch")
	}
	return nil
}

type preparationAudit struct {
	mu          sync.Mutex
	fail        bool
	failScope   string
	seen        map[string]struct{}
	transitions []idccap.BusinessAuditTransition
}

type preparationTemplates struct {
	template contracttemplatecap.Template
}

type preparationContractGateway struct {
	mu                 sync.Mutex
	calls              int
	terminationCalls   int
	linkCalls          int
	recheckCalls       int
	request            processauth.TencentContractFlowRequest
	terminationRequest processauth.TencentContractTerminationRequest
	linkRequest        processauth.TencentContractSigningLinkRequest
	recheckStatus      string
}

func (g *preparationContractGateway) CreateTencentContractTermination(
	_ context.Context,
	request processauth.TencentContractTerminationRequest,
	_ string,
) (processauth.TencentContractFlowResponse, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.terminationCalls++
	g.terminationRequest = request
	return processauth.TencentContractFlowResponse{
		RouteNo: "lwr_1123456789abcdef0123456789abcdef", Status: "signing",
	}, nil
}

type preparationCustomerReader struct{}

func (preparationCustomerReader) CurrentCustomer(context.Context) (*identitycap.CustomerSubject, error) {
	return &identitycap.CustomerSubject{
		CustomerNo: "CUSTOMER_001", CustomerTypeCode: "individual",
		PrincipalReference: "usr:101", ActorUserID: 101,
	}, nil
}

func (g *preparationContractGateway) CreateTencentContractFlow(
	_ context.Context,
	request processauth.TencentContractFlowRequest,
	_ string,
) (processauth.TencentContractFlowResponse, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls++
	g.request = request
	routeNo := "lwr_0123456789abcdef0123456789abcdef"
	if g.calls > 1 {
		routeNo = "lwr_2123456789abcdef0123456789abcdef"
	}
	return processauth.TencentContractFlowResponse{
		RouteNo: routeNo, Status: "signing",
	}, nil
}

func (g *preparationContractGateway) CreateTencentContractSigningLink(
	_ context.Context,
	input processauth.TencentContractSigningLinkRequest,
) (processauth.TencentContractSigningLinkResponse, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.linkCalls++
	g.linkRequest = input
	return processauth.TencentContractSigningLinkResponse{
		URL: "https://tencent.example.test/sign", ExpiresUnix: time.Now().Add(30 * time.Minute).Unix(),
	}, nil
}

func (g *preparationContractGateway) RecheckTencentContractFlow(
	_ context.Context,
	input processauth.TencentContractFlowRecheckRequest,
) (processauth.TencentContractFlowRecheckResponse, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.recheckCalls++
	status := g.recheckStatus
	if status == "" {
		status = "succeeded"
	}
	providerStatus := "ALL"
	if status == "failed" {
		providerStatus = "EXCEPTION"
	}
	return processauth.TencentContractFlowRecheckResponse{
		RouteNo: input.RouteNo, Status: status, ProviderStatus: providerStatus,
		ReconciliationStatus: "none", RouteVersion: 3,
		SupplierObservedUnix: time.Now().UTC().Unix(), DecisionCode: "applied",
	}, nil
}

func (g *preparationContractGateway) ListTencentContractDeliveries(
	context.Context,
	int,
) ([]processauth.TencentContractDeliveryItem, error) {
	return nil, nil
}

func (g *preparationContractGateway) AcknowledgeTencentContractDeliveries(
	context.Context,
	[]string,
	string,
) error {
	return nil
}

type preparationAuthorizer struct {
	mu       sync.Mutex
	allowed  bool
	requests []managementcap.AuthorizationRequest
}

func (a *preparationAuthorizer) Authorize(
	_ context.Context,
	request managementcap.AuthorizationRequest,
) (int, error) {
	if err := managementcap.ValidateAuthorizationRequest(request); err != nil {
		return 0, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.requests = append(a.requests, request)
	if !a.allowed {
		return 0, errors.New("contract creation is not authorized")
	}
	return 101, nil
}

func (s *preparationTemplates) ListAvailable(
	_ context.Context,
	selection contracttemplatecap.Selection,
) ([]contracttemplatecap.Template, error) {
	if err := contracttemplatecap.ValidateTemplate(s.template, selection); err != nil {
		return nil, err
	}
	return []contracttemplatecap.Template{s.template}, nil
}

func (s *preparationTemplates) ResolveAvailable(
	_ context.Context,
	selection contracttemplatecap.Selection,
	templateNos []string,
) ([]contracttemplatecap.Template, error) {
	if err := contracttemplatecap.ValidateResolveRequest(selection, templateNos); err != nil {
		return nil, err
	}
	if len(templateNos) != 1 || templateNos[0] != s.template.TemplateNo {
		return nil, contracttemplatecap.ErrTemplateUnavailable
	}
	if err := contracttemplatecap.ValidateTemplate(s.template, selection); err != nil {
		return nil, err
	}
	return []contracttemplatecap.Template{s.template}, nil
}

func (*preparationTemplates) OrderContractExists(context.Context, string) (bool, error) {
	return false, nil
}

func (a *preparationAudit) AppendBusinessTransition(
	_ context.Context,
	transition idccap.BusinessAuditTransition,
) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.fail || a.failScope == transition.IdempotencyScope {
		return errors.New("audit unavailable")
	}
	if a.seen == nil {
		a.seen = make(map[string]struct{})
	}
	key := transition.IdempotencyScope + "\x00" + transition.IdempotencyKey
	if _, exists := a.seen[key]; exists {
		return nil
	}
	a.seen[key] = struct{}{}
	a.transitions = append(a.transitions, transition)
	return nil
}

func (*preparationAudit) AppendBusinessBatchTransition(
	context.Context,
	idccap.BusinessAuditBatchTransition,
) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, nil
}

type preparationSecretVault struct {
	mu      sync.Mutex
	byKey   map[string]string
	payload map[string][]byte
	next    int
}

func newPreparationSecretVault() *preparationSecretVault {
	return &preparationSecretVault{byKey: make(map[string]string), payload: make(map[string][]byte)}
}

func (v *preparationSecretVault) Create(
	_ context.Context,
	idempotencyKey string,
	produce secretcap.PlaintextProducer,
) (secretcap.CreateResult, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if reference := v.byKey[idempotencyKey]; reference != "" {
		handle, _ := secretcap.NewHandle(reference)
		return secretcap.CreateResult{
			Handle: handle, Metadata: preparationSecretMetadata(), Reused: true,
		}, nil
	}
	var stored []byte
	if produce == nil {
		return secretcap.CreateResult{}, errors.New("producer unavailable")
	}
	if err := produce(func(value []byte) error {
		stored = append([]byte(nil), value...)
		return nil
	}); err != nil {
		clear(stored)
		return secretcap.CreateResult{}, err
	}
	v.next++
	reference := "SEC_CONTRACT_TEST_" + strings.Repeat("0", 28) + hex.EncodeToString([]byte{byte(v.next)})
	handle, err := secretcap.NewHandle(reference)
	if err != nil {
		clear(stored)
		return secretcap.CreateResult{}, err
	}
	v.byKey[idempotencyKey] = reference
	v.payload[reference] = stored
	return secretcap.CreateResult{Handle: handle, Metadata: preparationSecretMetadata()}, nil
}

func (v *preparationSecretVault) RecoverCreate(
	_ context.Context,
	idempotencyKey string,
) (secretcap.CreateResult, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	reference := v.byKey[idempotencyKey]
	if reference == "" {
		return secretcap.CreateResult{}, errors.New("secret unavailable")
	}
	handle, _ := secretcap.NewHandle(reference)
	return secretcap.CreateResult{Handle: handle, Metadata: preparationSecretMetadata(), Reused: true}, nil
}

func (*preparationSecretVault) Replace(
	context.Context,
	secretcap.Handle,
	time.Time,
	string,
	secretcap.PlaintextProducer,
) (secretcap.MutationResult, error) {
	return secretcap.MutationResult{}, errors.New("replace is unsupported")
}

func (*preparationSecretVault) Metadata(
	context.Context,
	secretcap.Handle,
) (secretcap.Metadata, error) {
	return preparationSecretMetadata(), nil
}

func (*preparationSecretVault) Revoke(
	context.Context,
	secretcap.Handle,
	time.Time,
) (secretcap.Metadata, error) {
	return secretcap.Metadata{}, errors.New("revoke is unsupported")
}

func (*preparationSecretVault) Rotate(
	context.Context,
	int,
) (secretcap.RotationResult, error) {
	return secretcap.RotationResult{}, nil
}

func (v *preparationSecretVault) Access(
	_ context.Context,
	request secretcap.AccessRequest,
	consume secretcap.Consumer,
) error {
	if err := secretcap.ValidateAccess(request, consume); err != nil {
		return err
	}
	v.mu.Lock()
	value := append([]byte(nil), v.payload[request.Handle.OpaqueRef()]...)
	v.mu.Unlock()
	if len(value) == 0 {
		return errors.New("secret unavailable")
	}
	defer clear(value)
	return consume(value)
}

func preparationSecretMetadata() secretcap.Metadata {
	return secretcap.Metadata{
		Configured: true,
		UpdatedAt:  time.Date(2026, time.August, 15, 8, 30, 0, 0, time.UTC),
	}
}

func TestElectronicContractPreparationPostgreSQL(t *testing.T) {
	database, cleanup := newPreparationTestDatabase(t)
	defer cleanup()
	ctx := context.Background()
	orders := &preparationOrders{items: map[string]*contractcap.OrderSnapshot{
		"ORDER_20260815-001": validOrderSnapshot(),
		"ORDER_20260815-002": func() *contractcap.OrderSnapshot {
			item := validOrderSnapshot()
			item.OrderNo = "ORDER_20260815-002"
			return item
		}(),
		"ORDER_20260815-003": func() *contractcap.OrderSnapshot {
			item := validOrderSnapshot()
			item.OrderNo = "ORDER_20260815-003"
			item.StatusCode = "fulfilled"
			item.RequiresContract = false
			item.Template = contractcap.TemplateSnapshot{}
			for index := range item.Items {
				item.Items[index].ContractProductName = ""
			}
			return item
		}(),
	}}
	defer func() {
		for _, item := range orders.items {
			item.Clear()
		}
	}()
	audit := &preparationAudit{fail: true}
	vault := newPreparationSecretVault()
	authorizer := &preparationAuthorizer{allowed: true}
	templates := &preparationTemplates{template: contracttemplatecap.Template{
		TemplateNo: "ctpl_0123456789abcdef0123456789abcdef", VersionNo: 5,
		Name: "Optional service contract", ScopeCode: "personal",
		SigningOrderCode: "customer_first", LanguageCode: "en-US",
	}}
	gateway := &preparationContractGateway{}
	lifecycle, err := contractlifecycle.New(database, gateway, audit)
	if err != nil {
		t.Fatalf("construct contract lifecycle service: %v", err)
	}
	serviceValue, err := NewWithGateway(
		database,
		contractcap.Runtime{Orders: orders, Audit: audit},
		vault,
		vault,
		templates,
		authorizer,
		testArtifactTextCatalog(t),
		gateway,
		lifecycle,
		preparationCustomerReader{},
		"https://customer.example.test/account/contracts?signing_return=1",
	)
	if err != nil {
		t.Fatalf("construct preparation service: %v", err)
	}
	service := serviceValue.(*serviceImpl)

	input := PrepareInput{
		OrderNo: "ORDER_20260815-001", IdempotencyKey: "contract-create-20260815-0001",
	}
	if prepared, prepareErr := service.PrepareRequired(ctx, input); prepareErr == nil {
		prepared.Clear()
		t.Fatal("audit failure did not roll back contract preparation")
	}
	count, err := database.GetValue(ctx, "SELECT count(*) FROM plugin_idc_compliance_contract")
	if err != nil || count.Int() != 0 {
		t.Fatalf("contract persisted after audit failure count=%d err=%v", count.Int(), err)
	}

	audit.mu.Lock()
	audit.fail = false
	audit.mu.Unlock()
	prepared, err := service.PrepareRequired(ctx, input)
	if err != nil {
		t.Fatalf("recover prepared contract: %v", err)
	}
	contractNo := prepared.ContractNo
	if !prepared.Reused || prepared.Preview == nil || prepared.Preview.SigningDate != "" ||
		len(prepared.Preview.Items) != 2 || prepared.AmountMinor != 129900 ||
		!prepared.SigningExpiresAt.Equal(prepared.CreatedAt.Add(SigningValidity)) {
		t.Fatalf("unexpected recovered preparation: %+v", prepared)
	}
	prepared.Clear()

	replayed, err := service.PrepareRequired(ctx, input)
	if err != nil || !replayed.Reused || replayed.ContractNo != contractNo {
		t.Fatalf("same-key replay result=%+v err=%v", replayed, err)
	}
	replayed.Clear()
	orderReplay, err := service.PrepareRequired(ctx, PrepareInput{
		OrderNo: "ORDER_20260815-001", IdempotencyKey: "contract-create-20260815-0002",
	})
	if err != nil || !orderReplay.Reused || orderReplay.ContractNo != contractNo {
		t.Fatalf("same-order replay result=%+v err=%v", orderReplay, err)
	}
	orderReplay.Clear()
	if conflicting, conflictErr := service.PrepareRequired(ctx, PrepareInput{
		OrderNo: "ORDER_20260815-002", IdempotencyKey: "contract-create-20260815-0001",
	}); conflictErr == nil {
		conflicting.Clear()
		t.Fatal("one idempotency key was accepted for a different order")
	}

	authorizer.mu.Lock()
	authorizer.allowed = false
	authorizer.mu.Unlock()
	if denied, deniedErr := service.PrepareManualOptional(ctx, ManualOptionalInput{
		OrderNo: "ORDER_20260815-003", TemplateNo: templates.template.TemplateNo,
		IdempotencyKey: "contract-create-20260815-denied-0001",
	}); deniedErr == nil {
		denied.Clear()
		t.Fatal("manual optional contract was prepared without its independent permission")
	}
	authorizer.mu.Lock()
	authorizer.allowed = true
	authorizer.mu.Unlock()
	candidates, err := service.ListManualOptionalTemplates(ctx, "ORDER_20260815-003")
	if err != nil || len(candidates) != 1 ||
		candidates[0].TemplateNo != templates.template.TemplateNo ||
		candidates[0].LanguageCode != "en-US" {
		t.Fatalf("list manual optional templates=%+v err=%v", candidates, err)
	}
	optional, err := service.PrepareManualOptional(ctx, ManualOptionalInput{
		OrderNo: "ORDER_20260815-003", TemplateNo: templates.template.TemplateNo,
		IdempotencyKey: "contract-create-20260815-optional-0001",
	})
	if err != nil {
		t.Fatalf("prepare manual optional contract: %v", err)
	}
	if optional.RequiredForFulfillment || optional.Template.TemplateNo != templates.template.TemplateNo ||
		optional.Preview == nil || len(optional.Preview.Items) != 2 ||
		optional.Preview.Items[0].ServiceName != "Dedicated Server" {
		t.Fatalf("unexpected manual optional preparation: %+v", optional)
	}
	optionalContractNo := optional.ContractNo
	optional.Clear()
	var optionalStored struct {
		RequiredForFulfillment bool `orm:"required_for_fulfillment"`
	}
	if err = database.GetScan(ctx, &optionalStored, `
SELECT required_for_fulfillment
FROM plugin_idc_compliance_contract
WHERE contract_no = ?`, optionalContractNo); err != nil || optionalStored.RequiredForFulfillment {
		t.Fatalf("optional contract changed fulfillment gate stored=%+v err=%v", optionalStored, err)
	}
	if orders.items["ORDER_20260815-003"].RequiresContract {
		t.Fatal("manual optional preparation changed the authoritative order requirement")
	}
	authorizer.mu.Lock()
	authorizationRequests := append([]managementcap.AuthorizationRequest(nil), authorizer.requests...)
	authorizer.mu.Unlock()
	if len(authorizationRequests) != 3 ||
		authorizationRequests[0].Permission != PermissionContractCreate ||
		authorizationRequests[0].TargetNo != "ORDER_20260815-003" ||
		authorizationRequests[1] != authorizationRequests[0] ||
		authorizationRequests[2] != authorizationRequests[0] {
		t.Fatalf("unexpected contract authorization requests: %+v", authorizationRequests)
	}
	observed := &contractcap.ObservedCustomerSigner{
		CustomerType: contractcap.CustomerTypeIndividual,
		LegalName:    []byte("示例客户"), PersonalIdentityNumber: []byte("110105199001011234"),
	}
	if err = service.MatchCustomerSigner(ctx, contractNo, observed); err != nil {
		t.Fatalf("match prepared contract signer: %v", err)
	}
	if len(observed.LegalName) != 0 || len(observed.PersonalIdentityNumber) != 0 {
		t.Fatal("provider signer identity buffers were not cleared")
	}

	submitted, err := service.SubmitSigning(ctx, SubmitSigningInput{
		ContractNo: contractNo, ExpectedVersion: 1,
		IdempotencyKey: "contract-submit-20260815-0001",
	})
	if err != nil || submitted == nil || submitted.StatusCode != "signing" || submitted.Version != 2 {
		t.Fatalf("submit contract signing=%+v err=%v", submitted, err)
	}
	replayedSubmission, err := service.SubmitSigning(ctx, SubmitSigningInput{
		ContractNo: contractNo, ExpectedVersion: 1,
		IdempotencyKey: "contract-submit-20260815-0001",
	})
	if err != nil || replayedSubmission == nil || !replayedSubmission.Reused ||
		replayedSubmission.FlowNo != submitted.FlowNo {
		t.Fatalf("replay contract signing=%+v err=%v", replayedSubmission, err)
	}
	gateway.mu.Lock()
	gatewayCalls := gateway.calls
	gatewayRequest := gateway.request
	gateway.mu.Unlock()
	if gatewayCalls != 1 || gatewayRequest.ContractNo != contractNo ||
		len(gatewayRequest.FormFields) < 1 || len(gatewayRequest.Approvers) != 2 {
		t.Fatalf("gateway calls=%d request=%+v", gatewayCalls, gatewayRequest)
	}
	var storedLaunch struct {
		ContractStatus  string  `orm:"contract_status"`
		ContractVersion int64   `orm:"contract_version"`
		FlowStatus      string  `orm:"flow_status"`
		GatewayRouteNo  *string `orm:"gateway_route_no"`
	}
	if err = database.GetScan(ctx, &storedLaunch, `
SELECT contract.status_code AS contract_status, contract.version AS contract_version,
       flow.status_code AS flow_status, flow.gateway_route_no
FROM plugin_idc_compliance_contract AS contract
JOIN plugin_idc_compliance_contract_flow AS flow ON flow.contract_no = contract.contract_no
WHERE contract.tenant_id = 0 AND contract.contract_no = ?`, contractNo); err != nil ||
		storedLaunch.ContractStatus != "signing" || storedLaunch.ContractVersion != 2 ||
		storedLaunch.FlowStatus != "signing" || storedLaunch.GatewayRouteNo == nil ||
		*storedLaunch.GatewayRouteNo != "lwr_0123456789abcdef0123456789abcdef" {
		t.Fatalf("stored signing launch=%+v err=%v", storedLaunch, err)
	}
	page, err := service.ListCurrentContracts(ctx, CustomerContractQuery{Limit: 20})
	if err != nil || page == nil || len(page.Items) != 2 || page.NextCursor != "" {
		t.Fatalf("customer contract page=%+v err=%v", page, err)
	}
	detail, err := service.CurrentOrderContract(ctx, "ORDER_20260815-001")
	if err != nil || detail.ContractNo != contractNo || detail.StatusCode != "processing" ||
		!detail.CanOpenSigning || !detail.CanRecheck {
		t.Fatalf("customer order contract=%+v err=%v", detail, err)
	}
	contractCountBefore, err := database.GetValue(ctx, "SELECT count(*) FROM plugin_idc_compliance_contract")
	if err != nil {
		t.Fatal(err)
	}
	flowCountBefore, err := database.GetValue(ctx, "SELECT count(*) FROM plugin_idc_compliance_contract_flow")
	if err != nil {
		t.Fatal(err)
	}
	link, err := service.CreateCurrentSigningLink(ctx, contractNo)
	if err != nil || link == nil || link.Contract.ContractNo != contractNo ||
		link.URL != "https://tencent.example.test/sign" {
		t.Fatalf("customer signing link=%+v err=%v", link, err)
	}
	contractCountAfter, _ := database.GetValue(ctx, "SELECT count(*) FROM plugin_idc_compliance_contract")
	flowCountAfter, _ := database.GetValue(ctx, "SELECT count(*) FROM plugin_idc_compliance_contract_flow")
	gateway.mu.Lock()
	linkCalls := gateway.linkCalls
	linkRequest := gateway.linkRequest
	gateway.mu.Unlock()
	if contractCountAfter.Int64() != contractCountBefore.Int64() ||
		flowCountAfter.Int64() != flowCountBefore.Int64() || linkCalls != 1 ||
		linkRequest.RouteNo != *storedLaunch.GatewayRouteNo ||
		linkRequest.JumpURL != "https://customer.example.test/account/contracts?signing_return=1" ||
		linkRequest.Approver.PartyCode != "customer" {
		t.Fatalf("link changed flow contract=%d/%d flow=%d/%d calls=%d request=%+v",
			contractCountBefore.Int64(), contractCountAfter.Int64(), flowCountBefore.Int64(),
			flowCountAfter.Int64(), linkCalls, linkRequest)
	}
	if _, err = database.Exec(ctx, `
UPDATE plugin_idc_compliance_contract_flow
SET status_code = 'result_unknown', version = version + 1
WHERE tenant_id = 0 AND contract_no = ?`, contractNo); err != nil {
		t.Fatalf("prepare result-unknown replay: %v", err)
	}
	unknownReplay, err := service.SubmitSigning(ctx, SubmitSigningInput{
		ContractNo: contractNo, ExpectedVersion: 1,
		IdempotencyKey: "contract-submit-20260815-0001",
	})
	if err != nil || unknownReplay == nil || !unknownReplay.Reused ||
		unknownReplay.StatusCode != "result_unknown" || unknownReplay.FlowNo != submitted.FlowNo {
		t.Fatalf("recover result-unknown signing=%+v err=%v", unknownReplay, err)
	}
	gateway.mu.Lock()
	gatewayCalls = gateway.calls
	gateway.mu.Unlock()
	if gatewayCalls != 1 {
		t.Fatalf("result-unknown replay called gateway %d times, want one total call", gatewayCalls)
	}

	orders.mu.Lock()
	prepareCalls, optionalCalls, matchCalls := orders.prepareCalls, orders.optionalCalls, orders.matchCalls
	orders.mu.Unlock()
	if prepareCalls != 2 || optionalCalls != 2 || matchCalls != 1 {
		t.Fatalf(
			"unexpected core calls prepare=%d optional=%d match=%d",
			prepareCalls, optionalCalls, matchCalls,
		)
	}
	audit.mu.Lock()
	auditCount := len(audit.transitions)
	audit.mu.Unlock()
	if auditCount != 5 {
		t.Fatalf("contract preparation audit count=%d want=5", auditCount)
	}
	refreshed, err := service.RecheckCurrentContract(ctx, contractNo)
	if err != nil || refreshed == nil || refreshed.StatusCode != "completed" ||
		refreshed.CanOpenSigning || refreshed.CanRecheck {
		t.Fatalf("customer contract recheck=%+v err=%v", refreshed, err)
	}
	gateway.mu.Lock()
	recheckCalls := gateway.recheckCalls
	gateway.mu.Unlock()
	if recheckCalls != 1 {
		t.Fatalf("authoritative customer recheck calls=%d want=1", recheckCalls)
	}

	rollbackPrepared, err := service.PrepareRequired(ctx, PrepareInput{
		OrderNo: "ORDER_20260815-002", IdempotencyKey: "contract-create-20260815-0003",
	})
	if err != nil {
		t.Fatalf("prepare contract for signing audit rollback: %v", err)
	}
	rollbackContractNo := rollbackPrepared.ContractNo
	rollbackVersion := rollbackPrepared.Version
	rollbackPrepared.Clear()
	audit.mu.Lock()
	audit.failScope = "compliance.contract.submit.signing"
	audit.mu.Unlock()
	if failedSubmission, submitErr := service.SubmitSigning(ctx, SubmitSigningInput{
		ContractNo: rollbackContractNo, ExpectedVersion: rollbackVersion,
		IdempotencyKey: "contract-submit-20260815-audit-rollback-0001",
	}); submitErr == nil {
		t.Fatalf("signing submission succeeded while final audit was unavailable: %+v", failedSubmission)
	}
	audit.mu.Lock()
	audit.failScope = ""
	audit.mu.Unlock()
	var rollbackStored struct {
		ContractStatus  string  `orm:"contract_status"`
		ContractVersion int64   `orm:"contract_version"`
		FlowStatus      string  `orm:"flow_status"`
		GatewayRouteNo  *string `orm:"gateway_route_no"`
	}
	if err = database.GetScan(ctx, &rollbackStored, `
SELECT contract.status_code AS contract_status, contract.version AS contract_version,
       flow.status_code AS flow_status, flow.gateway_route_no
FROM plugin_idc_compliance_contract AS contract
JOIN plugin_idc_compliance_contract_flow AS flow ON flow.contract_no = contract.contract_no
WHERE contract.tenant_id = 0 AND contract.contract_no = ?`, rollbackContractNo); err != nil ||
		rollbackStored.ContractStatus != "pending_launch" || rollbackStored.ContractVersion != rollbackVersion ||
		rollbackStored.FlowStatus != "submitting" || rollbackStored.GatewayRouteNo != nil {
		t.Fatalf("final audit failure did not roll back signing state stored=%+v err=%v", rollbackStored, err)
	}
}

func TestContractReissueReplacementAndTerminationPostgreSQL(t *testing.T) {
	database, cleanup := newPreparationTestDatabase(t)
	defer cleanup()
	ctx := context.Background()
	orders := &preparationOrders{items: map[string]*contractcap.OrderSnapshot{
		"ORDER_20260815-001": validOrderSnapshot(),
	}}
	defer orders.items["ORDER_20260815-001"].Clear()
	audit := &preparationAudit{}
	vault := newPreparationSecretVault()
	authorizer := &preparationAuthorizer{allowed: true}
	templates := &preparationTemplates{template: contracttemplatecap.Template{
		TemplateNo: "ctpl_0123456789abcdef0123456789abcdef", VersionNo: 5,
		Name: "Replacement service contract", ScopeCode: "personal",
		SigningOrderCode: "customer_first", LanguageCode: "en-US",
	}}
	gateway := &preparationContractGateway{}
	lifecycle, err := contractlifecycle.New(database, gateway, audit)
	if err != nil {
		t.Fatalf("construct replacement lifecycle: %v", err)
	}
	serviceValue, err := NewWithGateway(
		database, contractcap.Runtime{Orders: orders, Audit: audit}, vault, vault,
		templates, authorizer, testArtifactTextCatalog(t), gateway, lifecycle,
		preparationCustomerReader{},
		"https://customer.example.test/account/contracts?signing_return=1",
	)
	if err != nil {
		t.Fatalf("construct replacement preparation service: %v", err)
	}
	service := serviceValue.(*serviceImpl)
	if _, err = service.PrepareReplacement(ctx, PrepareReplacementInput{}); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("invalid replacement error is not structured: %v", err)
	}
	if _, err = service.GetReplacementOptions(ctx, ""); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("invalid replacement options error is not structured: %v", err)
	}
	if _, err = service.PrepareReplacement(ctx, PrepareReplacementInput{
		ContractNo:     "ctr_0123456789abcdef0123456789abcdef",
		TemplateNo:     "ctpl_0123456789abcdef0123456789abcdef",
		IdempotencyKey: "contract-replacement-missing-test-0001",
	}); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("missing replacement source error is not structured: %v", err)
	}
	if _, err = service.SubmitReissue(ctx, ReissueSigningInput{}); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("invalid reissue error is not structured: %v", err)
	}
	if _, err = service.TerminateContract(ctx, TerminateContractInput{}); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("invalid termination error is not structured: %v", err)
	}
	prepared, err := service.PrepareRequired(ctx, PrepareInput{
		OrderNo: "ORDER_20260815-001", IdempotencyKey: "contract-create-reissue-test-0001",
	})
	if err != nil {
		t.Fatalf("prepare original contract: %v", err)
	}
	originalContractNo := prepared.ContractNo
	prepared.Clear()
	submitted, err := service.SubmitSigning(ctx, SubmitSigningInput{
		ContractNo: originalContractNo, ExpectedVersion: 1,
		IdempotencyKey: "contract-submit-reissue-test-0001",
	})
	if err != nil {
		t.Fatalf("submit original contract: %v", err)
	}
	if submitted == nil || submitted.StatusCode != "signing" {
		t.Fatalf("unexpected original signing result: %+v", submitted)
	}
	databaseNow, err := database.GetValue(ctx, "SELECT CURRENT_TIMESTAMP")
	if err != nil {
		t.Fatalf("load operation test database time: %v", err)
	}
	rejectedAt := databaseNow.Time().UTC().Add(-3 * time.Minute)
	if _, err = lifecycle.Apply(ctx, contractlifecycle.Observation{
		RouteNo: "lwr_0123456789abcdef0123456789abcdef", RouteVersion: 2,
		ProviderStatus: "REJECT", SourceKind: "active_query", GatewayDecisionCode: "applied",
		SupplierObservedAt: rejectedAt, ObservedAt: rejectedAt,
	}, audit); err != nil {
		t.Fatalf("converge original rejection: %v", err)
	}
	replacementOptions, err := service.GetReplacementOptions(ctx, originalContractNo)
	if err != nil || replacementOptions == nil || replacementOptions.ContractNo != originalContractNo ||
		replacementOptions.OrderNo != "ORDER_20260815-001" || len(replacementOptions.Templates) != 1 ||
		replacementOptions.Templates[0].TemplateNo != templates.template.TemplateNo ||
		len(replacementOptions.Signers) != 0 {
		t.Fatalf("safe replacement options=%+v err=%v", replacementOptions, err)
	}

	replacement, err := service.PrepareReplacement(ctx, PrepareReplacementInput{
		ContractNo: originalContractNo, TemplateNo: templates.template.TemplateNo,
		IdempotencyKey: "contract-replacement-test-0001",
	})
	if err != nil {
		t.Fatalf("prepare changed replacement: %v", err)
	}
	replacementContractNo := replacement.ContractNo
	replacementVersion := replacement.Version
	replacementDeadline := replacement.SigningExpiresAt
	if replacementContractNo == originalContractNo || replacement.Template.TemplateNo != templates.template.TemplateNo ||
		replacement.Preview == nil || replacement.Preview.Items[0].ServiceName != "Dedicated Server" {
		t.Fatalf("unexpected changed replacement preview: %+v", replacement)
	}
	replacement.Clear()
	replayedReplacement, err := service.PrepareReplacement(ctx, PrepareReplacementInput{
		ContractNo: originalContractNo, TemplateNo: templates.template.TemplateNo,
		IdempotencyKey: "contract-replacement-test-0001",
	})
	if err != nil || replayedReplacement == nil || !replayedReplacement.Reused ||
		replayedReplacement.ContractNo != replacementContractNo {
		t.Fatalf("replay changed replacement=%+v err=%v", replayedReplacement, err)
	}
	replayedReplacement.Clear()
	var lineage struct {
		OldContractNo string `orm:"old_contract_no"`
		NewContractNo string `orm:"new_contract_no"`
		Count         int    `orm:"relation_count"`
	}
	if err = database.GetScan(ctx, &lineage, `
SELECT min(old_contract_no) AS old_contract_no, min(new_contract_no) AS new_contract_no,
       count(*) AS relation_count
FROM plugin_idc_compliance_contract_replacement`); err != nil || lineage.Count != 1 ||
		lineage.OldContractNo != originalContractNo || lineage.NewContractNo != replacementContractNo {
		t.Fatalf("replacement lineage=%+v err=%v", lineage, err)
	}
	current, err := service.CurrentOrderContract(ctx, "ORDER_20260815-001")
	if err != nil || current.ContractNo != replacementContractNo {
		t.Fatalf("current order did not select replacement leaf=%+v err=%v", current, err)
	}
	reissue, err := service.SubmitReissue(ctx, ReissueSigningInput{
		ContractNo: replacementContractNo, ExpectedVersion: replacementVersion,
		IdempotencyKey: "contract-reissue-test-0001",
	})
	if err != nil || reissue == nil || reissue.StatusCode != "signing" {
		t.Fatalf("submit replacement reissue=%+v err=%v", reissue, err)
	}
	reissueReplay, err := service.SubmitReissue(ctx, ReissueSigningInput{
		ContractNo: replacementContractNo, ExpectedVersion: replacementVersion,
		IdempotencyKey: "contract-reissue-test-0001",
	})
	if err != nil || reissueReplay == nil || !reissueReplay.Reused ||
		reissueReplay.FlowNo != reissue.FlowNo {
		t.Fatalf("replay replacement reissue=%+v err=%v", reissueReplay, err)
	}
	var reissueStored struct {
		PurposeCode string    `orm:"purpose_code"`
		ExpiresAt   time.Time `orm:"expires_at"`
		CreatedAt   time.Time `orm:"created_at"`
	}
	if err = database.GetScan(ctx, &reissueStored, `
SELECT purpose_code, expires_at, created_at
FROM plugin_idc_compliance_contract_flow
	WHERE tenant_id = 0 AND flow_no = ?`, reissue.FlowNo); err != nil ||
		reissueStored.PurposeCode != "reissue" ||
		!reissueStored.ExpiresAt.Equal(replacementDeadline) ||
		!reissueStored.ExpiresAt.After(reissueStored.CreatedAt) {
		t.Fatalf("stored replacement reissue=%+v err=%v", reissueStored, err)
	}
	completedAt := databaseNow.Time().UTC().Add(-2 * time.Minute)
	if _, err = lifecycle.Apply(ctx, contractlifecycle.Observation{
		RouteNo: "lwr_2123456789abcdef0123456789abcdef", RouteVersion: 2,
		ProviderStatus: "ALL", SourceKind: "callback", GatewayDecisionCode: "applied",
		SupplierObservedAt: completedAt, ObservedAt: completedAt,
	}, audit); err != nil {
		t.Fatalf("complete replacement contract: %v", err)
	}
	completedVersion, err := database.GetValue(ctx, `SELECT version
FROM plugin_idc_compliance_contract WHERE tenant_id = 0 AND contract_no = ?`, replacementContractNo)
	if err != nil || completedVersion.Int64() < 1 {
		t.Fatalf("load completed replacement version: %v", err)
	}
	terminationInput := TerminateContractInput{
		ContractNo: replacementContractNo, ExpectedVersion: completedVersion.Int64(),
		Reason: "双方协商一致解除", RemainInForceItem: "保密条款继续有效",
		OriginalExpenseSettlement: "费用已结清", Confirmed: true,
		IdempotencyKey: "contract-termination-test-0001",
	}
	termination, err := service.TerminateContract(ctx, terminationInput)
	if err != nil || termination == nil || termination.StatusCode != "signing" {
		t.Fatalf("submit contract termination=%+v err=%v", termination, err)
	}
	terminationReplay, err := service.TerminateContract(ctx, terminationInput)
	if err != nil || terminationReplay == nil || !terminationReplay.Reused ||
		terminationReplay.FlowNo != termination.FlowNo {
		t.Fatalf("replay contract termination=%+v err=%v", terminationReplay, err)
	}
	gateway.mu.Lock()
	flowCalls := gateway.calls
	terminationCalls := gateway.terminationCalls
	terminationRequest := gateway.terminationRequest
	gateway.mu.Unlock()
	if flowCalls != 2 || terminationCalls != 1 ||
		terminationRequest.ContractNo != replacementContractNo ||
		terminationRequest.Reason != terminationInput.Reason ||
		terminationRequest.OriginalRouteNo != "lwr_2123456789abcdef0123456789abcdef" {
		t.Fatalf("gateway operation calls flow=%d termination=%d request=%+v",
			flowCalls, terminationCalls, terminationRequest)
	}
	terminatedAt := databaseNow.Time().UTC().Add(-time.Minute)
	result, err := lifecycle.Apply(ctx, contractlifecycle.Observation{
		RouteNo: "lwr_1123456789abcdef0123456789abcdef", RouteVersion: 2,
		ProviderStatus: "ALL", SourceKind: "callback", GatewayDecisionCode: "applied",
		SupplierObservedAt: terminatedAt, ObservedAt: terminatedAt,
	}, audit)
	if err != nil || result.ContractStatus != "terminated" || result.FlowStatus != "completed" {
		t.Fatalf("converge contract termination=%+v err=%v", result, err)
	}
	var finalState struct {
		StatusCode   string    `orm:"status_code"`
		TerminatedAt time.Time `orm:"terminated_at"`
	}
	if err = database.GetScan(ctx, &finalState, `
SELECT status_code, terminated_at
FROM plugin_idc_compliance_contract
WHERE tenant_id = 0 AND contract_no = ?`, replacementContractNo); err != nil ||
		finalState.StatusCode != "terminated" || !finalState.TerminatedAt.Equal(terminatedAt) {
		t.Fatalf("authoritative termination state=%+v err=%v", finalState, err)
	}
	oldStatus, err := database.GetValue(ctx, `SELECT status_code
FROM plugin_idc_compliance_contract WHERE tenant_id = 0 AND contract_no = ?`, originalContractNo)
	if err != nil || oldStatus.String() != "rejected" {
		t.Fatalf("old contract history changed status=%q err=%v", oldStatus.String(), err)
	}
	authorizer.mu.Lock()
	requests := append([]managementcap.AuthorizationRequest(nil), authorizer.requests...)
	authorizer.mu.Unlock()
	var reissueAuthorized, terminateAuthorized bool
	for _, request := range requests {
		reissueAuthorized = reissueAuthorized || request.Permission == PermissionContractReissue
		terminateAuthorized = terminateAuthorized || request.Permission == PermissionContractTerminate
	}
	if !reissueAuthorized || !terminateAuthorized {
		t.Fatalf("independent contract operation permissions were not checked: %+v", requests)
	}
}

func newPreparationTestDatabase(t *testing.T) (gdb.DB, func()) {
	t.Helper()
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(preparationPostgreSQLDSNEnvironment))
	if dsn == "" {
		t.Skipf("set %s to run contract preparation PostgreSQL tests", preparationPostgreSQLDSNEnvironment)
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("open preparation test admin database: %v", err)
	}
	var random [8]byte
	if _, err = rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	schema := "idc_contract_prep_test_" + hex.EncodeToString(random[:])
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
	if _, err = database.Exec(ctx, preparationBaseSchema); err != nil {
		t.Fatalf("create preparation base schema: %v", err)
	}
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve preparation migration path")
	}
	migrationDirectory := filepath.Join(
		filepath.Dir(sourceFile), "..", "..", "..", "..", "manifest", "sql",
	)
	for _, name := range []string{
		"009-electronic-contract-template.sql", "010-electronic-contract-preparation.sql",
		"011-electronic-contract-provider-mapping.sql", "013-electronic-contract-launch.sql",
		"015-customer-contract-directory.sql", "016-contract-status-timeline.sql",
		"017-contract-reissue-termination.sql", "025-contract-failure-compensation.sql",
	} {
		migration, readErr := os.ReadFile(filepath.Join(migrationDirectory, name))
		if readErr != nil {
			t.Fatalf("read preparation migration %s: %v", name, readErr)
		}
		if _, err = database.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("apply preparation migration %s: %v", name, err)
		}
	}
	if _, err = database.Exec(ctx, preparationProviderFixtures); err != nil {
		t.Fatalf("create preparation provider fixtures: %v", err)
	}
	cleanup := func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Errorf("close preparation database: %v", closeErr)
		}
		if !strings.HasPrefix(schema, "idc_contract_prep_test_") {
			t.Errorf("refusing unexpected schema %q", schema)
		} else if _, dropErr := admin.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE"); dropErr != nil {
			t.Errorf("drop preparation schema: %v", dropErr)
		}
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Errorf("close preparation admin database: %v", closeErr)
		}
	}
	return database, cleanup
}

func cloneOrderSnapshot(source *contractcap.OrderSnapshot) *contractcap.OrderSnapshot {
	if source == nil {
		return nil
	}
	cloned := *source
	cloned.Customer.LegalName = append([]byte(nil), source.Customer.LegalName...)
	cloned.Customer.RegistrationIdentifier = append(
		[]byte(nil), source.Customer.RegistrationIdentifier...,
	)
	cloned.Customer.VerifiedPhone = append([]byte(nil), source.Customer.VerifiedPhone...)
	cloned.Merchant.LegalName = append([]byte(nil), source.Merchant.LegalName...)
	cloned.Items = append([]contractcap.OrderItem(nil), source.Items...)
	return &cloned
}

const preparationBaseSchema = `
CREATE TABLE plugin_idc_compliance_contract (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0,
    contract_no VARCHAR(64) NOT NULL UNIQUE,
    customer_no VARCHAR(64) NOT NULL,
    principal_no VARCHAR(64) NOT NULL,
    order_no VARCHAR(64) NOT NULL,
    service_no VARCHAR(64),
    contract_type_code VARCHAR(32) NOT NULL,
    required_for_fulfillment BOOLEAN NOT NULL DEFAULT FALSE,
    template_no VARCHAR(64) NOT NULL,
    template_version BIGINT NOT NULL,
    variable_snapshot_ref VARCHAR(256) NOT NULL,
    variable_snapshot_sha256 CHAR(64) NOT NULL,
    signer_snapshot_sha256 CHAR(64) NOT NULL,
    currency_code CHAR(3) NOT NULL,
    amount_minor BIGINT NOT NULL,
    status_code VARCHAR(24) NOT NULL DEFAULT 'draft',
    launched_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    terminated_at TIMESTAMPTZ,
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE plugin_idc_compliance_contract_flow (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0,
    flow_no VARCHAR(64) NOT NULL UNIQUE,
    contract_no VARCHAR(64) NOT NULL,
    sequence_no INT NOT NULL,
    purpose_code VARCHAR(16) NOT NULL,
    provider_code VARCHAR(32) NOT NULL,
    external_flow_ref VARCHAR(256),
    idempotency_sha256 CHAR(64) NOT NULL UNIQUE,
    request_snapshot_ref VARCHAR(512) NOT NULL,
    request_snapshot_sha256 CHAR(64) NOT NULL,
    status_code VARCHAR(24) NOT NULL,
    available_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    next_query_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    submitted_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    failure_code VARCHAR(128) NOT NULL DEFAULT '',
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (flow_no, contract_no),
    UNIQUE (contract_no, sequence_no),
    FOREIGN KEY (contract_no) REFERENCES plugin_idc_compliance_contract (contract_no)
);

CREATE OR REPLACE FUNCTION plugin_idc_compliance_reject_append_only_mutation()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'compliance history is append-only';
END;
$$;`

const preparationProviderFixtures = `
INSERT INTO plugin_idc_compliance_contract_template
    (tenant_id, template_no, origin_code, provider_code, provider_template_ref,
     provider_state_code, provider_last_seen_sync_no, provider_updated_at)
VALUES
    (0, 'TPL_001', 'tencent_synced', 'tencent_essbasic',
     '0123456789ABCDEF0123456789ABCDEF', 'active', 'sync_required', CURRENT_TIMESTAMP),
    (0, 'ctpl_0123456789abcdef0123456789abcdef', 'tencent_synced', 'tencent_essbasic',
     'FEDCBA9876543210FEDCBA9876543210', 'active', 'sync_optional', CURRENT_TIMESTAMP);

INSERT INTO plugin_idc_compliance_contract_template_version
    (tenant_id, template_no, version_no, name, scope_code, signing_order_code,
     language_code, status_code, provider_version, provider_snapshot_digest,
     current_provider_mapping_set_no)
VALUES
    (0, 'TPL_001', 2, '服务器服务合同', 'personal', 'customer_first', 'zh-CN',
     'enabled', '20260815001', repeat('1', 64), 'MAP_REQUIRED'),
    (0, 'ctpl_0123456789abcdef0123456789abcdef', 5, 'Optional service contract',
     'personal', 'customer_first', 'en-US', 'enabled', '20260815002', repeat('2', 64),
     'MAP_OPTIONAL');

INSERT INTO plugin_idc_compliance_ctpl_provider_component
    (tenant_id, template_no, version_no, component_ref, name, type_code, required, recipient_ref)
SELECT 0, template_no, version_no, component_ref, name, type_code, TRUE, recipient_ref
FROM (VALUES
    ('TPL_001', 2::BIGINT, 'contract-number', '合同编号', 'TEXT', NULL::VARCHAR),
    ('TPL_001', 2::BIGINT, 'service-lines', '服务明细', 'DYNAMIC_TABLE', NULL::VARCHAR),
    ('TPL_001', 2::BIGINT, 'signing-date', '签署日期', 'SIGN_DATE', 'customer'::VARCHAR),
    ('ctpl_0123456789abcdef0123456789abcdef', 5::BIGINT, 'contract-number', 'Contract number', 'TEXT', NULL::VARCHAR),
    ('ctpl_0123456789abcdef0123456789abcdef', 5::BIGINT, 'service-lines', 'Service details', 'DYNAMIC_TABLE', NULL::VARCHAR),
    ('ctpl_0123456789abcdef0123456789abcdef', 5::BIGINT, 'signing-date', 'Signing date', 'SIGN_DATE', 'customer'::VARCHAR)
) AS fixture(template_no, version_no, component_ref, name, type_code, recipient_ref);

INSERT INTO plugin_idc_compliance_ctpl_provider_recipient
    (tenant_id, template_no, version_no, recipient_ref, sequence_no, type_code,
     role_name, require_sign, routing_order, is_promoter, sign_component_count)
SELECT 0, template_no, version_no, recipient_ref, sequence_no, type_code,
       role_name, TRUE, routing_order, is_promoter, 1
FROM (VALUES
    ('TPL_001', 2::BIGINT, 'customer', 1, 'INDIVIDUAL', 'Customer', 1::BIGINT, FALSE),
    ('TPL_001', 2::BIGINT, 'merchant', 2, 'PROMOTER', 'Merchant', 2::BIGINT, TRUE),
    ('ctpl_0123456789abcdef0123456789abcdef', 5::BIGINT, 'customer', 1, 'INDIVIDUAL', 'Customer', 1::BIGINT, FALSE),
    ('ctpl_0123456789abcdef0123456789abcdef', 5::BIGINT, 'merchant', 2, 'PROMOTER', 'Merchant', 2::BIGINT, TRUE)
) AS fixture(template_no, version_no, recipient_ref, sequence_no, type_code, role_name, routing_order, is_promoter);

INSERT INTO plugin_idc_compliance_ctpl_provider_mapping_set
    (tenant_id, mapping_set_no, template_no, version_no, provider_snapshot_digest,
     command_idempotency_digest, command_fingerprint, created_by_user_id)
VALUES
    (0, 'MAP_REQUIRED', 'TPL_001', 2, repeat('1', 64), repeat('3', 64), repeat('4', 64), 1),
    (0, 'MAP_OPTIONAL', 'ctpl_0123456789abcdef0123456789abcdef', 5,
     repeat('2', 64), repeat('5', 64), repeat('6', 64), 1);

INSERT INTO plugin_idc_compliance_ctpl_provider_field_mapping
    (tenant_id, mapping_set_no, template_no, version_no, component_ref, value_code)
SELECT 0, mapping_set_no, template_no, version_no, component_ref, value_code
FROM (VALUES
    ('MAP_REQUIRED', 'TPL_001', 2::BIGINT, 'contract-number', 'contract_no'),
    ('MAP_REQUIRED', 'TPL_001', 2::BIGINT, 'service-lines', 'service_lines'),
    ('MAP_REQUIRED', 'TPL_001', 2::BIGINT, 'signing-date', 'signing_date'),
    ('MAP_OPTIONAL', 'ctpl_0123456789abcdef0123456789abcdef', 5::BIGINT, 'contract-number', 'contract_no'),
    ('MAP_OPTIONAL', 'ctpl_0123456789abcdef0123456789abcdef', 5::BIGINT, 'service-lines', 'service_lines'),
    ('MAP_OPTIONAL', 'ctpl_0123456789abcdef0123456789abcdef', 5::BIGINT, 'signing-date', 'signing_date')
) AS fixture(mapping_set_no, template_no, version_no, component_ref, value_code);

INSERT INTO plugin_idc_compliance_ctpl_provider_recipient_mapping
    (tenant_id, mapping_set_no, template_no, version_no, recipient_ref, party_code)
SELECT 0, mapping_set_no, template_no, version_no, recipient_ref, party_code
FROM (VALUES
    ('MAP_REQUIRED', 'TPL_001', 2::BIGINT, 'customer', 'customer'),
    ('MAP_REQUIRED', 'TPL_001', 2::BIGINT, 'merchant', 'merchant'),
    ('MAP_OPTIONAL', 'ctpl_0123456789abcdef0123456789abcdef', 5::BIGINT, 'customer', 'customer'),
    ('MAP_OPTIONAL', 'ctpl_0123456789abcdef0123456789abcdef', 5::BIGINT, 'merchant', 'merchant')
) AS fixture(mapping_set_no, template_no, version_no, recipient_ref, party_code);`
