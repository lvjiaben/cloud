package admin

import (
	"lina-plugin-idc-core/backend/internal/service/currency"
	"reflect"

	"lina-plugin-idc-core/backend/internal/service/audit"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/catalogdraft"
	"lina-plugin-idc-core/backend/internal/service/commerce"
	"lina-plugin-idc-core/backend/internal/service/contractpolicy"
	"lina-plugin-idc-core/backend/internal/service/customer"
	"lina-plugin-idc-core/backend/internal/service/customermanagement"
	"lina-plugin-idc-core/backend/internal/service/financeops"
	"lina-plugin-idc-core/backend/internal/service/marketing"
	"lina-plugin-idc-core/backend/internal/service/operations"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
	"lina-plugin-idc-core/backend/internal/service/productfulfillment"
	"lina-plugin-idc-core/backend/internal/service/saleschannel"
	"lina-plugin-idc-core/backend/internal/service/salesmanagement"
	"lina-plugin-idc-core/backend/internal/service/serveroperation"
)

type ControllerV1 struct {
	currencies                currency.Service
	customerAudit             audit.Reader
	customerManagement        customermanagement.Service
	customerMutation          customermanagement.MutationService
	customerAccountLifecycle  customer.AccountLifecycleService
	catalogManagement         catalog.ProductManagementReader
	catalogWriter             catalog.ManagementService
	catalogImport             catalogdraft.Service
	catalogCopy               catalogdraft.CopyService
	catalogPublication        catalog.PublicationService
	catalogPublicationPreview catalog.PublicationPreviewService
	marketingOverview         marketing.ManagementOverviewReader
	marketingDirectory        marketing.ManagementDirectoryReader
	marketingAssetDirectory   marketing.AssetDirectoryReader
	marketingActions          marketing.ManagementActionService
	commerceRiskReview        commerce.OrderRiskReviewService
	commerceOrders            commerce.ManagementOrderReader
	commerceAssistedQuotes    commerce.AssistedQuoteService
	finance                   billing.ManagementFinanceReader
	financeCommands           billing.ManagementFinanceCommandService
	credit                    billing.ManagementCreditReader
	creditCommands            billing.ManagementCreditCommandService
	contractPolicies          contractpolicy.ManagementService
	financialOperations       financeops.Service
	refunds                   billing.ManagementRefundService
	marketingInsights         marketing.MarketingInsightsService
	marketingCouponBatches    marketing.CouponBatchService
	serverOperationPolicy     serveroperation.PolicyManagement
	accessoryPricing          serveroperation.AccessoryPricing
	serverAccessoryManagement serveroperation.ManagementService
	serverBatches             serveroperation.BatchManagement
	productAutomation         productautomation.Service
	productFulfillment        productfulfillment.ManagementService
	salesChannel              saleschannel.Service
	salesManagement           salesmanagement.Service
	operations                operations.Service
}

func NewV1(
	customerManagement customermanagement.Service,
	customerMutation customermanagement.MutationService,
	catalogManagement catalog.ProductManagementReader,
	catalogWriter catalog.ManagementService,
	catalogPublication catalog.PublicationService,
	catalogPublicationPreview catalog.PublicationPreviewService,
	marketingOverview marketing.ManagementOverviewReader,
	marketingDirectory marketing.ManagementDirectoryReader,
	marketingAssetDirectory marketing.AssetDirectoryReader,
	marketingActions marketing.ManagementActionService,
	commerceRiskReview commerce.OrderRiskReviewService,
	commerceOrders commerce.ManagementOrderReader,
	commerceAssistedQuotes commerce.AssistedQuoteService,
	finance billing.ManagementFinanceReader,
	financeCommands billing.ManagementFinanceCommandService,
	credit billing.ManagementCreditReader,
	creditCommands billing.ManagementCreditCommandService,
) (*ControllerV1, error) {
	dependencies := []any{customerManagement, customerMutation, catalogManagement, catalogWriter, catalogPublication, catalogPublicationPreview, marketingOverview, marketingDirectory, marketingAssetDirectory, marketingActions, commerceRiskReview, commerceOrders, commerceAssistedQuotes, finance, financeCommands, credit, creditCommands}
	for _, dependency := range dependencies {
		if nilControllerDependency(dependency) {
			return nil, gerror.New("IDC admin controller requires all runtime services")
		}
	}
	return &ControllerV1{customerManagement: customerManagement, customerMutation: customerMutation, catalogManagement: catalogManagement, catalogWriter: catalogWriter, catalogPublication: catalogPublication, catalogPublicationPreview: catalogPublicationPreview, marketingOverview: marketingOverview, marketingDirectory: marketingDirectory, marketingAssetDirectory: marketingAssetDirectory, marketingActions: marketingActions, commerceRiskReview: commerceRiskReview, commerceOrders: commerceOrders, commerceAssistedQuotes: commerceAssistedQuotes, finance: finance, financeCommands: financeCommands, credit: credit, creditCommands: creditCommands}, nil
}

