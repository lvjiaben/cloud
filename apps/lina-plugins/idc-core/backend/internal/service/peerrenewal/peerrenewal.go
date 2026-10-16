// Package peerrenewal adapts exact frozen same-system entitlements to governed
// dynamic provider renewal calls. It never selects an arbitrary upstream service.
package peerrenewal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/renewalownercap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
)

// NewRouter adds the connector owner to the startup-frozen owner router using
// the resource owner's confidential issuer. Missing dependencies fail startup.
func NewRouter(base renewalownercap.Router, database gdb.DB, extensions pluginhost.BusinessExtensionRegistrar, outbound resourceinterfaceprojectioncap.ExistingBusinessOutboundSource) (renewalownercap.Router, error) {
	if base == nil || database == nil || extensions == nil || outbound == nil {
		return nil, gerror.New("peer renewal requires shared owner dependencies")
	}
	if _, exists := base.Resolve(renewalownercap.PeerOwnerID); exists {
		return nil, gerror.New("peer renewal owner already registered")
	}
	return &router{base: base, peer: &source{database: database, extensions: extensions, outbound: outbound}}, nil
}

type router struct {
	base renewalownercap.Router
	peer renewalownercap.Source
}

func (r *router) Resolve(owner string) (renewalownercap.Source, bool) {
	if owner == renewalownercap.PeerOwnerID {
		return r.peer, true
	}
	return r.base.Resolve(owner)
}

type source struct {
	database   gdb.DB
	extensions pluginhost.BusinessExtensionRegistrar
	outbound   resourceinterfaceprojectioncap.ExistingBusinessOutboundSource
}

// CreateTask submits only the operation whose invoice and entitlement facts were frozen by core.
func (s *source) CreateTask(ctx context.Context, input renewalownercap.TaskRequest) (*renewalownercap.Task, error) {
	return s.invoke(ctx, input, renewalownercap.PeerRenewMethod)
}

// ReadTask reconciles the same deterministic operation without replaying its write.
func (s *source) ReadTask(ctx context.Context, input renewalownercap.TaskReadRequest) (*renewalownercap.Task, error) {
	if renewalownercap.ValidateTaskReadRequest(input) != nil || input.TaskNo != input.Request.OperationNo {
		return nil, renewalownercap.ErrTaskConflict
	}
	return s.invoke(ctx, input.Request, renewalownercap.PeerRenewQueryMethod)
}
