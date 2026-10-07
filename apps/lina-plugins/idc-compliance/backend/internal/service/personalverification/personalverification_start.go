// This file initiates one non-replayable personal verification after durable local preparation.

package personalverification

import (
	"context"
	"errors"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/internal/provider/aliyunface"
	"lina-plugin-idc-core/backend/cap/identitycap"
)

// Start creates local recovery evidence before issuing one non-replayable provider request.
func (s *serviceImpl) Start(ctx context.Context, input StartInput) (*Verification, error) {
	defer clearBytes(input.CertName)
	defer clearBytes(input.CertNo)
	if s == nil || ctx == nil || ctx.Err() != nil || input.ConsentNo == "" {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	identity, err := validateStartInput(input, s.now())
	if err != nil {
		return nil, err
	}
	defer clearBytes(identity.name)
	defer clearBytes(identity.number)
	retentionUntil, err := s.privacyGate.RequireSensitiveCollection(
		ctx, "personal_verification", "personal_verification_material",
	)
	if err != nil || retentionUntil.IsZero() {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	subject, err := s.identities.PreparePersonal(ctx, input.ConsentNo)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeNotAccessible)
	}
	configuration, err := s.configurations.Resolve(ctx)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeProviderUnavailable)
	}
	stage, err := s.createInitiationStage(
		ctx, *subject, input.ConsentNo, configuration, identity.maskedName, identity.maskedNumber, retentionUntil,
	)
	if err != nil {
		return nil, err
	}
	challenge, err := newSnapshotChallenge(s.random)
	if err != nil {
		return nil, s.failStart(
			ctx, stage, reasonSnapshotInvalid, attemptStatusCanceled, err, CodeUnavailable,
		)
	}
	defer clearBytes(challenge)
	producer, err := newSnapshotProducer(challenge, identity.name, identity.number)
	if err != nil {
		return nil, s.failStart(ctx, stage, reasonSnapshotInvalid, attemptStatusCanceled, err, CodeInvalid)
	}
	created, err := s.snapshotManager.Create(ctx, "personal-snapshot:"+stage.caseNo, producer)
	if err != nil {
		return nil, s.failStart(ctx, stage, reasonSnapshotUnavailable, attemptStatusCanceled, err, CodeUnavailable)
	}
	if created.Handle.IsZero() || !created.Metadata.Configured ||
		created.Metadata.UpdatedAt.IsZero() {
		return nil, s.failStart(
			ctx, stage, reasonSnapshotUnavailable, attemptStatusCanceled, nil, CodeUnavailable,
		)
	}
	digest := snapshotDigest(challenge)
	attachErr := s.attachSnapshot(ctx, stage, created.Handle.OpaqueRef(), digest)
	if digest == "" || attachErr != nil {
		if attachErr == nil {
			attachErr = gerror.New("personal identity snapshot digest is invalid")
		}
		revoked, revokeErr := s.snapshotManager.Revoke(ctx, created.Handle, created.Metadata.UpdatedAt)
		if revokeErr != nil {
			attachErr = gerror.Wrap(revokeErr, "revoke unattached personal identity snapshot")
		} else if revoked.Configured {
			attachErr = gerror.New("unattached personal identity snapshot remained configured")
		}
		return nil, s.failStart(
			ctx, stage, reasonSnapshotPersistenceFailed, attemptStatusCanceled,
			attachErr, CodeUnavailable,
		)
	}
	projection, err := s.identities.ProjectPersonal(ctx, identitycap.PersonalProjectionInput{
		CustomerNo: subject.CustomerNo, CaseNo: stage.caseNo, Status: identitycap.StatusPending,
	})
	if err != nil {
		return nil, s.failStart(ctx, stage, reasonProjectionUnavailable, attemptStatusCanceled, err, CodeUnavailable)
	}
	if projection == nil || projection.Status != identitycap.StatusPending {
		return nil, s.failStart(
			ctx, stage, reasonProjectionUnavailable, attemptStatusCanceled, nil, CodeUnavailable,
		)
	}
	client, err := s.clients.Create(configuration)
	if err != nil {
		return nil, s.failStart(
			ctx, stage, reasonProviderClientUnavailable, attemptStatusCanceled, err, CodeProviderUnavailable,
		)
	}
	request := aliyunface.InitiateRequest{
		SceneID: configuration.SceneID, OuterOrderNo: stage.outerOrder,
		CertType: input.CertType, CertName: string(identity.name), CertNo: string(identity.number),
		ReturnURL: input.ReturnURL,
	}
	initiation, err := client.Initiate(ctx, request)
	request.CertName, request.CertNo = "", ""
	if err != nil {
		var callError *aliyunface.CallError
		if errors.As(err, &callError) && callError.Unknown {
			return nil, s.failStart(
				ctx, stage, reasonProviderInitializationUnknown, attemptStatusUnknown, nil, CodeReviewRequired,
			)
		}
		return nil, s.failStart(
			ctx, stage, reasonProviderInitializationRejected, attemptStatusCanceled, err, CodeProviderUnavailable,
		)
	}
	issuedAt := s.now().UTC().Truncate(time.Microsecond)
	if err = s.completeInitiation(ctx, stage, initiation.CertifyID, issuedAt); err != nil {
		return nil, s.failStart(
			ctx, stage, reasonProviderInitializationPersistenceFailed, attemptStatusUnknown, err, CodeReviewRequired,
		)
	}
	expiresAt := issuedAt.Add(aliyunface.CertificationLifetime)
	return &Verification{
		CaseNo: stage.caseNo, AttemptNo: stage.attemptNo, Status: StatusVerifying,
		ContinuationURL: initiation.CertifyURL, MaskedName: stage.maskedName,
		MaskedIdentityNo: stage.maskedIdentity, SubmittedAt: &issuedAt, ExpiresAt: &expiresAt,
	}, nil
}

func (s *serviceImpl) failStart(
	ctx context.Context,
	stage initiationStage,
	reason verificationReasonCode,
	attemptStatus attemptStatusCode,
	cause error,
	code *bizerr.Code,
) error {
	if err := s.requireReview(ctx, stage, reason, attemptStatus); err != nil {
		return bizerr.WrapCode(err, CodeUnavailable)
	}
	if cause != nil {
		return bizerr.WrapCode(cause, code)
	}
	return bizerr.NewCode(code)
}
