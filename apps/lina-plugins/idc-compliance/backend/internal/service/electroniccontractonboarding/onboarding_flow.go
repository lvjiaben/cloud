// This file orchestrates the read, official-link, recheck, and safe projection flow.

package electroniccontractonboarding

import (
	"context"
	"strings"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/internal/localgatewayclient"
	"lina-plugin-idc-compliance/backend/internal/service/supplierconfig"
	"lina-plugin-idc-core/backend/cap/idccap"
)

func (s *serviceImpl) Get(ctx context.Context) (*View, error) {
	merchant, configuration, err := s.loadLocal(ctx)
	if err != nil {
		return nil, err
	}
	status, statusErr := s.gateway.TencentOnboardingStatus(ctx)
	if statusErr != nil {
		status = localgatewayclient.TencentOnboardingResult{
			State: "temporarily_unavailable", ReasonCode: "local_gateway_unavailable",
		}
	}
	return project(merchant, configuration, status, false), nil
}

func (s *serviceImpl) CreateLink(ctx context.Context, idempotencyKey string) (*View, error) {
	return s.refresh(ctx, idempotencyKey, true)
}

func (s *serviceImpl) Recheck(ctx context.Context, idempotencyKey string) (*View, error) {
	return s.refresh(ctx, idempotencyKey, false)
}

func (s *serviceImpl) refresh(
	ctx context.Context,
	idempotencyKey string,
	includeURL bool,
) (*View, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if len(idempotencyKey) < 8 || len(idempotencyKey) > 128 {
		return nil, bizerr.NewCode(supplierconfig.CodeInvalid)
	}
	merchant, configuration, err := s.loadLocal(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(merchant.LegalName) == "" || strings.TrimSpace(merchant.CountryCode) == "" {
		return project(merchant, configuration, localgatewayclient.TencentOnboardingResult{
			State: "not_configured", ReasonCode: "merchant_profile_incomplete",
		}, false), nil
	}
	var status localgatewayclient.TencentOnboardingResult
	if includeURL {
		if err = s.suppliers.StartSubjectBinding(ctx, providerCode); err != nil {
			return nil, err
		}
		status, err = s.gateway.CreateTencentOnboardingLink(ctx, merchant.LegalName, idempotencyKey)
	} else {
		status, err = s.gateway.RecheckTencentOnboarding(ctx, merchant.LegalName, idempotencyKey)
	}
	if err != nil {
		return project(merchant, configuration, localgatewayclient.TencentOnboardingResult{
			State: "temporarily_unavailable", ReasonCode: "tencent_temporarily_unavailable",
		}, false), nil
	}
	if status.State == "active" && status.BindingNo != "" {
		configuration, err = s.suppliers.RecordManagedBinding(ctx, providerCode, supplierconfig.GatewayResult{
			OperationNo: status.OperationNo, BindingNo: status.BindingNo,
		})
		if err != nil {
			return nil, err
		}
	}
	return project(merchant, configuration, status, includeURL), nil
}

func (s *serviceImpl) loadLocal(
	ctx context.Context,
) (idccap.MerchantContext, *supplierconfig.Configuration, error) {
	configuration, err := s.suppliers.Get(ctx, providerCode)
	if err != nil {
		return idccap.MerchantContext{}, nil, err
	}
	merchant, err := s.merchants.CurrentMerchant(ctx)
	if err != nil {
		return idccap.MerchantContext{}, nil, err
	}
	return merchant, configuration, nil
}

func project(
	merchant idccap.MerchantContext,
	configuration *supplierconfig.Configuration,
	status localgatewayclient.TencentOnboardingResult,
	includeURL bool,
) *View {
	state := status.State
	reason := status.ReasonCode
	if strings.TrimSpace(merchant.LegalName) == "" || strings.TrimSpace(merchant.CountryCode) == "" {
		state, reason = "not_configured", "merchant_profile_incomplete"
	}
	view := &View{
		Merchant: Merchant{
			LegalName: merchant.LegalName, CountryCode: merchant.CountryCode,
			MerchantNo: merchant.MerchantNo,
		},
		State: state, ReasonCode: reason,
		OrganizationReady: status.OrganizationReady, OperatorReady: status.OperatorReady,
		BindingNo: status.BindingNo, Capabilities: status.Capabilities,
	}
	if configuration != nil {
		view.SubjectStatus = string(configuration.SubjectStatus)
		if view.BindingNo == "" {
			view.BindingNo = configuration.SubjectBindingNo
		}
		if configuration.SubjectStatus == supplierconfig.SubjectActive && view.State == "ready" {
			view.State = "active"
			view.OrganizationReady, view.OperatorReady = true, true
		}
	}
	if includeURL {
		view.ConsoleURL = status.ConsoleURL
	}
	return view
}
