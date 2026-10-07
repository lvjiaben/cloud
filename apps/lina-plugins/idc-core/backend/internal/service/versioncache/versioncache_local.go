// This file owns bounded process-local snapshots, revisions, and rebuild gates.

package versioncache

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

type storedSnapshot struct {
	snapshot Snapshot
	revision string
	loadedAt time.Time
}

func cloneStored(value storedSnapshot) storedSnapshot {
	value.snapshot.Document = append([]byte(nil), value.snapshot.Document...)
	return value
}

type localStore struct {
	mutex       sync.Mutex
	entries     map[string]storedSnapshot
	revisions   map[string]string
	revisionUse map[string]uint64
	maximum     int
	maxBytes    int
	usedBytes   int
	revisionNo  atomic.Uint64
	touchNo     uint64
}

func (s *localStore) initialize(maximum int, maxBytes int) {
	s.entries = make(map[string]storedSnapshot)
	s.revisions = make(map[string]string)
	s.revisionUse = make(map[string]uint64)
	s.maximum = maximum
	s.maxBytes = maxBytes
}

func (s *localStore) get(key string) (storedSnapshot, bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	value, ok := s.entries[key]
	return cloneStored(value), ok
}

func (s *localStore) set(key string, value storedSnapshot) {
	value = cloneStored(value)
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if existing, ok := s.entries[key]; ok {
		s.usedBytes -= storedSnapshotBytes(existing)
	}
	s.entries[key] = value
	s.usedBytes += storedSnapshotBytes(value)
	for len(s.entries) > s.maximum || s.usedBytes > s.maxBytes {
		oldestKey := ""
		var oldest time.Time
		for candidateKey, candidate := range s.entries {
			if candidateKey == key && len(s.entries) > 1 {
				continue
			}
			if oldestKey == "" || candidate.loadedAt.Before(oldest) {
				oldestKey = candidateKey
				oldest = candidate.loadedAt
			}
		}
		if oldestKey == "" {
			break
		}
		s.usedBytes -= storedSnapshotBytes(s.entries[oldestKey])
		delete(s.entries, oldestKey)
	}
}

func (s *localStore) invalidate(key string) string {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if existing, ok := s.entries[key]; ok {
		s.usedBytes -= storedSnapshotBytes(existing)
		delete(s.entries, key)
	}
	revision := "l" + base36(s.revisionNo.Add(1))
	s.revisions[key] = revision
	s.touchRevision(key)
	s.pruneRevisions(key)
	return revision
}

func (s *localStore) revision(key string) string {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if revision := s.revisions[key]; revision != "" {
		s.touchRevision(key)
		return revision
	}
	return initialRevision
}

func (s *localStore) setRevision(key string, revision string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.revisions[key] = revision
	s.touchRevision(key)
	s.pruneRevisions(key)
}

func (s *localStore) touchRevision(key string) {
	s.touchNo++
	s.revisionUse[key] = s.touchNo
}

// pruneRevisions bounds invalidation-only key cardinality. Dropping a local
// revision can only cause an authoritative cache miss: invalidate already
// removed that key's local snapshot, and an entry whose revision is dropped no
// longer matches initialRevision.
func (s *localStore) pruneRevisions(preserve string) {
	for len(s.revisions) > s.maximum {
		candidate := ""
		candidateUse := ^uint64(0)
		for key, used := range s.revisionUse {
			if key == preserve {
				continue
			}
			if _, hasEntry := s.entries[key]; hasEntry {
				continue
			}
			if candidate == "" || used < candidateUse {
				candidate = key
				candidateUse = used
			}
		}
		if candidate == "" {
			for key, used := range s.revisionUse {
				if key == preserve {
					continue
				}
				if candidate == "" || used < candidateUse {
					candidate = key
					candidateUse = used
				}
			}
		}
		if candidate == "" {
			return
		}
		delete(s.revisions, candidate)
		delete(s.revisionUse, candidate)
	}
}

func storedSnapshotBytes(value storedSnapshot) int {
	return len(value.snapshot.Document) + len(value.snapshot.Version) + len(value.revision) + 64
}

func base36(value uint64) string {
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyz"
	var encoded [13]byte
	position := len(encoded)
	for {
		position--
		encoded[position] = alphabet[value%36]
		value /= 36
		if value == 0 {
			return string(encoded[position:])
		}
	}
}

type keyedGate struct {
	semaphore chan struct{}
	refs      int
}

type keyedGates struct {
	mutex sync.Mutex
	items map[string]*keyedGate
}

func (g *keyedGates) initialize() {
	g.items = make(map[string]*keyedGate)
}

func (g *keyedGates) acquire(ctx context.Context, key string) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	g.mutex.Lock()
	gate := g.items[key]
	if gate == nil {
		gate = &keyedGate{semaphore: make(chan struct{}, 1)}
		gate.semaphore <- struct{}{}
		g.items[key] = gate
	}
	gate.refs++
	g.mutex.Unlock()

	select {
	case <-ctx.Done():
		g.releaseReference(key, gate)
		return nil, ctx.Err()
	case <-gate.semaphore:
		if err := ctx.Err(); err != nil {
			gate.semaphore <- struct{}{}
			g.releaseReference(key, gate)
			return nil, err
		}
		return func() {
			gate.semaphore <- struct{}{}
			g.releaseReference(key, gate)
		}, nil
	}
}

func (g *keyedGates) releaseReference(key string, gate *keyedGate) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	gate.refs--
	if gate.refs == 0 && g.items[key] == gate {
		delete(g.items, key)
	}
}
