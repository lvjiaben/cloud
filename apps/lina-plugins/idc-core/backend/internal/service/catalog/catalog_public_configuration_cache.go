// This file keeps immutable published graphs out of the database hot path after discovery.

package catalog

import (
	"container/list"
	"sync"
	"sync/atomic"
)

const maximumPublicConfigurationCacheEntries = 256

type publicConfigurationCacheEntry struct {
	revision string
	graph    PublicConfigurationGraph
}

type publicConfigurationCache struct {
	mu      sync.Mutex
	entries map[string]*list.Element
	order   *list.List
	metrics publicConfigurationCacheMetricCounters
}

type publicConfigurationCacheMetricCounters struct {
	hits        atomic.Uint64
	misses      atomic.Uint64
	writes      atomic.Uint64
	evictions   atomic.Uint64
	corruptions atomic.Uint64
}

func newPublicConfigurationCache() *publicConfigurationCache {
	return &publicConfigurationCache{
		entries: make(map[string]*list.Element, maximumPublicConfigurationCacheEntries),
		order:   list.New(),
	}
}

func (c *publicConfigurationCache) get(revision string) (PublicConfigurationGraph, bool) {
	if c == nil || revision == "" {
		return PublicConfigurationGraph{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	element, exists := c.entries[revision]
	if !exists {
		c.metrics.misses.Add(1)
		return PublicConfigurationGraph{}, false
	}
	entry, valid := element.Value.(publicConfigurationCacheEntry)
	if !valid {
		c.metrics.misses.Add(1)
		c.metrics.corruptions.Add(1)
		delete(c.entries, revision)
		c.order.Remove(element)
		return PublicConfigurationGraph{}, false
	}
	c.order.MoveToFront(element)
	c.metrics.hits.Add(1)
	return entry.graph, true
}

func (c *publicConfigurationCache) put(graph PublicConfigurationGraph) {
	if c == nil || graph.Revision == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics.writes.Add(1)
	if existing, exists := c.entries[graph.Revision]; exists {
		existing.Value = publicConfigurationCacheEntry{revision: graph.Revision, graph: graph}
		c.order.MoveToFront(existing)
		return
	}
	element := c.order.PushFront(publicConfigurationCacheEntry{revision: graph.Revision, graph: graph})
	c.entries[graph.Revision] = element
	if c.order.Len() <= maximumPublicConfigurationCacheEntries {
		return
	}
	oldest := c.order.Back()
	if oldest == nil {
		return
	}
	c.order.Remove(oldest)
	if entry, valid := oldest.Value.(publicConfigurationCacheEntry); valid {
		delete(c.entries, entry.revision)
		c.metrics.evictions.Add(1)
		return
	}
	c.metrics.corruptions.Add(1)
	for revision, element := range c.entries {
		if element == oldest {
			delete(c.entries, revision)
			c.metrics.evictions.Add(1)
			return
		}
	}
}

func (c *publicConfigurationCache) snapshotMetrics() PublicConfigurationCacheMetrics {
	if c == nil {
		return PublicConfigurationCacheMetrics{}
	}
	return PublicConfigurationCacheMetrics{
		Hits: c.metrics.hits.Load(), Misses: c.metrics.misses.Load(),
		Writes: c.metrics.writes.Load(), Evictions: c.metrics.evictions.Load(),
		Corruptions: c.metrics.corruptions.Load(),
	}
}
