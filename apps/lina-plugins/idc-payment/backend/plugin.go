// Package backend registers the IDC payment source plugin with the host.
package backend

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/plugin/capability"
	"lina-core/pkg/plugin/pluginhost"
	billingspi "lina-plugin-idc-core/backend/cap/billingcap/spi"
	idcpayment "lina-plugin-idc-payment"
	"lina-plugin-idc-payment/backend/cap/paymentcap"
	"lina-plugin-idc-payment/backend/internal/composition"
	accountcontroller "lina-plugin-idc-payment/backend/internal/controller/account"
	admincontroller "lina-plugin-idc-payment/backend/internal/controller/admin"
	callbackcontroller "lina-plugin-idc-payment/backend/internal/controller/callback"
	"lina-plugin-idc-payment/backend/internal/provider/alipay"
	"lina-plugin-idc-payment/backend/internal/provider/epay"
	"lina-plugin-idc-payment/backend/internal/provider/unionpay"
	"lina-plugin-idc-payment/backend/internal/provider/wechatpay"
	"lina-plugin-idc-payment/backend/internal/service/channelconfig"
	"lina-plugin-idc-payment/backend/internal/service/exceptionfund"
	"lina-plugin-idc-payment/backend/internal/service/paymentattempt"
	"lina-plugin-idc-payment/backend/internal/service/paymentflow"
	"lina-plugin-idc-payment/backend/internal/service/reconciliation"
)

// pluginID is the immutable source-plugin identifier.
const pluginID = "idc-payment"

// init registers the embedded plugin whose owner dependency is declared in its manifest.
func init() {
	plugin := pluginhost.NewDeclarations(pluginID)
	plugin.Assets().UseEmbeddedFiles(idcpayment.EmbeddedFiles)
	if err := plugin.BusinessExtensions().RegisterProviderValidator(
		paymentcap.DynamicProviderPoint,
		paymentcap.DynamicProviderContractVersion,
		paymentcap.ValidateDynamicProviderDeclarations,
	); err != nil {
		panic(err)
	}
	if err := plugin.HTTP().RegisterRoutes(pluginhost.ExtensionPointHTTPRouteRegister, pluginhost.CallbackExecutionModeBlocking, registerRoutes); err != nil {
		panic(err)
	}
	if err := plugin.Jobs().RegisterJobs(pluginhost.ExtensionPointJobsRegister, pluginhost.CallbackExecutionModeBlocking, registerJobs); err != nil {
		panic(err)
	}
	if err := pluginhost.RegisterSourcePlugin(plugin); err != nil {
		panic(err)
	}
}

func registerRoutes(ctx context.Context, registrar pluginhost.HTTPRegistrar) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil || registrar.Services() == nil {
		return gerror.New("idc-payment routes require host services")
	}
	runtime, err := buildRuntime(ctx, registrar.Services(), registrar.BusinessExtensions())
	if err != nil {
		return err
	}
	routes := registrar.Routes()
	middlewares := routes.Middlewares()
	routes.Group(routes.APIPrefix(), func(group pluginhost.RouteGroup) {
		group.Group("/api/v1", func(group pluginhost.RouteGroup) {
			group.Group("", func(group pluginhost.RouteGroup) {
				group.Middleware(middlewares.NeverDoneCtx(), middlewares.CORS(), middlewares.RequestBodyLimit())
				group.Bind(callbackcontroller.NewV1(runtime.flow))
			})
			group.Group("", func(group pluginhost.RouteGroup) {
				group.Middleware(middlewares.NeverDoneCtx(), middlewares.HandlerResponse(), middlewares.CORS(), middlewares.RequestBodyLimit(), middlewares.Ctx(), middlewares.Auth(), middlewares.Tenancy())
				group.Bind(accountcontroller.NewV1(runtime.flow))
			})
			group.Group("", func(group pluginhost.RouteGroup) {
				group.Middleware(middlewares.NeverDoneCtx(), middlewares.HandlerResponse(), middlewares.CORS(), middlewares.RequestBodyLimit(), middlewares.Ctx(), middlewares.Auth(), middlewares.Tenancy(), middlewares.Permission())
				group.Bind(admincontroller.NewV1(runtime.reconciliation, runtime.exceptionFunds, runtime.channels, registrar.Services().BizCtx(), registrar.Services().Files()))
			})
		})
	})
	return routes.Err()
}

func registerJobs(ctx context.Context, registrar pluginhost.JobsRegistrar) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil || registrar.Services() == nil {
		return gerror.New("idc-payment jobs require host services")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	runtime, err := buildRuntime(ctx, registrar.Services(), registrar.BusinessExtensions())
	if err != nil {
		return err
	}
	for _, job := range runtimeJobs(runtime) {
		if err = registrar.AddWithMetadata(ctx, job.schedule, job.id, job.title, job.description, job.run); err != nil {
			return err
		}
	}
	return nil
}

type paymentJob struct {
	schedule, id, title, description string
	run                              func(context.Context) error
}

