// This file persists validated configuration snapshots and keeps secret
// plaintext behind the owner-scoped secretcap lifecycle boundary.

package productautomation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/internal/model/do"
)

type storedConfigurationSecret struct {
	fieldKey string
	result   secretcap.CreateResult
}

func (s *serviceImpl) createConfigurationSecrets(
	ctx context.Context,
	interfaceNo string,
	values map[string]string,
) ([]storedConfigurationSecret, *interfaceSecretOperation, error) {
	if len(values) == 0 {
		return []storedConfigurationSecret{}, nil, nil
	}
	if s.secrets == nil {
		return nil, nil, gerror.New("product automation secret storage is unavailable")
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	operation, err := s.prepareInterfaceSecretOperation(ctx, interfaceNo, 0, keys)
	if err != nil {
		return nil, nil, err
	}
	stored := make([]storedConfigurationSecret, 0, len(keys))
	for _, key := range keys {
		idempotencyKey := secretOperationIdempotencyKey(operation.OperationNo, key)
		plaintext := []byte(values[key])
		result, err := s.secrets.Create(ctx, idempotencyKey, func(consume secretcap.Consumer) error {
			return consume(plaintext)
		})
		clear(plaintext)
		if err != nil {
			recovered, recoverErr := s.secrets.RecoverCreate(ctx, idempotencyKey)
			if recoverErr != nil {
				failures := []error{gerror.Wrap(err, "store interface configuration secret"), gerror.Wrap(recoverErr, "recover uncertain interface configuration secret")}
				failure := errors.Join(failures...)
				failures = append(failures, s.markInterfaceSecretOperationPending(ctx, operation, failure), s.compensateInterfaceSecretOperation(ctx, operation))
				return nil, operation, errors.Join(failures...)
			}
			result = recovered
		}
		stored = append(stored, storedConfigurationSecret{fieldKey: key, result: result})
		if err = s.recordInterfaceSecretOperationField(ctx, operation.ID, key, result); err != nil {
			failures := []error{err, s.markInterfaceSecretOperationPending(ctx, operation, err), s.compensateInterfaceSecretOperation(ctx, operation)}
			return nil, operation, errors.Join(failures...)
		}
	}
	if err = s.markInterfaceSecretOperationReady(ctx, operation.ID); err != nil {
		return nil, operation, errors.Join(err, s.markInterfaceSecretOperationPending(ctx, operation, err), s.compensateInterfaceSecretOperation(ctx, operation))
	}
	return stored, operation, nil
}

func (s *serviceImpl) persistInterfaceConfiguration(
	ctx context.Context,
	interfaceNo string,
	actorUserID int,
	compiled *canonicalConfiguration,
	schemaVersion string,
	secrets []storedConfigurationSecret,
) error {
	declarationDigest := configurationDigest(compiled.DeclarationJSON)
	valueDigest := configurationDigest(compiled.ValuesJSON)
	revisionID, err := s.database.Model(do.ProductAutomationInterfaceConfigurationRevision{}).Ctx(ctx).Data(do.ProductAutomationInterfaceConfigurationRevision{
		TenantId: idccap.DefaultTenantID, InterfaceInstanceNo: interfaceNo, Revision: 1,
		SchemaVersion: schemaVersion, DeclarationSnapshot: string(compiled.DeclarationJSON),
		PublicValueSnapshot: string(compiled.ValuesJSON), DeclarationDigest: declarationDigest,
		ValueDigest: valueDigest, SecretFieldCount: len(secrets), CreatedBy: actorUserID,
	}).InsertAndGetId()
	if err != nil {
		return gerror.Wrap(err, "insert interface configuration revision")
	}
	for _, value := range secrets {
		updatedAt := value.result.Metadata.UpdatedAt
		if _, err = s.database.Model(do.ProductAutomationInterfaceSecretReference{}).Ctx(ctx).Data(do.ProductAutomationInterfaceSecretReference{
			TenantId: idccap.DefaultTenantID, ConfigurationRevisionId: revisionID,
			FieldKey: value.fieldKey, SecretRef: value.result.Handle.OpaqueRef(), Configured: value.result.Metadata.Configured,
			MaskedTail: value.result.Metadata.MaskedTail, SecretUpdatedAt: &updatedAt,
		}).Insert(); err != nil {
			return gerror.Wrap(err, "insert interface secret reference")
		}
	}
	return nil
}

func configurationDigest(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}
