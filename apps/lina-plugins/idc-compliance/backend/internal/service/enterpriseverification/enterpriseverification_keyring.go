// This file owns the installation-local enterprise material encryption keyring.

package enterpriseverification

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-compliance/backend/internal/model/do"
	"lina-plugin-idc-compliance/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

const (
	// activeEnterpriseKeyNo is the installation-local keyring generation in use.
	activeEnterpriseKeyNo = "enterprise_material_key_v1"
	// enterpriseKeyCreateIdempotency safely recovers interrupted key creation.
	enterpriseKeyCreateIdempotency = "enterprise-material-master-key-v1"
	// enterpriseKeyStatusActive selects the only key eligible for new envelopes.
	enterpriseKeyStatusActive = "active"
)

func (s *serviceImpl) ensureActiveKey(ctx context.Context) (string, error) {
	return ensureEnterpriseKey(ctx, s.keyManager)
}

func ensureEnterpriseKey(
	ctx context.Context,
	manager secretcap.GeneratedManager,
) (string, error) {
	row, err := loadEnterpriseKey(ctx, activeEnterpriseKeyNo)
	if err != nil {
		return "", err
	}
	if row != nil {
		return row.KeyNo, nil
	}
	created, err := manager.CreateGenerated(
		ctx, enterpriseKeyCreateIdempotency, secretcap.GeneratedSecretPolicyURLSafe256,
	)
	if err != nil || created.Handle.IsZero() {
		return "", gerror.New("enterprise material encryption key is unavailable")
	}
	_, err = dao.EnterpriseEncryptionKey.Ctx(ctx).Data(do.EnterpriseEncryptionKey{
		TenantId: idccap.DefaultTenantID, KeyNo: activeEnterpriseKeyNo,
		SecretRef: created.Handle.OpaqueRef(), Generation: 1,
		StatusCode: enterpriseKeyStatusActive, Version: 1,
	}).InsertIgnore()
	if err != nil {
		return "", gerror.New("enterprise material encryption key is unavailable")
	}
	row, err = loadEnterpriseKey(ctx, activeEnterpriseKeyNo)
	if err != nil || row == nil || row.SecretRef != created.Handle.OpaqueRef() {
		return "", gerror.New("enterprise material encryption key is inconsistent")
	}
	return row.KeyNo, nil
}

func loadEnterpriseKey(ctx context.Context, keyNo string) (*entity.EnterpriseEncryptionKey, error) {
	var row *entity.EnterpriseEncryptionKey
	err := dao.EnterpriseEncryptionKey.Ctx(ctx).Fields(
		"key_no", "secret_ref", "generation", "status_code", "version",
	).Where(do.EnterpriseEncryptionKey{
		TenantId: idccap.DefaultTenantID, KeyNo: keyNo, StatusCode: enterpriseKeyStatusActive,
	}).Limit(1).Scan(&row)
	if err != nil {
		return nil, gerror.New("enterprise material encryption key is unavailable")
	}
	if row != nil && (row.Generation != 1 || row.Version <= 0 || row.SecretRef == "") {
		return nil, gerror.New("enterprise material encryption key is inconsistent")
	}
	return row, nil
}

func (s *serviceImpl) withKey(
	ctx context.Context,
	keyNo string,
	consume func([]byte) error,
) error {
	return accessEnterpriseKey(ctx, s.keyAccessor, keyNo, consume)
}

func accessEnterpriseKey(
	ctx context.Context,
	accessor secretcap.Accessor,
	keyNo string,
	consume func([]byte) error,
) error {
	if consume == nil {
		return gerror.New("enterprise material encryption consumer is unavailable")
	}
	row, err := loadEnterpriseKey(ctx, keyNo)
	if err != nil || row == nil {
		return gerror.New("enterprise material encryption key is unavailable")
	}
	handle, err := secretcap.NewHandle(row.SecretRef)
	if err != nil {
		return gerror.New("enterprise material encryption key is unavailable")
	}
	purpose, err := secretcap.NewPurpose("compliance.enterprise-material-encryption")
	if err != nil {
		return gerror.New("enterprise material encryption purpose is unavailable")
	}
	request, err := secretcap.NewAccessRequest(handle, purpose)
	if err != nil {
		return gerror.New("enterprise material encryption request is unavailable")
	}
	return accessor.Access(ctx, request, func(plaintext []byte) error {
		key, decodeErr := decodeGeneratedKey(plaintext)
		if decodeErr != nil {
			return decodeErr
		}
		defer clearBytes(key)
		return consume(key)
	})
}
