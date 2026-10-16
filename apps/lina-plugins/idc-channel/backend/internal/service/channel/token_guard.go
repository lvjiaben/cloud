package channel

import (
	"lina-core/pkg/logger"
	"net"
	"strings"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"
	"lina-core/pkg/bizerr"
)

// TokenGuard authenticates the public downstream API without exposing the
// host's staff or customer JWT boundary to channel integrations.
type TokenGuard struct{ service Service }

func NewTokenGuard(service Service) *TokenGuard { return &TokenGuard{service: service} }

func (guard *TokenGuard) Middleware(request *ghttp.Request) {
	if request == nil {
		return
	}
	if guard == nil || guard.service == nil {
		request.SetError(bizerr.NewCode(codeTokenNotAccepted))
		return
	}
	appID, appSecret, ok := request.Request.BasicAuth()
	if !ok || strings.TrimSpace(appID) == "" {
		request.SetError(bizerr.NewCode(codeTokenNotAccepted))
		return
	}
	started := time.Now()
	scope := requiredScopeForRequest(request.Method, request.URL.Path)
	credential, err := guard.service.AuthenticateToken(request.Context(), appSecret, directPeerIP(request.RemoteAddr), scope)
	if credential.TokenID > 0 && credential.ChannelID == appID {
		defer func() {
			call := APICall{ChannelID: credential.ChannelID, Method: request.Method, Path: requiredScopeForRequest(request.Method, request.URL.Path), Succeeded: request.GetError() == nil, DurationMillis: time.Since(started).Milliseconds()}
			// Store a bounded operation identifier, never user-supplied path segments or secrets.
			if logErr := guard.service.RecordCall(request.Context(), call); logErr != nil {
				logger.Error(request.Context(), "Unable to record channel API call", logErr)
			}
		}()
	}
	if err != nil || credential.ChannelID != appID {
		request.SetError(bizerr.NewCode(codeTokenNotAccepted))
		return
	}
	request.SetCtx(withTokenCredential(request.Context(), credential))
	request.Middleware.Next()
}

func requiredScopeForRequest(method string, path string) string {
	method = strings.ToUpper(strings.TrimSpace(method))
	path = strings.TrimRight(strings.TrimSpace(path), "/")
	if scope := requiredCatalogScope(method, path); scope != "" {
		return scope
	}
	switch {
	case method == "GET" && strings.HasSuffix(path, "/channel/capabilities"):
		return "capability.read"
	case method == "POST" && strings.HasSuffix(path, "/channel/orders"):
		return "order.create"
	case method == "POST" && strings.HasSuffix(path, "/cancel") && strings.Contains(path, "/channel/orders/"):
		return "order.cancel"
	case method == "POST" && strings.HasSuffix(path, "/settle") && strings.Contains(path, "/channel/orders/"):
		return "settlement.repay"
	case method == "GET" && strings.HasSuffix(path, "/channel/finance"):
		return "settlement.read"
	case method == "POST" && strings.HasSuffix(path, "/channel/finance/credit-repayments"):
		return "settlement.repay"
	case method == "GET" && strings.Contains(path, "/channel/services"):
		return "service.read"
	case method == "GET" && strings.Contains(path, "/channel/orders/"):
		return "order.read"
	case method == "GET" && strings.Contains(path, "/channel/webhooks/"):
		return "webhook.read"
	case method == "POST" && strings.Contains(path, "/channel/services/") && strings.Contains(path, "/operations/"):
		if strings.HasSuffix(path, "/confidential-output") {
			return "service.output"
		}
		operation := path[strings.LastIndex(path, "/")+1:]
		switch operation {
		case "renew", "reconfigure", "reinstall", "console", "change_ip", "terminate":
			return "service." + operation
		}
	}
	return ""
}

func requiredCatalogScope(method, path string) string {
	switch {
	case method == "POST" && strings.Contains(path, "/channel/catalog/products/") && strings.HasSuffix(path, "/file-uploads"):
		return "order.create"
	case method == "PUT" && strings.Contains(path, "/channel/file-uploads/") && strings.HasSuffix(path, "/content"):
		return "order.create"
	case method == "GET" && strings.HasSuffix(path, "/channel/catalog"):
		return "catalog.read"
	case method == "GET" && strings.HasSuffix(path, "/channel/catalog/products"):
		return "catalog.read"
	case (method == "GET" || method == "POST") && strings.Contains(path, "/channel/catalog/products/"):
		return "catalog.read"
	case method == "POST" && strings.HasSuffix(path, "/channel/quotes"):
		return "catalog.read"
	default:
		return ""
	}
}

// directPeerIP does not trust spoofable forwarding headers for network restrictions.
func directPeerIP(remote string) string {
	host, _, err := net.SplitHostPort(remote)
	if err == nil {
		return host
	}
	return remote
}
