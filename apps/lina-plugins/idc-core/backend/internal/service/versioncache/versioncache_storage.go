// This file implements public reads, standalone caching, clustered hit
// selection, and post-commit invalidation orchestration.

package versioncache

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"

	"lina-plugin-idc-core/backend/internal/service/versioncache/internal/coordination"
)

var errClusterStateAdvanced = errors.New("version cache cluster state advanced")

const (
	initialRevision     = "0"
	degradedRevision    = "degraded"
	contentionPollEvery = 25 * time.Millisecond
	maximumGetAttempts  = 3
)

// Get returns a validated snapshot from local/shared acceleration or authority.
func (s *serviceImpl) Get(ctx context.Context, key Key) (*Result, error) {
	if isNilInterface(ctx) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	normalized, err := normalizeKey(key)
	if err != nil {
		return nil, err
	}
	if !s.clusterEnabled {
		return s.getStandalone(ctx, normalized)
	}
	for range maximumGetAttempts {
		result, attemptErr := s.getClusteredAttempt(ctx, normalized)
		if !errors.Is(attemptErr, errClusterStateAdvanced) {
			return result, attemptErr
		}
	}
	return nil, bizerr.NewCode(CodeAuthorityUnavailable)
}

func (s *serviceImpl) getStandalone(ctx context.Context, key Key) (*Result, error) {
	storageKey := stableStorageKey(key)
	revision := s.local.revision(storageKey)
	now := s.currentTime()
	if local, found := s.local.get(storageKey); found && local.revision == revision &&
		s.withinStaleBound(local.loadedAt, now) {
		s.metrics.localHits.Add(1)
		return resultFromStored(local, SourceLocal, false, false), nil
	}
	release, gateErr := s.gates.acquire(ctx, storageKey)
	if gateErr != nil {
		return nil, bizerr.WrapCode(gateErr, CodeAuthorityUnavailable)
	}
	defer release()
	revision = s.local.revision(storageKey)
	now = s.currentTime()
	if local, found := s.local.get(storageKey); found && local.revision == revision &&
		s.withinStaleBound(local.loadedAt, now) {
		s.metrics.localHits.Add(1)
		return resultFromStored(local, SourceLocal, false, false), nil
	}
	loaded, err := s.loadAuthority(ctx, key, "")
	if err != nil {
		return nil, err
	}
	loaded.revision = revision
	s.local.set(storageKey, loaded)
	s.metrics.rebuilds.Add(1)
	return resultFromStored(loaded, SourceAuthority, false, false), nil
}

func (s *serviceImpl) getClusteredAttempt(ctx context.Context, key Key) (*Result, error) {
	storageKey := stableStorageKey(key)
	prior, _ := s.local.get(storageKey)
	watermark, watermarkErr := s.loadWatermark(ctx, key)
	if watermarkErr != nil {
		return s.degradedWithoutWatermark(ctx, key, storageKey, prior)
	}
	state, found, stateErr := s.coordination.Read(ctx, storageKey)
	if stateErr != nil {
		s.metrics.backendFailures.Add(1)
		return s.degradedAtWatermark(ctx, key, storageKey, prior, watermark)
	}
	if metadataCorrupt(state, found) {
		s.metrics.corruptEntries.Add(1)
		repaired, repairErr := s.coordination.Advance(ctx, storageKey, watermark)
		if repairErr != nil {
			s.metrics.backendFailures.Add(1)
			return s.degradedAtWatermark(ctx, key, storageKey, prior, watermark)
		}
		state, found = repaired, true
	}
	if shared, usable := s.clusterSnapshot(state, found, watermark, s.currentTime()); usable {
		revision := clusterRevision(state)
		if local, localFound := s.local.get(storageKey); localFound &&
			local.revision == revision && s.withinStaleBound(local.loadedAt, s.currentTime()) {
			s.metrics.localHits.Add(1)
			return resultFromStored(local, SourceLocal, false, false), nil
		}
		s.local.set(storageKey, shared)
		s.metrics.sharedHits.Add(1)
		return resultFromStored(shared, SourceShared, false, false), nil
	}
	return s.rebuildClustered(ctx, key, storageKey, watermark, prior)
}

