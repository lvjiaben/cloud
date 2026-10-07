// This file exposes ownership-only customer SSH public-key endpoints.

package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/sshkey"
)

func (c *ControllerV1) SSHPublicKeyList(ctx context.Context, _ *v1.SSHPublicKeyListReq) (*v1.SSHPublicKeyListRes, error) {
	items, err := c.sshKeys.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*v1.SSHPublicKeyItem, 0, len(items))
	for index := range items {
		result = append(result, toAPISSHPublicKey(&items[index]))
	}
	return &v1.SSHPublicKeyListRes{Items: result}, nil
}

func (c *ControllerV1) SSHPublicKeyCreate(ctx context.Context, req *v1.SSHPublicKeyCreateReq) (*v1.SSHPublicKeyCreateRes, error) {
	item, err := c.sshKeys.Create(ctx, sshkey.CreateInput{
		Name: req.Name, AuthorizedKey: req.AuthorizedKey, IdempotencyKey: req.RequestNo,
	})
	if err != nil {
		return nil, err
	}
	return &v1.SSHPublicKeyCreateRes{Item: toAPISSHPublicKey(item)}, nil
}

func (c *ControllerV1) SSHPublicKeyRevoke(ctx context.Context, req *v1.SSHPublicKeyRevokeReq) (*v1.SSHPublicKeyRevokeRes, error) {
	item, err := c.sshKeys.Revoke(ctx, sshkey.RevokeInput{
		KeyNo: req.KeyNo, ExpectedVersion: req.ExpectedVersion, IdempotencyKey: req.RequestNo,
	})
	if err != nil {
		return nil, err
	}
	return &v1.SSHPublicKeyRevokeRes{Item: toAPISSHPublicKey(item)}, nil
}

func toAPISSHPublicKey(item *sshkey.PublicKey) *v1.SSHPublicKeyItem {
	if item == nil {
		return nil
	}
	result := &v1.SSHPublicKeyItem{KeyNo: item.KeyNo, Name: item.Name, Algorithm: item.Algorithm,
		Fingerprint: item.Fingerprint, Status: string(item.Status), Version: item.Version,
		CreatedAt: item.CreatedAt.UnixMilli()}
	if item.RevokedAt != nil {
		value := item.RevokedAt.UTC().UnixMilli()
		result.RevokedAt = &value
	}
	return result
}
