// This file validates newly created or changed usernames at the existing public
// write routes. Existing values may be echoed unchanged; no account is renamed.
package profileguard

import (
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/gogf/gf/v2/net/ghttp"
	"lina-core/pkg/plugin/capability/usercap"
	"lina-core/pkg/plugin/pluginhost"
)

var (
	usernameEmailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+$`)
	usernamePhonePattern = regexp.MustCompile(`^\+[1-9][0-9]{6,14}$`)
)

// onlyUsernameCreate leaves every unrelated route and its middleware intact.
func onlyUsernameCreate(handler pluginhost.MiddlewareHandler) pluginhost.MiddlewareHandler {
	return func(r *ghttp.Request) {
		value := strings.ToLower(path.Clean(r.URL.Path))
		if r.Method == http.MethodPost && (value == "/api/v1/auth/register" || value == "/api/v1/user") {
			handler(r)
			return
		}
		r.Middleware.Next()
	}
}

func (g *guard) handleUsernameCreate(r *ghttp.Request) {
	if !g.checkUsername(r, true) {
		return
	}
	r.Middleware.Next()
}

// usernameAllowed matches the login form's numeric, phone and email namespaces.
func usernameAllowed(value string) bool {
	value = strings.TrimSpace(value)
	numeric := value != ""
	for _, character := range value {
		if !unicode.IsDigit(character) {
			numeric = false
			break
		}
	}
	return !numeric && !usernameEmailPattern.MatchString(value) && !usernamePhonePattern.MatchString(value)
}

// checkUsername makes at most one data-scoped read, only for a disallowed value
// echoed by an edit form. The real route retains all authorization and writes.
func (g *guard) checkUsername(r *ghttp.Request, creating bool) bool {
	var (
		value              string
		present, validType bool
	)
	for key, input := range r.GetMap() {
		key = strings.ToLower(strings.NewReplacer("_", "", "-", "", " ", "").Replace(key))
		if key != "username" {
			continue
		}
		if present {
			g.rejectCode(r, codeUsernameInvalid, http.StatusBadRequest)
			return false
		}
		present = true
		value, validType = input.(string)
	}
	if !present || (validType && usernameAllowed(value)) {
		return true
	}
	if !creating && validType && g.users != nil {
		target, _ := profileUpdateTarget(r)
		if target == 0 {
			target = g.bizctx.Current(r.Context()).UserID
		}
		current, err := g.users.Get(r.Context(), usercap.UserID(strconv.Itoa(target)))
		if err != nil {
			r.SetError(err)
			return false
		}
		if current != nil && current.Username == value {
			return true
		}
	}
	g.rejectCode(r, codeUsernameInvalid, http.StatusBadRequest)
	return false
}