func runtimeJobs(runtime *paymentRuntime) []paymentJob {
	return []paymentJob{
		{"@every 2s", "idc-payment-confirmation", "IDC Payment Confirmation", "Posts bounded verified payment work to the core financial owner.", boundedPaymentWork(16, runtime.flow.RunOneConfirmation)},
		{"@every 2s", "idc-payment-order-closure", "IDC Payment Order Closure", "Closes unfinished payment attempts before the core releases a cancelled order.", boundedPaymentWork(8, runtime.flow.RunOnePaymentClosure)},
		{"@every 2s", "idc-payment-refund", "IDC Refund Execution", "Preflights every original source before executing bounded core-approved channel refunds.", boundedPaymentWork(8, runtime.flow.RunOneRefund)},
		{"@every 2s", "idc-payment-exception-refund", "IDC Exception Fund Refund", "Executes one verified original-source refund without reviving the cancelled order.", boundedPaymentWork(8, func(ctx context.Context) (bool, error) {
			return runtime.exceptionFunds.RunOneAutomaticRefund(ctx, runtime.channels)
		})},
		{"@every 30s", "idc-payment-query", "IDC Payment Active Query", "Queries at most eight pending payments through verified provider capabilities.", boundedPaymentWork(8, runtime.flow.RunOneQuery)},
		{"@every 1h", "idc-payment-daily-reconciliation", "IDC Payment Daily Reconciliation", "Creates idempotent bounded daily payment and refund reconciliation batches only through verified provider capabilities.", func(jobCtx context.Context) error {
			location, locationErr := time.LoadLocation(runtime.merchantTimezone)
			if locationErr != nil {
				return locationErr
			}
			businessDate := time.Now().In(location).AddDate(0, 0, -1)
			_, runErr := runtime.reconciliation.RunAll(jobCtx, businessDate, runtime.merchantTimezone, 50)
			return runErr
		}},
	}
}

func boundedPaymentWork(limit int, run func(context.Context) (bool, error)) func(context.Context) error {
	return func(ctx context.Context) error {
		for index := 0; index < limit; index++ {
			worked, err := run(ctx)
			if err != nil || !worked {
				return err
			}
		}
		return nil
	}
}

type paymentRuntime struct {
	flow             *paymentflow.Service
	reconciliation   *reconciliation.Service
	exceptionFunds   *exceptionfund.Service
	channels         *channelconfig.Service
	merchantTimezone string
}

func buildRuntime(ctx context.Context, services capability.Services, extensions ...pluginhost.BusinessExtensionRegistrar) (*paymentRuntime, error) {
	snapshot, err := billingspi.Freeze()
	if err != nil {
		return nil, err
	}
	billingFactory := snapshot.Factory()
	if billingFactory == nil {
		return nil, errors.New("payment billing owner is unavailable")
	}
	billing, err := billingFactory(ctx)
	if err != nil {
		return nil, err
	}
	secretSource, err := composition.LoadSecretSource(ctx)
	if err != nil {
		return nil, err
	}
	accessor, manager, err := composition.BindPaymentCredentials(secretSource)
	if err != nil {
		return nil, err
	}
	epayAdapter, err := epay.New("")
	if err != nil {
		return nil, err
	}
	unionPayAdapter, err := unionpay.New("", nil)
	if err != nil {
		return nil, err
	}
	adapters := []paymentcap.Adapter{alipay.New(nil), wechatpay.New(nil), epayAdapter, unionPayAdapter}
	registry := map[paymentcap.ProviderCode]paymentcap.Adapter{}
	for _, adapter := range adapters {
		registry[adapter.Provider()] = adapter
	}
	providerDirectories := make([]channelconfig.ProviderDirectory, 0, len(extensions))
	for _, extension := range extensions {
		if extension != nil {
			providerDirectories = append(providerDirectories, extension)
		}
	}
	channels, err := channelconfig.New(g.DB(), accessor, manager, adapters, func(_ context.Context, provider paymentcap.ProviderCode, credentials paymentcap.Credentials) error {
		validator, ok := registry[provider].(paymentcap.CredentialValidator)
		if !ok {
			return errors.New("payment credential validation is unavailable")
		}
		return validator.ValidateCredentials(credentials)
	}, providerDirectories...)
	if err != nil {
		return nil, err
	}
	attempts, err := paymentattempt.New(g.DB(), billing)
	if err != nil {
		return nil, err
	}
	origin, err := services.HostConfig().String(ctx, "idc.payment.public-origin", "https://localhost.invalid")
	if err != nil {
		return nil, err
	}
	flow, err := paymentflow.New(attempts, channels, paymentflow.Config{PublicOrigin: origin})
	if err != nil {
		return nil, err
	}
	timezone, err := services.HostConfig().String(ctx, "idc.merchant.timezone", "Asia/Shanghai")
	if err != nil {
		return nil, err
	}
	if _, err = time.LoadLocation(timezone); err != nil {
		return nil, errors.New("merchant timezone is invalid")
	}
	source, err := reconciliation.NewProviderSource(g.DB(), channels, &http.Client{Timeout: 30 * time.Second})
	if err != nil {
		return nil, err
	}
	reconciliationService, err := reconciliation.New(g.DB(), channels, attempts, source)
	if err != nil {
		return nil, err
	}
	exceptionFundService, err := exceptionfund.New(g.DB(), billing)
	if err != nil {
		return nil, err
	}
	return &paymentRuntime{flow: flow, reconciliation: reconciliationService, exceptionFunds: exceptionFundService, channels: channels, merchantTimezone: timezone}, nil
}
