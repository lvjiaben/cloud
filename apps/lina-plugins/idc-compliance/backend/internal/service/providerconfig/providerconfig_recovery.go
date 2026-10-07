// This file resumes one persisted fail-closed credential mutation without
// depending on a browser retaining the original request idempotency key.
package providerconfig

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-compliance/backend/internal/provider/aliyunface"
)

// Recover retries only the current updating mutation with its server-owned
// operation key. Secretcap must create, replace, or replay the exact mutation
// before the local configuration can return to configured.
func (s *serviceImpl) Recover(ctx context.Context, input RecoveryInput) (*Recovery, error) {
	defer clearRecoveryInput(&input)
	actorUserID, err := s.authorize(ctx, PermissionWrite)
	if err != nil {
		return nil, err
	}
	if err = s.ensureRecentLogin(ctx, actorUserID); err != nil {
		return nil, err
	}
	owned := RecoveryInput{
		AccessKeyID:     append([]byte(nil), input.AccessKeyID...),
		AccessKeySecret: append([]byte(nil), input.AccessKeySecret...),
	}
	defer clearRecoveryInput(&owned)
	if aliyunface.ValidateCredential(owned.AccessKeyID, owned.AccessKeySecret) != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	stage, digest, err := s.loadRecoveryStage(ctx)
	if err != nil {
		return nil, err
	}
	producer, err := aliyunface.NewCredentialProducer(owned.AccessKeyID, owned.AccessKeySecret)
	if err != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	mutation, err := s.mutateSecret(ctx, stage, producer)
	if err != nil {
		return nil, err
	}
	if err = s.completeMutation(ctx, stage, digest, mutation.handle); err != nil {
		return nil, err
	}
	row, err := s.load(ctx, false)
	if err != nil || row == nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	configuration, err := s.project(ctx, row)
	if err != nil {
		return nil, err
	}
	return &Recovery{
		Configuration:          configuration,
		ReusedExistingMutation: mutation.reused,
	}, nil
}

func (s *serviceImpl) loadRecoveryStage(ctx context.Context) (mutationStage, string, error) {
	var stage mutationStage
	var digest string
	err := dao.ProviderConfiguration.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, err := s.load(ctx, true)
		if err != nil {
			return err
		}
		if row == nil || Status(row.StatusCode) != statusUpdating ||
			row.LastMutationDigest == nil || !lowerHex64Pattern.MatchString(*row.LastMutationDigest) {
			return bizerr.NewCode(CodeConflict)
		}
		if err = s.resumeStage(row, &stage); err != nil {
			return err
		}
		digest = *row.LastMutationDigest
		return nil
	})
	if err != nil {
		return mutationStage{}, "", publicWriteError(err)
	}
	return stage, digest, nil
}
