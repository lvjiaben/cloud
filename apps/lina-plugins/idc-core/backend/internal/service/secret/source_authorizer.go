// This file implements the fail-closed source-consumer scope policy.

package secret

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/cap/supportcap"
)

const (
	coreOwnerPluginID     = supportcap.OwnerPluginID
	coreSupportLinkScope  = supportcap.AnonymousAccessSecretScope
	resourceOwnerPluginID = "idc-resource"
	resourceChannelPrefix = "resource.channel.c"
	notifyOwnerPluginID   = "idc-notify"
	notifyChannelPrefix   = "notify.channel."
	notifyVerifyPrefix    = "notify.verification.c"
	paymentOwnerPluginID  = "idc-payment"
	paymentChannelScope   = "payment.channel-credential"
	paymentChannelPurpose = "payment.channel-use"
)

var notifySecretChannels = map[string]struct{}{
	"in_app":          {},
	"email":           {},
	"sms":             {},
	"wechat-official": {},
	"wechat-work":     {},
	"signed-webhook":  {},
}

var notifySecretPurposes = map[string]struct{}{
	"notify.send":     {},
	"notify.test":     {},
	"notify.query":    {},
	"notify.validate": {},
}

// SourceAuthorizer permits only exact secret scopes composed for trusted source
// plugins. Each scope keeps its own lifecycle and callback-purpose whitelist.
type SourceAuthorizer struct{}

// Authorize validates the exact structural grant attached by trusted startup
// composition. It never derives authority from a client request or role name.
func (SourceAuthorizer) Authorize(ctx context.Context, request AuthorizationRequest) error {
	if ctx == nil || ctx.Err() != nil ||
		request.Scope.TenantID != idccap.DefaultTenantID ||
		!allowedSourceRequest(request) {
		return gerror.New("secret source access is not authorized")
	}
	return nil
}

func allowedSourceRequest(request AuthorizationRequest) bool {
	switch request.Scope.OwnerPluginID {
	case coreOwnerPluginID:
		if request.Scope.CallerScope == secretcap.CoreProductAutomationInterfaceScope {
			return allowedCoreProductAutomationInterfaceAction(request)
		}
		if request.Scope.CallerScope == secretcap.CoreInvoiceProfileBankAccountScope {
			return allowedCoreInvoiceProfileAction(request)
		}
		if request.Scope.CallerScope == secretcap.CoreSupportUpstreamChannelScope {
			return allowedCoreSupportUpstreamAction(request)
		}
		return request.Scope.CallerScope == coreSupportLinkScope && allowedCoreSupportLinkAction(request)
	case resourceOwnerPluginID:
		return validResourceChannelScope(request.Scope.CallerScope) &&
			allowedResourceAction(request)
	case notifyOwnerPluginID:
		if validNotifyVerificationScope(request.Scope.CallerScope) {
			return allowedNotifyVerificationAction(request)
		}
		return validNotifyChannelScope(request.Scope.CallerScope) && allowedNotifyAction(request)
	case paymentOwnerPluginID:
		return request.Scope.CallerScope == paymentChannelScope && allowedPaymentChannelAction(request)
	default:
		return false
	}
}

func allowedCoreProductAutomationInterfaceAction(request AuthorizationRequest) bool {
	switch request.Action {
	case ActionCreate, ActionRecoverCreate, ActionUpdate, ActionMetadata, ActionRevoke, ActionRotate:
		return request.Purpose == ""
	case ActionAccess:
		return request.Purpose == "product-automation.invoke" || request.Purpose == "product-automation.validate"
	default:
		return false
	}
}

func allowedCoreSupportUpstreamAction(request AuthorizationRequest) bool {
	switch request.Action {
	case ActionCreate, ActionRecoverCreate, ActionUpdate, ActionMetadata, ActionRevoke, ActionRotate:
		return request.Purpose == ""
	case ActionAccess:
		return request.Purpose == "core.support-upstream-send" || request.Purpose == "core.support-upstream-query"
	default:
		return false
	}
}

func allowedPaymentChannelAction(request AuthorizationRequest) bool {
	switch request.Action {
	case ActionCreate, ActionRecoverCreate, ActionMetadata:
		return request.Purpose == ""
	case ActionAccess:
		return request.Purpose == secretcap.Purpose(paymentChannelPurpose)
	default:
		return false
	}
}

func allowedCoreInvoiceProfileAction(request AuthorizationRequest) bool {
	switch request.Action {
	case ActionCreate, ActionRecoverCreate, ActionMetadata:
		return request.Purpose == ""
	default:
		return false
	}
}

func allowedCoreSupportLinkAction(request AuthorizationRequest) bool {
	switch request.Action {
	case ActionCreate, ActionRecoverCreate:
		return request.Purpose == ""
	case ActionAccess:
		return request.Purpose == "core.support-anonymous-access"
	default:
		return false
	}
}

func allowedResourceAction(request AuthorizationRequest) bool {
	switch request.Action {
	case ActionCreate, ActionRecoverCreate, ActionUpdate, ActionMetadata, ActionRevoke, ActionRotate:
		return request.Purpose == ""
	case ActionAccess:
		return allowedResourcePurpose(string(request.Purpose))
	default:
		return false
	}
}

func validResourceChannelScope(value string) bool {
	if len(value) != len(resourceChannelPrefix)+32 || !strings.HasPrefix(value, resourceChannelPrefix) {
		return false
	}
	for _, character := range []byte(value[len(resourceChannelPrefix):]) {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func allowedResourcePurpose(value string) bool {
	return strings.HasPrefix(value, "provider.") || strings.HasPrefix(value, "resource.")
}

func allowedNotifyAction(request AuthorizationRequest) bool {
	switch request.Action {
	case ActionCreate, ActionRecoverCreate, ActionUpdate, ActionMetadata, ActionRevoke, ActionRotate:
		return request.Purpose == ""
	case ActionAccess:
		_, allowed := notifySecretPurposes[string(request.Purpose)]
		return allowed
	default:
		return false
	}
}

func allowedNotifyVerificationAction(request AuthorizationRequest) bool {
	switch request.Action {
	case ActionCreate, ActionRecoverCreate, ActionMetadata, ActionRotate:
		return request.Purpose == ""
	case ActionAccess:
		return request.Purpose == "notify.verify"
	default:
		return false
	}
}

func validNotifyVerificationScope(value string) bool {
	if len(value) != len(notifyVerifyPrefix)+32 || !strings.HasPrefix(value, notifyVerifyPrefix) {
		return false
	}
	for _, character := range []byte(value[len(notifyVerifyPrefix):]) {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func validNotifyChannelScope(value string) bool {
	if !strings.HasPrefix(value, notifyChannelPrefix) {
		return false
	}
	remainder := value[len(notifyChannelPrefix):]
	separator := strings.LastIndex(remainder, ".c")
	if separator <= 0 || separator+34 != len(remainder) {
		return false
	}
	if _, allowed := notifySecretChannels[remainder[:separator]]; !allowed {
		return false
	}
	for _, character := range []byte(remainder[separator+2:]) {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}
