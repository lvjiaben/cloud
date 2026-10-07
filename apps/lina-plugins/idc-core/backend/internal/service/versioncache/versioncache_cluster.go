// This file coordinates cross-node rebuild ownership, bounded contention, and
// atomic fenced publication through the dedicated coordination component.

package versioncache

import (
	"context"
	"errors"
	"time"

	"lina-core/pkg/bizerr"

	"lina-plugin-idc-core/backend/internal/service/versioncache/internal/coordination"
)

func (s *serviceImpl) rebuildClustered(
	ctx context.Context,
	key Key,
	storageKey string,
	expectedWatermark string,
	prior storedSnapshot,
) (*Result, error) {
	releaseLocal, gateErr := s.gates.acquire(ctx, storageKey)
	if gateErr != nil {
		return nil, bizerr.WrapCode(gateErr, CodeAuthorityUnavailable)
	}
	defer releaseLocal()
	if latest, found := s.local.get(storageKey); found {
		prior = latest
	}
	watermark, state, found, prepareErr := s.currentClusterState(ctx, key, storageKey)
	if prepareErr != nil {
		return s.degradedAtWatermarkLocked(ctx, key, storageKey, prior, expectedWatermark)
	}
	if watermark != expectedWatermark {
		return nil, errClusterStateAdvanced
	}
	if shared, usable := s.clusterSnapshot(state, found, watermark, s.currentTime()); usable {
		s.local.set(storageKey, shared)
		s.metrics.sharedHits.Add(1)
		return resultFromStored(shared, SourceShared, false, false), nil
	}

	guard, contentionExpiry, acquireErr := s.coordination.Acquire(ctx, storageKey)
	if acquireErr != nil {
		if ctx.Err() != nil {
			return nil, bizerr.WrapCode(ctx.Err(), CodeAuthorityUnavailable)
		}
		s.metrics.backendFailures.Add(1)
		return s.degradedAtWatermarkLocked(ctx, key, storageKey, prior, watermark)
	}
	if guard == nil {
		s.metrics.contentions.Add(1)
		winner, winnerFound, waitErr := s.waitForClusterWinner(
			ctx, key, storageKey, watermark, contentionExpiry,
		)
		if waitErr != nil {
			if errors.Is(waitErr, errClusterStateAdvanced) {
				return nil, waitErr
			}
			if ctx.Err() != nil {
				return nil, bizerr.WrapCode(ctx.Err(), CodeAuthorityUnavailable)
			}
			s.metrics.backendFailures.Add(1)
			return s.degradedAtWatermarkLocked(ctx, key, storageKey, prior, watermark)
		}
		if winnerFound {
			s.local.set(storageKey, winner)
			s.metrics.sharedHits.Add(1)
			return resultFromStored(winner, SourceShared, false, false), nil
		}
		guard, _, acquireErr = s.coordination.Acquire(ctx, storageKey)
		if acquireErr != nil || guard == nil {
			if ctx.Err() != nil {
				return nil, bizerr.WrapCode(ctx.Err(), CodeAuthorityUnavailable)
			}
			s.metrics.backendFailures.Add(1)
			return s.degradedAtWatermarkLocked(ctx, key, storageKey, prior, watermark)
		}
	}
	return s.rebuildWithGuard(ctx, key, storageKey, prior, guard)
}

func (s *serviceImpl) rebuildWithGuard(
	ctx context.Context,
	key Key,
	storageKey string,
	prior storedSnapshot,
	guard *coordination.Guard,
) (*Result, error) {
	defer func() {
		if err := guard.Release(ctx); err != nil {
			s.metrics.lockReleaseFailures.Add(1)
		}
	}()
	result, operationErr := s.rebuildWhileOwned(key, storageKey, prior, guard)
	stopErr := guard.StopAndCheck()
	if operationErr != nil {
		return nil, operationErr
	}
	if stopErr != nil {
		s.metrics.backendFailures.Add(1)
		return s.staleOrUnavailable(prior)
	}
	return result, nil
}