func BindContractPolicies(controller *ControllerV1, service contractpolicy.ManagementService) error {
	return bindAdminDependency(controller, service, controller.contractPolicies != nil, func() { controller.contractPolicies = service })
}

// BindCatalogCopy attaches the existing owner services before route binding.
func BindCatalogCopy(controller *ControllerV1, service catalogdraft.CopyService) error {
	return bindAdminDependency(controller, service, controller.catalogCopy != nil, func() { controller.catalogCopy = service })
}

// BindCatalogImport attaches the startup-created owner import orchestrator.
func BindCatalogImport(controller *ControllerV1, service catalogdraft.Service) error {
	return bindAdminDependency(controller, service, controller.catalogImport != nil, func() { controller.catalogImport = service })
}

func BindCustomerAccountLifecycle(controller *ControllerV1, service customer.AccountLifecycleService) error {
	return bindAdminDependency(controller, service, controller.customerAccountLifecycle != nil, func() { controller.customerAccountLifecycle = service })
}
func BindFinancialOperations(controller *ControllerV1, service financeops.Service) error {
	return bindAdminDependency(controller, service, controller.financialOperations != nil, func() { controller.financialOperations = service })
}
func BindRefunds(controller *ControllerV1, service billing.ManagementRefundService) error {
	return bindAdminDependency(controller, service, controller.refunds != nil, func() { controller.refunds = service })
}
func BindMarketingInsights(controller *ControllerV1, service marketing.MarketingInsightsService) error {
	return bindAdminDependency(controller, service, controller.marketingInsights != nil, func() { controller.marketingInsights = service })
}
func BindMarketingCouponBatches(controller *ControllerV1, service marketing.CouponBatchService) error {
	return bindAdminDependency(controller, service, controller.marketingCouponBatches != nil, func() { controller.marketingCouponBatches = service })
}
func BindServerOperationPolicy(controller *ControllerV1, service serveroperation.PolicyManagement) error {
	return bindAdminDependency(controller, service, controller.serverOperationPolicy != nil, func() { controller.serverOperationPolicy = service })
}
func BindAccessoryPricing(controller *ControllerV1, service serveroperation.AccessoryPricing) error {
	return bindAdminDependency(controller, service, controller.accessoryPricing != nil, func() { controller.accessoryPricing = service })
}
func BindServerAccessoryManagement(controller *ControllerV1, service serveroperation.ManagementService) error {
	return bindAdminDependency(controller, service, controller.serverAccessoryManagement != nil, func() { controller.serverAccessoryManagement = service })
}
func BindServerBatches(controller *ControllerV1, service serveroperation.BatchManagement) error {
	return bindAdminDependency(controller, service, controller.serverBatches != nil, func() { controller.serverBatches = service })
}
func BindProductAutomation(controller *ControllerV1, service productautomation.Service) error {
	return bindAdminDependency(controller, service, controller.productAutomation != nil, func() { controller.productAutomation = service })
}

func BindProductFulfillment(controller *ControllerV1, service productfulfillment.ManagementService) error {
	return bindAdminDependency(controller, service, controller.productFulfillment != nil, func() { controller.productFulfillment = service })
}

func BindSalesChannel(controller *ControllerV1, service saleschannel.Service) error {
	return bindAdminDependency(controller, service, controller.salesChannel != nil, func() { controller.salesChannel = service })
}

func BindOperations(controller *ControllerV1, service operations.Service) error {
	return bindAdminDependency(controller, service, controller.operations != nil, func() { controller.operations = service })
}

func bindAdminDependency(controller *ControllerV1, service any, alreadyBound bool, assign func()) error {
	if controller == nil || nilControllerDependency(service) || alreadyBound {
		return gerror.New("IDC admin controller dependency must be bound exactly once")
	}
	assign()
	return nil
}

func nilControllerDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

// BindSalesManagement binds the shared staff management owner.
func BindSalesManagement(controller *ControllerV1, service salesmanagement.Service) error {
	return bindAdminDependency(controller, service, controller.salesManagement != nil, func() { controller.salesManagement = service })
}

// BindCustomerAudit binds the shared startup audit reader; missing and duplicate bindings fail.
func BindCustomerAudit(controller *ControllerV1, reader audit.Reader) error {
	if controller == nil {
		return gerror.New("customer audit binding requires controller")
	}
	return bindAdminDependency(controller, reader, controller.customerAudit != nil, func() { controller.customerAudit = reader })
}

// BindCurrencies attaches the startup-owned currency management service.
func BindCurrencies(c *ControllerV1, svc currency.Service) error {
	return bindAdminDependency(c, svc, c.currencies != nil, func() { c.currencies = svc })
}
