// This file derives a fail-closed, purpose-separated provider-selection audience from host identity services.

package productautomationtoken

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"strconv"
	"sync"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/sessioncap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const (
	maximumProviderSelectionSessionIDBytes = 2048
	providerSelectionActorPurpose          = "idc-core/product-automation/provider-selection/audience/actor/v1\x00"
	providerSelectionSessionPurpose        = "idc-core/product-automation/provider-selection/audience/session/v1\x00"
)

// ProductionAudienceReader derives opaque bindings only after the current
// request, persisted principal, and authoritative online session agree.
type ProductionAudienceReader struct {
	mu        sync.RWMutex
	key       []byte
	bizCtx    bizctxcap.Service
	principal principal.Reader
	sessions  sessioncap.Service
}

var _ AudienceReader = (*ProductionAudienceReader)(nil)

// NewProductionAudienceReader constructs the production audience adapter. The
// supplied key is copied into the adapter and always cleared before returning,
// so callers cannot accidentally retain the temporary loader result.
func NewProductionAudienceReader(
	key []byte,
	bizCtx bizctxcap.Service,
	principalReader principal.Reader,
	sessions sessioncap.Service,
) (*ProductionAudienceReader, error) {
	defer clear(key)
	if len(key) != sha256.Size {
		return nil, gerror.New("provider selection audience requires a 32-byte key")
	}
	if isNilDependency(bizCtx) || isNilDependency(principalReader) || isNilDependency(sessions) {
		return nil, gerror.New("provider selection audience requires host identity services")
	}
	return &ProductionAudienceReader{
		key: append([]byte(nil), key...), bizCtx: bizCtx,
		principal: principalReader, sessions: sessions,
	}, nil
}

// Close clears the adapter-owned key. Reads after Close fail closed.
func (r *ProductionAudienceReader) Close() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	clear(r.key)
	r.key = nil
}

// ReadProviderSelectionAudience returns only opaque actor and session digests
// for one active, direct staff session in the installation's default tenant.
func (r *ProductionAudienceReader) ReadProviderSelectionAudience(ctx context.Context) (Audience, error) {
	if r == nil || ctx == nil || ctx.Err() != nil || isNilDependency(r.bizCtx) ||
		isNilDependency(r.principal) || isNilDependency(r.sessions) {
		return Audience{}, gerror.New("provider selection audience is unavailable")
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.key) != sha256.Size {
		return Audience{}, gerror.New("provider selection audience is unavailable")
	}
	current := r.bizCtx.Current(ctx)
	if current.UserID <= 0 || current.TenantID != idccap.DefaultTenantID ||
		current.TokenID == "" || len(current.TokenID) > maximumProviderSelectionSessionIDBytes ||
		current.ActingUserID != 0 || current.IsImpersonation || current.ActingAsTenant {
		return Audience{}, gerror.New("provider selection audience is unavailable")
	}
	audience, err := r.principal.Current(ctx)
	if err != nil || ctx.Err() != nil || audience == nil || audience.UserID != current.UserID ||
		audience.Code != principal.AudienceStaff || audience.Status != principal.AudienceStatusActive {
		return Audience{}, gerror.New("provider selection audience is unavailable")
	}
	session, err := r.sessions.Current(ctx)
	if err != nil || ctx.Err() != nil || session == nil || string(session.ID) != current.TokenID ||
		session.UserID != strconv.Itoa(current.UserID) ||
		string(session.TenantID) != strconv.Itoa(idccap.DefaultTenantID) {
		return Audience{}, gerror.New("provider selection audience is unavailable")
	}

	actorDigest, err := providerSelectionDigest(r.key, providerSelectionActorPurpose, current.TenantID, current.UserID, "")
	if err != nil {
		return Audience{}, gerror.New("provider selection audience is unavailable")
	}
	sessionDigest, err := providerSelectionDigest(r.key, providerSelectionSessionPurpose, current.TenantID, current.UserID, current.TokenID)
	if err != nil {
		return Audience{}, gerror.New("provider selection audience is unavailable")
	}
	return Audience{
		TenantID: current.TenantID, ActorDigest: actorDigest, SessionDigest: sessionDigest,
		DirectStaff: true,
	}, nil
}

func providerSelectionDigest(key []byte, purpose string, tenantID int, userID int, sessionID string) (string, error) {
	mac := hmac.New(sha256.New, key)
	if _, err := mac.Write([]byte(purpose)); err != nil {
		return "", err
	}
	var numeric [16]byte
	binary.BigEndian.PutUint64(numeric[:8], uint64(tenantID))
	binary.BigEndian.PutUint64(numeric[8:], uint64(userID))
	if _, err := mac.Write(numeric[:]); err != nil {
		return "", err
	}
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(sessionID)))
	if _, err := mac.Write(length[:]); err != nil {
		return "", err
	}
	if _, err := mac.Write([]byte(sessionID)); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
