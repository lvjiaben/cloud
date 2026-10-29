// Package account exposes authenticated customer commerce endpoints.
package account

import (
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/commerce"
	"lina-plugin-idc-core/backend/internal/service/invoiceprofile"
	"lina-plugin-idc-core/backend/internal/service/marketing"
	"lina-plugin-idc-core/backend/internal/service/purchaseconfigurationfile"
	"lina-plugin-idc-core/backend/internal/service/purchaseserverlogin"
	"lina-plugin-idc-core/backend/internal/service/renewal"
	"lina-plugin-idc-core/backend/internal/service/serveroperation"
	"lina-plugin-idc-core/backend/internal/service/sshkey"
)

type ControllerV1 struct {
	commerce              commerce.Service
	paymentPreparation    commerce.PaymentPreparationService
	orders                commerce.AccountOrderReader
	finance               billing.AccountFinanceReader
	financeCommands       billing.AccountFinanceCommandService
	refunds               billing.AccountRefundService
	credit                billing.AccountCreditReader
	creditCommands        billing.AccountCreditCommandService
	renewals              renewal.Service
	sshKeys               sshkey.Service
	invoiceProfiles       invoiceprofile.Service
	purchaseFiles         purchaseconfigurationfile.Service
	serverLogins          purchaseserverlogin.Service
	servers               serveroperation.Directory
	serverOperations      serveroperation.AccountService
	accessoryPricing      serveroperation.AccessoryPricing
	purchaseConfiguration catalog.PublicStructuredConfigurationReader
	coupons               marketing.CouponClaimService
}

func NewV1(
	commerceService commerce.Service,
	paymentPreparation commerce.PaymentPreparationService,
	orders commerce.AccountOrderReader,
	finance billing.AccountFinanceReader,
	financeCommands billing.AccountFinanceCommandService,
	refunds billing.AccountRefundService,
	credit billing.AccountCreditReader,
	creditCommands billing.AccountCreditCommandService,
	renewals renewal.Service,
	profiles invoiceprofile.Service,
	servers serveroperation.Directory,
	serverOperations serveroperation.AccountService,
	accessoryPricing serveroperation.AccessoryPricing,
	keys sshkey.Service,
) (*ControllerV1, error) {
	if commerceService == nil || paymentPreparation == nil || orders == nil ||
		finance == nil || financeCommands == nil || refunds == nil ||
		credit == nil || creditCommands == nil || renewals == nil ||
		profiles == nil || servers == nil || serverOperations == nil ||
		accessoryPricing == nil || keys == nil {
		return nil, gerror.New("account controller requires all runtime services")
	}
	return &ControllerV1{
		commerce:           commerceService,
		paymentPreparation: paymentPreparation,
		orders:             orders,
		finance:            finance,
		financeCommands:    financeCommands,
		refunds:            refunds,
		credit:             credit,
		creditCommands:     creditCommands,
		renewals:           renewals,
		invoiceProfiles:    profiles,
		sshKeys:            keys,
		servers:            servers,
		serverOperations:   serverOperations,
		accessoryPricing:   accessoryPricing,
	}, nil
}

func BindCouponClaims(controller *ControllerV1, coupons marketing.CouponClaimService) error {
	if controller == nil || coupons == nil {
		return gerror.New("account controller coupon binding requires both parties")
	}
	if controller.coupons != nil {
		return gerror.New("account controller coupon service is already bound")
	}
	controller.coupons = coupons
	return nil
}

func BindPurchaseConfigurationFiles(controller *ControllerV1, files purchaseconfigurationfile.Service, purchaseConfiguration catalog.PublicStructuredConfigurationReader) error {
	if controller == nil || files == nil || purchaseConfiguration == nil {
		return gerror.New("account controller purchase-configuration binding requires all parties")
	}
	if controller.purchaseFiles != nil || controller.purchaseConfiguration != nil {
		return gerror.New("account controller purchase-configuration services are already bound")
	}
	controller.purchaseFiles = files
	controller.purchaseConfiguration = purchaseConfiguration
	return nil
}

// BindPurchaseServerLogins installs the same registry used by cart creation.
func BindPurchaseServerLogins(controller *ControllerV1, service purchaseserverlogin.Service) error {
	if controller == nil || service == nil || controller.serverLogins != nil {
		return gerror.New("account server login binding requires one unbound controller and service")
	}
	controller.serverLogins = service
	return nil
}
