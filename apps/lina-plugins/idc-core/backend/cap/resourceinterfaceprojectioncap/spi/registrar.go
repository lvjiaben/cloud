// Package spi owns startup declarations for resource-interface projection.
package spi

import (
	"context"
	"sync"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
)

type SinkFactory func(context.Context) (resourceinterfaceprojectioncap.Sink, error)
type CandidateSourceFactory func(context.Context) (resourceinterfaceprojectioncap.CandidateSource, error)

// ExistingBusinessOutboundFactory supplies the resource-owned credential boundary.
type ExistingBusinessOutboundFactory func(context.Context) (resourceinterfaceprojectioncap.ExistingBusinessOutboundSource, error)

// CatalogSourceFactory binds a direct-staff authorizer to the read-only source.
type CatalogSourceFactory func(context.Context, managementcap.Authorizer) (resourceinterfaceprojectioncap.CatalogSource, error)

// UsageReaderFactory binds the owner's database and the requester's governed
// staff authorizer once during resource-plugin startup.
type UsageReaderFactory func(context.Context, managementcap.Authorizer) (resourceinterfaceprojectioncap.UsageReader, error)

type Snapshot struct {
	sink      SinkFactory
	candidate CandidateSourceFactory
	usage     UsageReaderFactory
	catalog   CatalogSourceFactory
	outbound  ExistingBusinessOutboundFactory
}

func (snapshot Snapshot) SinkFactory() SinkFactory                       { return snapshot.sink }
func (snapshot Snapshot) CandidateSourceFactory() CandidateSourceFactory { return snapshot.candidate }

// UsageReaderFactory returns the immutable declaration-time usage factory.
func (snapshot Snapshot) UsageReaderFactory() UsageReaderFactory     { return snapshot.usage }
func (snapshot Snapshot) CatalogSourceFactory() CatalogSourceFactory { return snapshot.catalog }

var registry = struct {
	sync.Mutex
	frozen    bool
	sink      SinkFactory
	candidate CandidateSourceFactory
	usage     UsageReaderFactory
	catalog   CatalogSourceFactory
	outbound  ExistingBusinessOutboundFactory
}{}

func DeclareSink(factory SinkFactory) error {
	registry.Lock()
	defer registry.Unlock()
	if registry.frozen || factory == nil || registry.sink != nil {
		return gerror.New("resource interface projection sink declaration is invalid")
	}
	registry.sink = factory
	return nil
}

func DeclareCandidateSource(factory CandidateSourceFactory) error {
	registry.Lock()
	defer registry.Unlock()
	if registry.frozen || factory == nil || registry.candidate != nil {
		return gerror.New("resource interface candidate source declaration is invalid")
	}
	registry.candidate = factory
	return nil
}

// DeclareUsageReader publishes the product/server owner's read-only projection.
func DeclareUsageReader(factory UsageReaderFactory) error {
	registry.Lock()
	defer registry.Unlock()
	if registry.frozen || factory == nil || registry.usage != nil {
		return gerror.New("resource interface usage declaration is invalid")
	}
	registry.usage = factory
	return nil
}

// DeclareCatalogSource publishes only supplier sales reads, not draft writes.
func DeclareCatalogSource(factory CatalogSourceFactory) error {
	registry.Lock()
	defer registry.Unlock()
	if registry.frozen || factory == nil || registry.catalog != nil {
		return gerror.New("resource catalog source declaration is invalid")
	}
	registry.catalog = factory
	return nil
}

// DeclareExistingBusinessOutbound registers the source-only paid-business bridge.
func DeclareExistingBusinessOutbound(factory ExistingBusinessOutboundFactory) error {
	registry.Lock()
	defer registry.Unlock()
	if registry.frozen || factory == nil || registry.outbound != nil {
		return gerror.New("resource existing-business outbound declaration is invalid")
	}
	registry.outbound = factory
	return nil
}

// ExistingBusinessOutboundFactory returns the immutable owner bridge factory.
func (snapshot Snapshot) ExistingBusinessOutboundFactory() ExistingBusinessOutboundFactory {
	return snapshot.outbound
}

func Freeze() Snapshot {
	registry.Lock()
	defer registry.Unlock()
	registry.frozen = true
	return Snapshot{sink: registry.sink, candidate: registry.candidate, usage: registry.usage, catalog: registry.catalog, outbound: registry.outbound}
}
