// This file verifies the four-step Tencent onboarding orchestration and safe projections.

package electroniccontractonboarding

import (
	"context"
	"testing"

	"lina-plugin-idc-compliance/backend/internal/localgatewayclient"
	"lina-plugin-idc-compliance/backend/internal/service/supplierconfig"
	"lina-plugin-idc-core/backend/cap/idccap"
)

type merchantStub struct{ current idccap.MerchantContext }

func (s merchantStub) CurrentMerchant(context.Context) (idccap.MerchantContext, error) {
	return s.current, nil
}

type supplierStub struct {
	supplierconfig.Service
	configuration *supplierconfig.Configuration
	started       int
	recorded      supplierconfig.GatewayResult
}

func (s *supplierStub) Get(context.Context, string) (*supplierconfig.Configuration, error) {
	return s.configuration, nil
}

func (s *supplierStub) StartSubjectBinding(context.Context, string) error {
	s.started++
	return nil
}

func (s *supplierStub) RecordManagedBinding(
	_ context.Context,
	_ string,
	result supplierconfig.GatewayResult,
) (*supplierconfig.Configuration, error) {
	s.recorded = result
	s.configuration = &supplierconfig.Configuration{
		ProviderCode: providerCode, SubjectStatus: supplierconfig.SubjectActive,
		SubjectBindingNo: result.BindingNo,
	}
	return s.configuration, nil
}

type gatewayStub struct {
	status      localgatewayclient.TencentOnboardingResult
	link        localgatewayclient.TencentOnboardingResult
	statusCall  int
	linkCall    int
	recheckCall int
}

func (s *gatewayStub) TencentOnboardingStatus(context.Context) (localgatewayclient.TencentOnboardingResult, error) {
	s.statusCall++
	return s.status, nil
}

func (s *gatewayStub) CreateTencentOnboardingLink(
	context.Context,
	string,
	string,
) (localgatewayclient.TencentOnboardingResult, error) {
	s.linkCall++
	return s.link, nil
}

func (s *gatewayStub) RecheckTencentOnboarding(
	context.Context,
	string,
	string,
) (localgatewayclient.TencentOnboardingResult, error) {
	s.recheckCall++
	return s.status, nil
}

func completeMerchant() merchantStub {
	return merchantStub{current: idccap.MerchantContext{
		TenantID: idccap.DefaultTenantID, MerchantNo: "merchant-local",
		LegalName: "示例 IDC 有限公司", CountryCode: "CN",
		DefaultLocale: "zh-CN", DefaultCurrency: "CNY", Timezone: "Asia/Shanghai",
		Enabled: true, Version: 1,
	}}
}

func TestCreateLinkReturnsOfficialURLAndRecordsOnlyOpaqueVerifiedBinding(t *testing.T) {
	suppliers := &supplierStub{configuration: &supplierconfig.Configuration{
		ProviderCode: providerCode, SubjectStatus: supplierconfig.SubjectUnbound,
	}}
	localGateway := &gatewayStub{link: localgatewayclient.TencentOnboardingResult{
		State: "active", ConsoleURL: "https://qian.tencent.com/one-use",
		OrganizationReady: true, OperatorReady: true,
		BindingNo:   "lsb_0123456789abcdef0123456789abcdef",
		OperationNo: "lsi_0123456789abcdef0123456789abcdef",
	}}
	service, err := New(completeMerchant(), suppliers, localGateway)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	view, err := service.CreateLink(context.Background(), "onboarding-request-0001")
	if err != nil {
		t.Fatalf("CreateLink returned error: %v", err)
	}
	if suppliers.started != 1 || localGateway.linkCall != 1 ||
		view.ConsoleURL != "https://qian.tencent.com/one-use" ||
		view.Merchant.LegalName != "示例 IDC 有限公司" ||
		suppliers.recorded.BindingNo != "lsb_0123456789abcdef0123456789abcdef" ||
		view.SubjectStatus != string(supplierconfig.SubjectActive) {
		t.Fatalf("unexpected onboarding projection: view=%+v suppliers=%+v gateway=%+v", view, suppliers, localGateway)
	}
}

func TestRecheckReadsStatusWithoutCreatingAnotherConsoleURL(t *testing.T) {
	suppliers := &supplierStub{configuration: &supplierconfig.Configuration{
		ProviderCode: providerCode, SubjectStatus: supplierconfig.SubjectUnbound,
	}}
	localGateway := &gatewayStub{status: localgatewayclient.TencentOnboardingResult{
		State: "enterprise_pending", ConsoleURL: "https://qian.tencent.com/unused",
	}}
	service, err := New(completeMerchant(), suppliers, localGateway)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	view, err := service.Recheck(context.Background(), "recheck-request-0001")
	if err != nil || view.ConsoleURL != "" || view.State != "enterprise_pending" ||
		localGateway.recheckCall != 1 || localGateway.statusCall != 0 ||
		localGateway.linkCall != 0 || suppliers.started != 0 {
		t.Fatalf("recheck projection=%+v err=%v", view, err)
	}
}

func TestIncompleteMerchantProfileFailsClosedWithoutCallingTencent(t *testing.T) {
	merchant := completeMerchant()
	merchant.current.LegalName = ""
	suppliers := &supplierStub{configuration: &supplierconfig.Configuration{
		ProviderCode: providerCode, SubjectStatus: supplierconfig.SubjectUnbound,
	}}
	localGateway := &gatewayStub{}
	service, err := New(merchant, suppliers, localGateway)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	view, err := service.CreateLink(context.Background(), "onboarding-request-0002")
	if err != nil || view.State != "not_configured" ||
		view.ReasonCode != "merchant_profile_incomplete" ||
		suppliers.started != 0 || localGateway.linkCall != 0 {
		t.Fatalf("blocked projection=%+v suppliers=%+v gateway=%+v err=%v", view, suppliers, localGateway, err)
	}
}
