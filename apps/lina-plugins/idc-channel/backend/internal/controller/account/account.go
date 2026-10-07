// Package account adapts current-customer channel token commands to HTTP.
package account

import (
	"context"
	"time"

	api "lina-plugin-idc-channel/backend/api/account"
	"lina-plugin-idc-channel/backend/api/account/v1"
	channelservice "lina-plugin-idc-channel/backend/internal/service/channel"
)

type ControllerV1 struct{ service channelservice.Service }

func NewV1(service channelservice.Service) api.IAccountV1 { return &ControllerV1{service: service} }

func unixMillis(value *time.Time) *int64 {
	if value == nil {
		return nil
	}
	milliseconds := value.UnixMilli()
	return &milliseconds
}

func (c *ControllerV1) ListTokens(ctx context.Context, _ *v1.ListTokensReq) (*v1.ListTokensRes, error) {
	rows, err := c.service.ListOwnTokens(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]*v1.TokenSummary, 0, len(rows))
	for _, row := range rows {
		item := &v1.TokenSummary{Id: row.ID, Scopes: row.Scopes, SourceCIDRs: row.SourceCIDRs,
			CreatedAt: row.CreatedAt.UnixMilli(), ExpiresAt: unixMillis(row.ExpiresAt),
			RevokedAt: unixMillis(row.RevokedAt)}
		items = append(items, item)
	}
	return &v1.ListTokensRes{List: items}, nil
}

func (c *ControllerV1) CreateToken(ctx context.Context, req *v1.CreateTokenReq) (*v1.CreateTokenRes, error) {
	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		value := time.UnixMilli(*req.ExpiresAt)
		expiresAt = &value
	}
	result, err := c.service.CreateOwnToken(ctx, channelservice.TokenInput{Scopes: req.Scopes, SourceCIDRs: req.SourceCIDRs, ExpiresAt: expiresAt})
	if err != nil {
		return nil, err
	}
	return &v1.CreateTokenRes{Id: result.ID, Token: result.Token, ExpiresAt: unixMillis(result.ExpiresAt)}, nil
}

func (c *ControllerV1) RevokeToken(ctx context.Context, req *v1.RevokeTokenReq) (*v1.RevokeTokenRes, error) {
	if err := c.service.RevokeOwnToken(ctx, req.Id); err != nil {
		return nil, err
	}
	return &v1.RevokeTokenRes{Id: req.Id, Revoked: true}, nil
}