func (s *serviceImpl) rebuildWhileOwned(
	key Key,
	storageKey string,
	prior storedSnapshot,
	guard *coordination.Guard,
) (*Result, error) {
	ctx := guard.Context()
	if err := guard.Check(); err != nil {
		return nil, technicalUnavailable(err)
	}
	watermark, state, found, stateErr := s.currentClusterState(ctx, key, storageKey)
	if stateErr != nil {
		s.metrics.backendFailures.Add(1)
		loaded, loadErr := s.loadAuthority(ctx, key, "")
		if loadErr != nil {
			return s.staleOrUnavailable(prior)
		}
		loaded.revision = degradedRevision
		s.local.set(storageKey, loaded)
		s.metrics.rebuilds.Add(1)
		s.metrics.degradedReads.Add(1)
		return resultFromStored(loaded, SourceAuthority, false, true), nil
	}
	if shared, usable := s.clusterSnapshot(state, found, watermark, s.currentTime()); usable {
		s.local.set(storageKey, shared)
		s.metrics.sharedHits.Add(1)
		return resultFromStored(shared, SourceShared, false, false), nil
	}
	loaded, loadErr := s.loadAuthority(ctx, key, watermark)
	if loadErr != nil {
		if errors.Is(loadErr, errClusterStateAdvanced) {
			return nil, loadErr
		}
		return s.staleOrUnavailable(prior)
	}
	published, publishErr := guard.Publish(ctx, storageKey, state, watermark, coordination.Value{
		Version: loaded.snapshot.Version, Document: loaded.snapshot.Document, LoadedAt: loaded.loadedAt,
	})
	if publishErr != nil {
		if errors.Is(publishErr, coordination.ErrStateChanged) || errors.Is(publishErr, coordination.ErrLeaseLost) {
			return nil, errClusterStateAdvanced
		}
		s.metrics.backendFailures.Add(1)
		loaded.revision = degradedRevision
		s.local.set(storageKey, loaded)
		s.metrics.rebuilds.Add(1)
		s.metrics.degradedReads.Add(1)
		return resultFromStored(loaded, SourceAuthority, false, true), nil
	}
	shared, usable := s.clusterSnapshot(published, true, watermark, s.currentTime())
	if !usable {
		s.metrics.corruptEntries.Add(1)
		return nil, bizerr.NewCode(CodeAuthorityUnavailable)
	}
	s.local.set(storageKey, shared)
	s.metrics.rebuilds.Add(1)
	return resultFromStored(shared, SourceAuthority, false, false), nil
}

func (s *serviceImpl) currentClusterState(
	ctx context.Context,
	key Key,
	storageKey string,
) (string, coordination.State, bool, error) {
	watermark, watermarkErr := s.loadWatermark(ctx, key)
	if watermarkErr != nil {
		return "", coordination.State{}, false, watermarkErr
	}
	state, found, stateErr := s.coordination.Read(ctx, storageKey)
	if stateErr != nil {
		return "", coordination.State{}, false, stateErr
	}
	if metadataCorrupt(state, found) {
		s.metrics.corruptEntries.Add(1)
		state, stateErr = s.coordination.Advance(ctx, storageKey, watermark)
		if stateErr != nil {
			return "", coordination.State{}, false, stateErr
		}
		found = true
	}
	return watermark, state, found, nil
}

func (s *serviceImpl) waitForClusterWinner(
	ctx context.Context,
	key Key,
	storageKey string,
	watermark string,
	contentionExpiry time.Time,
) (storedSnapshot, bool, error) {
	now := time.Now()
	deadline := contentionExpiry
	maximumDeadline := now.Add(s.config.LockLease)
	if deadline.After(maximumDeadline) {
		deadline = maximumDeadline
	}
	if !deadline.After(now) {
		return storedSnapshot{}, false, nil
	}
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	ticker := time.NewTicker(contentionPollEvery)
	defer ticker.Stop()
	for {
		state, found, err := s.coordination.Read(ctx, storageKey)
		if err != nil {
			return storedSnapshot{}, false, err
		}
		if shared, usable := s.clusterSnapshot(state, found, watermark, s.currentTime()); usable {
			confirmed, confirmErr := s.loadWatermark(ctx, key)
			if confirmErr != nil {
				return storedSnapshot{}, false, confirmErr
			}
			if confirmed != watermark {
				return storedSnapshot{}, false, errClusterStateAdvanced
			}
			return shared, true, nil
		}
		select {
		case <-ctx.Done():
			return storedSnapshot{}, false, ctx.Err()
		case <-timer.C:
			return storedSnapshot{}, false, nil
		case <-ticker.C:
		}
	}
}

func (s *serviceImpl) degradedAtWatermarkLocked(
	ctx context.Context,
	key Key,
	storageKey string,
	prior storedSnapshot,
	watermark string,
) (*Result, error) {
	if latest, found := s.local.get(storageKey); found {
		prior = latest
	}
	if prior.snapshot.Version == watermark && s.withinStaleBound(prior.loadedAt, s.currentTime()) {
		s.metrics.degradedReads.Add(1)
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