// Invalidate advances the persistent cluster generation after, never inside,
// the authoritative transaction. Failure removes the local entry and returns a
// retryable error; clustered reads still compare against Authority.Watermark so
// another node cannot accept the pre-transaction shared snapshot.
func (s *serviceImpl) Invalidate(ctx context.Context, key Key) (string, error) {
	if isNilInterface(ctx) {
		return "", bizerr.NewCode(CodeInvalid)
	}
	normalized, err := normalizeKey(key)
	if err != nil {
		return "", err
	}
	storageKey := stableStorageKey(normalized)
	localRevision := s.local.invalidate(storageKey)
	if !s.clusterEnabled {
		s.metrics.invalidations.Add(1)
		return localRevision, nil
	}
	watermark, watermarkErr := s.loadWatermark(ctx, normalized)
	if watermarkErr != nil {
		s.metrics.invalidationFailures.Add(1)
		return "", bizerr.NewCode(CodeUnavailable)
	}
	state, advanceErr := s.coordination.Advance(ctx, storageKey, watermark)
	if advanceErr != nil || metadataCorrupt(state, true) || state.Watermark != watermark {
		s.metrics.invalidationFailures.Add(1)
		s.metrics.backendFailures.Add(1)
		return "", bizerr.NewCode(CodeUnavailable)
	}
	revision := clusterRevision(state)
	s.local.setRevision(storageKey, revision)
	s.metrics.invalidations.Add(1)
	return revision, nil
}

func (s *serviceImpl) degradedAtWatermark(
	ctx context.Context,
	key Key,
	storageKey string,
	prior storedSnapshot,
	watermark string,
) (*Result, error) {
	release, gateErr := s.gates.acquire(ctx, storageKey)
	if gateErr != nil {
		return nil, bizerr.WrapCode(gateErr, CodeAuthorityUnavailable)
	}
	defer release()
	if latest, found := s.local.get(storageKey); found {
		prior = latest
	}
	if prior.snapshot.Version == watermark && s.withinStaleBound(prior.loadedAt, s.currentTime()) {
		s.metrics.degradedReads.Add(1)
		if prior.revision == degradedRevision {
			s.metrics.localHits.Add(1)
			return resultFromStored(prior, SourceLocal, false, true), nil
		}
		s.metrics.staleReads.Add(1)
		return resultFromStored(prior, SourceStaleLocal, true, true), nil
	}
	loaded, err := s.loadAuthority(ctx, key, watermark)
	if err != nil {
		return s.staleOrUnavailable(prior)
	}
	loaded.revision = degradedRevision
	s.local.set(storageKey, loaded)
	s.metrics.rebuilds.Add(1)
	s.metrics.degradedReads.Add(1)
	return resultFromStored(loaded, SourceAuthority, false, true), nil
}

func (s *serviceImpl) degradedWithoutWatermark(
	ctx context.Context,
	key Key,
	storageKey string,
	prior storedSnapshot,
) (*Result, error) {
	release, gateErr := s.gates.acquire(ctx, storageKey)
	if gateErr != nil {
		return nil, bizerr.WrapCode(gateErr, CodeAuthorityUnavailable)
	}
	defer release()
	if latest, found := s.local.get(storageKey); found {
		prior = latest
	}
	loaded, err := s.loadAuthority(ctx, key, "")
	if err != nil {
		return s.staleOrUnavailable(prior)
	}
	loaded.revision = degradedRevision
	s.local.set(storageKey, loaded)
	s.metrics.rebuilds.Add(1)
	s.metrics.degradedReads.Add(1)
	return resultFromStored(loaded, SourceAuthority, false, true), nil
}

func (s *serviceImpl) staleOrUnavailable(prior storedSnapshot) (*Result, error) {
	if len(prior.snapshot.Document) > 0 && s.withinStaleBound(prior.loadedAt, s.currentTime()) {
		s.metrics.degradedReads.Add(1)
		s.metrics.staleReads.Add(1)
		return resultFromStored(prior, SourceStaleLocal, true, true), nil
	}
	return nil, bizerr.NewCode(CodeAuthorityUnavailable)
}

func clusterRevision(state coordination.State) string {
	return strconv.FormatUint(state.Generation, 36) + ":" + state.Watermark
}

func metadataCorrupt(state coordination.State, found bool) bool {
	return found && (state.Generation == 0 || !validSourceVersion(state.Watermark))
}

func technicalUnavailable(err error) error {
	return bizerr.WrapCode(gerror.Wrap(err, "coordinate version cache"), CodeAuthorityUnavailable)
}
