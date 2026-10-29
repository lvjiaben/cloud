// GoFrame DTO merging permits a body/query value to shadow a path parameter.
// Native resource identities always come from the router that matched the URL.
package accountnative

import (
	"context"
	"regexp"

	"github.com/gogf/gf/v2/frame/g"
	"lina-plugin-idc-resource/backend/internal/service/customeroperations"
)

var nativePathPatterns = map[string]*regexp.Regexp{
	"serverNo":      regexp.MustCompile(`^SVR-[0-9A-F]{32}$`),
	"entitlementNo": regexp.MustCompile(`^ent_[0-9a-f]{32}$`),
	"operationNo":   regexp.MustCompile(`^nso_[0-9a-f]{32}$`),
	"name":          regexp.MustCompile(`^(generatedCredential|consoleTicket)$`),
}

func bindNativePath(ctx context.Context, name string, target *string) error {
	request := g.RequestFromCtx(ctx)
	pattern := nativePathPatterns[name]
	if request == nil || pattern == nil || target == nil {
		return customeroperations.ErrInvalid
	}
	value := request.GetRouter(name).String()
	if !pattern.MatchString(value) {
		return customeroperations.ErrInvalid
	}
	*target = value
	return nil
}
