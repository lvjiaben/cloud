// This file validates authoritative watermarks, canonical snapshots, and
// detached shared-state projections.

package versioncache

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"lina-core/pkg/bizerr"

	"lina-plugin-idc-core/backend/internal/service/versioncache/internal/coordination"
)

func (s *serviceImpl) loadWatermark(ctx context.Context, key Key) (string, error) {
	watermark, err := s.authority.Watermark(ctx, key)
	s.metrics.watermarkLoads.Add(1)
	if err != nil || !validSourceVersion(watermark) {
		if err == nil {
			return "", bizerr.NewCode(CodeAuthorityUnavailable)
		}
		return "", bizerr.WrapCode(err, CodeAuthorityUnavailable)
	}
	return watermark, nil
}

func (s *serviceImpl) loadAuthority(
	ctx context.Context,
	key Key,
	expectedWatermark string,
) (storedSnapshot, error) {
	snapshot, err := s.authority.Load(ctx, key)
	s.metrics.authorityLoads.Add(1)
	if err != nil {
		return storedSnapshot{}, bizerr.WrapCode(err, CodeAuthorityUnavailable)
	}
	normalized, err := s.normalizeSnapshot(snapshot)
	if err != nil {
		return storedSnapshot{}, bizerr.WrapCode(err, CodeAuthorityUnavailable)
	}
	if expectedWatermark != "" {
		if normalized.Version != expectedWatermark {
			return storedSnapshot{}, errClusterStateAdvanced
		}
		confirmed, confirmErr := s.loadWatermark(ctx, key)
		if confirmErr != nil {
			return storedSnapshot{}, confirmErr
		}
		if confirmed != expectedWatermark {
			return storedSnapshot{}, errClusterStateAdvanced
		}
	}
	return storedSnapshot{snapshot: normalized, loadedAt: s.currentTime()}, nil
}

func (s *serviceImpl) clusterSnapshot(
	state coordination.State,
	found bool,
	watermark string,
	now time.Time,
) (storedSnapshot, bool) {
	if !found || metadataCorrupt(state, found) || state.Watermark != watermark || state.Value == nil ||
		state.Value.Version != watermark || !s.validLoadedAt(state.Value.LoadedAt, now) {
		return storedSnapshot{}, false
	}
	loadedAt := state.Value.LoadedAt.UTC().Truncate(time.Millisecond)
	// A remote publisher may be ahead by AllowedClockSkew. Conservatively age
	// every remote timestamp by the full allowance; otherwise a timestamp that
	// was initially in the future could extend the hard MaximumStale window for
	// a process that first observes it later.
	loadedAt = loadedAt.Add(-s.config.AllowedClockSkew)
	if loadedAt.After(now) {
		return storedSnapshot{}, false
	}
	if now.Sub(loadedAt) > s.config.MaximumStale {
		return storedSnapshot{}, false
	}
	normalized, err := s.normalizeSnapshot(Snapshot{
		Version: state.Value.Version, Document: state.Value.Document,
	})
	if err != nil || !bytes.Equal(normalized.Document, state.Value.Document) {
		s.metrics.corruptEntries.Add(1)
		return storedSnapshot{}, false
	}
	return storedSnapshot{
		snapshot: normalized, revision: clusterRevision(state), loadedAt: loadedAt,
	}, true
}

func resultFromStored(value storedSnapshot, source Source, stale bool, degraded bool) *Result {
	return &Result{
		Snapshot: Snapshot{
			Version: value.snapshot.Version, Document: append([]byte(nil), value.snapshot.Document...),
		},
		Revision: value.revision, LoadedAt: value.loadedAt,
		Source: source, Stale: stale, Degraded: degraded,
	}
}

func (s *serviceImpl) currentTime() time.Time {
	return s.now().UTC().Truncate(time.Millisecond)
}

func (s *serviceImpl) withinStaleBound(loadedAt time.Time, now time.Time) bool {
	return s.validLoadedAt(loadedAt, now) && now.Sub(loadedAt) <= s.config.MaximumStale
}

func (s *serviceImpl) validLoadedAt(loadedAt time.Time, now time.Time) bool {
	return !loadedAt.IsZero() && !now.IsZero() && !loadedAt.After(now.Add(s.config.AllowedClockSkew))
}

func stableStorageKey(key Key) string {
	digest := sha256.Sum256([]byte(string(key.Kind) + "\x00" + key.ID))
	return hex.EncodeToString(digest[:])
}
