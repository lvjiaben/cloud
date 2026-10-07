package channel

import (
	"strings"

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
	parts := strings.Fields(request.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		request.SetError(bizerr.NewCode(codeTokenNotAccepted))
		return
	}
	scope := requiredScopeForRequest(request.Method, request.URL.Path)
	credential, err := guard.service.AuthenticateToken(request.Context(), parts[1], request.GetClientIp(), scope)
	if err != nil {
		request.SetError(err)
		return
	}
	request.SetCtx(withTokenCredential(request.Context(), credential))
	request.Middleware.Next()
}

func requiredScopeForRequest(method string, path string) string {
	method = strings.ToUpper(strings.TrimSpace(method))
	path = strings.TrimRight(strings.TrimSpace(path), "/")
	switch {
	case method == "GET" && strings.HasSuffix(path, "/channel/catalog"):
		return "catalog.read"
	case method == "GET" && strings.HasSuffix(path, "/channel/quotes"):
		return "catalog.read"
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
