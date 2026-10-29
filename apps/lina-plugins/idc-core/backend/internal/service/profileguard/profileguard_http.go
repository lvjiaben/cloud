// This file composes the original profile write with verified contact and username
// policy checks, preserving caller permissions and transaction rollback.

package profileguard

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/net/ghttp"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/customerlogincap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/profilechangecap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const maxProfileBody = 32 << 10

// profileUpdateTarget recognizes only the two published identity update routes.
func profileUpdateTarget(r *ghttp.Request) (int, bool) {
	if r.Method != http.MethodPut {
		return 0, false
	}
	value := strings.ToLower(path.Clean(r.URL.Path))
	if value == "/api/v1/user/profile" {
		return 0, true
	}
	value = strings.TrimPrefix(value, "/api/v1/user/")
	id, err := strconv.Atoi(value)
	return id, err == nil && id > 0
}

func onlyProfileUpdate(handler pluginhost.MiddlewareHandler) pluginhost.MiddlewareHandler {
	return func(r *ghttp.Request) {
		if _, ok := profileUpdateTarget(r); ok {
			handler(r)
			return
		}
		r.Middleware.Next()
	}
}

func (g *guard) handle(r *ghttp.Request) {
	if r.Body != nil {
		body, err := io.ReadAll(io.LimitReader(r.Body, maxProfileBody+1))
		if err != nil || len(body) > maxProfileBody {
			g.reject(r)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
	}
	if !g.checkUsername(r, false) {
		return
	}
	parsed := parseProfilePatch(r)
	input, count, invalid, patch, password := parsed.input, parsed.count, parsed.invalid, parsed.patch, parsed.password
	if count == 0 && patch.Username == nil && !password {
		r.Middleware.Next()
		return
	}
	current := g.bizctx.Current(r.Context())
	if current.UserID <= 0 {
		g.reject(r)
		return
	}
	if current.TenantID != idccap.DefaultTenantID {
		r.Middleware.Next()
		return
	}
	audience, err := g.audiences.Current(r.Context())
	if err != nil {
		r.SetError(err)
		return
	}
	staff := (audience != nil && audience.Code == principal.AudienceStaff) || (audience == nil && (current.IsSuperAdmin || idccap.HasAdminPermission(current.Permissions)))
	target, _ := profileUpdateTarget(r)
	if target == 0 {
		target = current.UserID
	}
	provider, err := g.provider(r.Context())
	if err != nil {
		r.SetError(err)
		return
	}
	recovery := r.Header.Get("X-Password-Recovery")
	if recovery != "" {
		validator, ok := provider.(profilechangecap.RecoveryProvider)
		if !ok || !password || count != 0 || patch.Username != nil || target != current.UserID || len(r.GetMap()) != 1 {
			g.reject(r)
			return
		}
		g.runIndex(r, target, patch, func(write profilechangecap.Write) error { return validator.ApplyRecovery(r.Context(), recovery, write) })
		return
	}
	if count == 0 || staff {
		g.runIndex(r, target, patch, func(write profilechangecap.Write) error {
			if provider == nil {
				return write(r.Context())
			}
			return provider.Serialize(r.Context(), target, write)
		})
		return
	}
	if provider == nil || invalid || count != 1 || target != current.UserID {
		g.reject(r)
		return
	}
	proofs := strings.Split(r.Header.Get("X-Contact-Verification"), ".")
	if len(proofs) != 2 {
		g.reject(r)
		return
	}
	input.OldVerification, input.NewVerification = proofs[0], proofs[1]
	g.runIndex(r, target, patch, func(write profilechangecap.Write) error { return provider.Apply(r.Context(), input, write) })
}

// provider resolves the startup instance only while its owner remains enabled.
func (g *guard) provider(ctx context.Context) (profilechangecap.Provider, error) {
	if g.verifier == nil || g.plugins == nil {
		return nil, nil
	}
	enabled, err := g.plugins.IsEnabledAuthoritative(ctx, plugincap.PluginID(profilechangecap.ProviderPluginID))
	if err != nil || !enabled {
		return nil, err
	}
	return g.verifier(), nil
}

func (g *guard) runIndex(r *ghttp.Request, target int, patch customerlogincap.Patch, apply func(profilechangecap.Write) error) {
	g.run(r, func(write profilechangecap.Write) error {
		return apply(func(ctx context.Context) error {
			if err := write(ctx); err != nil {
				return err
			}
			if g.directory != nil {
				return g.directory.RefreshUser(ctx, target, patch)
			}
			return nil
		})
	})
}

// run preserves the original route, permission middleware and request identity.
// A transaction error clears any buffered downstream response before returning.
func (g *guard) run(r *ghttp.Request, apply func(profilechangecap.Write) error) {
	original := r.Context()
	err := apply(func(ctx context.Context) error {
		r.SetCtx(ctx)
		defer r.SetCtx(original)
		r.Middleware.Next()
		return r.GetError()
	})
	if err != nil {
		r.Response.ClearBuffer()
		r.SetError(err)
	}
}

func (g *guard) reject(r *ghttp.Request) {
	g.rejectCode(r, codeContactVerificationRequired, http.StatusForbidden)
}

func (g *guard) rejectCode(r *ghttp.Request, code *bizerr.Code, status int) {
	err := bizerr.NewCode(code)
	structured, ok := bizerr.As(err)
	if !ok {
		r.SetError(err)
		r.ExitAll()
		return
	}
	r.Response.Header().Set("Cache-Control", "no-store")
	r.Response.WriteHeader(status)
	r.Response.WriteJsonExit(struct {
		Code          int            `json:"code"`
		Message       string         `json:"message"`
		ErrorCode     string         `json:"errorCode"`
		MessageKey    string         `json:"messageKey"`
		MessageParams map[string]any `json:"messageParams"`
	}{structured.TypeCode().Code(), g.translations.Translate(r.Context(), structured.MessageKey(), structured.Fallback()), structured.RuntimeCode(), structured.MessageKey(), structured.Params()})
}

// profilePatch contains only fields whose mutation changes login proofs.
type profilePatch struct {
	input    profilechangecap.Input
	count    int
	invalid  bool
	patch    customerlogincap.Patch
	password bool
}

func parseProfilePatch(r *ghttp.Request) profilePatch {
	var (
		input    profilechangecap.Input
		count    int
		invalid  bool
		patch    customerlogincap.Patch
		password bool
	)
	for key, value := range r.GetMap() {
		key = strings.ToLower(strings.NewReplacer("_", "", "-", "", " ", "").Replace(key))
		if key == "username" {
			if text, ok := value.(string); ok {
				patch.Username = &text
			}
		}
		if key == "password" {
			password = true
		}
		if key != "phone" && key != "email" {
			continue
		}
		count++
		text, ok := value.(string)
		if !ok {
			invalid = true
			continue
		}
		if key == "phone" {
			if input.Phone != nil {
				invalid = true
			}
			input.Phone = &text
		} else {
			if input.Email != nil {
				invalid = true
			}
			input.Email = &text
		}
	}
	patch.Email, patch.Phone = input.Email, input.Phone
	return profilePatch{input, count, invalid, patch, password}
}
