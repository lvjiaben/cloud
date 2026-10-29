// This file injects native server management through its resource owner.
package accountnative

import (
	"context"
	"errors"
	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/customerauthcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-resource/backend/api/accountnative"
	"lina-plugin-idc-resource/backend/internal/service/customeroperations"
	"lina-plugin-idc-resource/backend/internal/service/serverfacts"
)

type operations interface {
	Prepare(context.Context, customeroperations.Command) (*customerauthcap.SessionConfirmation, error)
	Options(context.Context, string) (*customeroperations.Options, error)
	GetOperation(context.Context, string, string) (*customeroperations.Operation, error)
	Operations(context.Context, string, int, int, string) (*customeroperations.OperationPage, error)
	RevealOutput(context.Context, string, string, string, bool, secretcap.Consumer) error
	Facts(context.Context, []string) ([]serverfacts.Fact, error)
	List(context.Context, int, int) (*customeroperations.Page, error)
	Get(context.Context, string) (*customeroperations.Detail, error)
	Resolve(context.Context, string) (string, error)
	Create(context.Context, customeroperations.Command) (*customeroperations.Operation, error)
	Reveal(context.Context, string, bool, secretcap.Consumer) error
}

type nativeCommerce interface {
	CommercialOperations(context.Context, customeroperations.CommercialHistoryQuery) (*servercommercialcap.OperationPage, error)
	AccessoryBilling(context.Context, customeroperations.AccessoryBillingQuery) (*servercommercialcap.AccessoryBillingPage, error)
	Configuration(context.Context, customeroperations.ConfigurationCommand) (*servercommercialcap.ConfigurationContext, error)
	QuoteTarget(context.Context, customeroperations.ConfigurationCommand) (*servercommercialcap.TargetQuote, error)
	Quote(context.Context, customeroperations.CommercialQuoteCommand) (*servercommercialcap.Quote, error)
	PrepareCheckout(context.Context, customeroperations.CommercialCheckoutCommand) (*customerauthcap.SessionConfirmation, error)
	Checkout(context.Context, customeroperations.CommercialCheckoutCommand) (*servercommercialcap.Purchase, error)
	TrafficPackages(context.Context, string) ([]servercommercialcap.TrafficPackage, error)
}

type ControllerV1 struct {
	service    operations
	commercial nativeCommerce
}

func NewV1(service operations, commercial nativeCommerce) (accountnative.IAccountnativeV1, error) {
	if service == nil || commercial == nil {
		return nil, customeroperations.ErrUnavailable
	}
	return &ControllerV1{service: service, commercial: commercial}, nil
}

func publicCommercialError(err error) error {
	if _, ok := bizerr.As(err); ok {
		return err
	}
	return publicError(err)
}

var (
	codeConfigurationTimeout = bizerr.MustDefine("IDC_RESOURCE_CUSTOMER_CONFIGURATION_TIMEOUT", "Server configuration could not be loaded in time; retry shortly", gcode.CodeBusinessValidationFailed)
	codeUnavailable          = bizerr.MustDefine("IDC_RESOURCE_CUSTOMER_OPERATION_UNAVAILABLE", "The customer server is unavailable", gcode.CodeNotAuthorized)
	codeConflict             = bizerr.MustDefine("IDC_RESOURCE_CUSTOMER_OPERATION_CONFLICT", "The server state changed or another operation is unfinished; refresh before retrying", gcode.CodeBusinessValidationFailed)
	codeInvalid              = bizerr.MustDefine("IDC_RESOURCE_CUSTOMER_OPERATION_INVALID", "The customer server operation requires valid input and confirmation", gcode.CodeInvalidParameter)
)

func publicError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, customeroperations.ErrConfigurationTimeout) {
		return bizerr.NewCode(codeConfigurationTimeout)
	}
	if errors.Is(err, customeroperations.ErrConflict) {
		return bizerr.NewCode(codeConflict)
	}
	if errors.Is(err, customeroperations.ErrInvalid) {
		return bizerr.NewCode(codeInvalid)
	}
	return bizerr.NewCode(codeUnavailable)
}
