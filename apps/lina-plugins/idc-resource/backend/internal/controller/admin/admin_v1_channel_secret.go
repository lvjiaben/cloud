package admin

import (
	"context"
	"encoding/base64"
	"time"

	"lina-plugin-idc-resource/backend/api/admin/v1"
	"lina-plugin-idc-resource/backend/internal/service/channelsecret"
)

func (c *ControllerV1) ChannelSecretAccessCreate(
	ctx context.Context,
	req *v1.ChannelSecretAccessCreateReq,
) (*v1.ChannelSecretAccessCreateRes, error) {
	item, err := c.secrets.IssueAccess(ctx, channelsecret.AccessRequest{
		Operation: channelsecret.Operation(req.Operation), ChannelNo: req.ChannelNo,
		BindingName: req.BindingName, SecretDigest: req.SecretDigest,
		ExpectedUpdatedAt: unixMilliTime(req.ExpectedUpdatedAt), RotateLimit: req.RotateLimit,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ChannelSecretAccessCreateRes{Item: v1.ChannelSecretAccessContext{
		Token: item.Token, ExpiresAt: item.ExpiresAt.UnixMilli(),
	}}, nil
}

func (c *ControllerV1) ChannelSecretCreate(
	ctx context.Context,
	req *v1.ChannelSecretCreateReq,
) (*v1.ChannelSecretCreateRes, error) {
	secret, decodeErr := base64.StdEncoding.DecodeString(string(req.Secret))
	if decodeErr != nil {
		clear(req.Secret)
		return nil, decodeErr
	}
	item, err := c.secrets.Create(ctx, channelsecret.MutationInput{
		ChannelNo: req.ChannelNo, BindingName: req.BindingName, Secret: secret,
		IdempotencyKey: req.IdempotencyKey, AccessContext: req.AccessContext,
	})
	clear(secret)
	clear(req.Secret)
	if err != nil {
		return nil, err
	}
	return &v1.ChannelSecretCreateRes{Item: channelSecretStatus(item)}, nil
}

func (c *ControllerV1) ChannelSecretReplace(
	ctx context.Context,
	req *v1.ChannelSecretReplaceReq,
) (*v1.ChannelSecretReplaceRes, error) {
	secret, decodeErr := base64.StdEncoding.DecodeString(string(req.Secret))
	if decodeErr != nil {
		clear(req.Secret)
		return nil, decodeErr
	}
	item, err := c.secrets.Replace(ctx, channelsecret.MutationInput{
		ChannelNo: req.ChannelNo, BindingName: req.BindingName, Secret: secret,
		ExpectedUpdatedAt: unixMilliTime(req.ExpectedUpdatedAt),
		IdempotencyKey:    req.IdempotencyKey, AccessContext: req.AccessContext,
	})
	clear(secret)
	clear(req.Secret)
	if err != nil {
		return nil, err
	}
	return &v1.ChannelSecretReplaceRes{Item: channelSecretStatus(item)}, nil
}

func (c *ControllerV1) ChannelSecretRevoke(
	ctx context.Context,
	req *v1.ChannelSecretRevokeReq,
) (*v1.ChannelSecretRevokeRes, error) {
	item, err := c.secrets.Revoke(ctx, channelsecret.MutationInput{
		ChannelNo: req.ChannelNo, BindingName: req.BindingName,
		ExpectedUpdatedAt: unixMilliTime(req.ExpectedUpdatedAt),
		IdempotencyKey:    req.IdempotencyKey, AccessContext: req.AccessContext,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ChannelSecretRevokeRes{Item: channelSecretStatus(item)}, nil
}

func (c *ControllerV1) ChannelSecretRotate(
	ctx context.Context,
	req *v1.ChannelSecretRotateReq,
) (*v1.ChannelSecretRotateRes, error) {
	item, err := c.secrets.Rotate(ctx, channelsecret.MutationInput{
		ChannelNo: req.ChannelNo, RotateLimit: req.Limit,
		IdempotencyKey: req.IdempotencyKey, AccessContext: req.AccessContext,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ChannelSecretRotateRes{Rewrapped: item.Rewrapped, Remaining: item.Remaining}, nil
}

func channelSecretStatus(item *channelsecret.Status) v1.ChannelSecretStatus {
	return v1.ChannelSecretStatus{
		ChannelNo: item.ChannelNo, BindingName: item.BindingName,
		Configured: item.Configured, MaskedTail: item.MaskedTail,
		UpdatedAt: item.UpdatedAt.UnixMilli(),
	}
}

func unixMilliTime(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(value).UTC()
}
