package productautomation

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/cap/supplierinterfacecap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

const (
	dynamicCandidateCacheTTL     = 30 * time.Second
	dynamicCandidateCacheEntries = 256
)

type dynamicCandidateCacheEntry struct {
	key       string
	expiresAt time.Time
	value     configurationcontract.ResolvedCandidates
}

func (s *serviceImpl) loadVersionConfigurationRow(ctx context.Context, versionNo string) (*entity.CatalogProductVersionConfiguration, error) {
	var row *entity.CatalogProductVersionConfiguration
	err := s.database.Model(do.CatalogProductVersionConfiguration{}).Ctx(ctx).Where(do.CatalogProductVersionConfiguration{
		TenantId: idccap.DefaultTenantID, VersionNo: versionNo,
	}).Scan(&row)
	return row, err
}

type dynamicCandidateInflight struct {
	done    chan struct{}
	value   configurationcontract.ResolvedCandidates
	err     error
	waiters int
}

// dynamicCandidateCache is a non-authoritative acceleration cache. Exact
// immutable release evidence, method, and the complete request digest are in
// every key. Independent cluster nodes may refresh separately; the maximum
// observable staleness is the fixed 30-second TTL and any miss safely calls the
// governed provider again.
type dynamicCandidateCache struct {
	mu       sync.Mutex
	entries  map[string]*list.Element
	order    *list.List
	inflight map[string]*dynamicCandidateInflight
	now      func() time.Time
}

func newDynamicCandidateCache() *dynamicCandidateCache {
	return &dynamicCandidateCache{
		entries: make(map[string]*list.Element, dynamicCandidateCacheEntries),
		order:   list.New(), inflight: make(map[string]*dynamicCandidateInflight), now: time.Now,
	}
}

func (c *dynamicCandidateCache) get(key string) (configurationcontract.ResolvedCandidates, bool) {
	if c == nil || key == "" {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.getLocked(key)
}

func (c *dynamicCandidateCache) getLocked(key string) (configurationcontract.ResolvedCandidates, bool) {
	element := c.entries[key]
	if element == nil {
		return nil, false
	}
	entry, ok := element.Value.(dynamicCandidateCacheEntry)
	if !ok || !c.now().Before(entry.expiresAt) {
		delete(c.entries, key)
		c.order.Remove(element)
		return nil, false
	}
	c.order.MoveToFront(element)
	return cloneResolvedCandidates(entry.value), true
}

func (c *dynamicCandidateCache) load(
	ctx context.Context,
	key string,
	loader func() (configurationcontract.ResolvedCandidates, error),
) (value configurationcontract.ResolvedCandidates, err error) {
	if c == nil || key == "" {
		return loader()
	}
	if value, ok := c.get(key); ok {
		return value, nil
	}
	c.mu.Lock()
	if value, ok := c.getLocked(key); ok {
		c.mu.Unlock()
		return value, nil
	}
	if current := c.inflight[key]; current != nil {
		current.waiters++
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-current.done:
			return cloneResolvedCandidates(current.value), current.err
		}
	}
	call := &dynamicCandidateInflight{done: make(chan struct{})}
	c.inflight[key] = call
	c.mu.Unlock()

	defer func() {
		if recover() != nil {
			value = nil
			err = gerror.New("plugin.idc-core.product_automation.dynamic_candidates.unavailable")
		}
		if err == nil {
			c.put(key, value)
		}
		c.mu.Lock()
		call.value = cloneResolvedCandidates(value)
		call.err = err
		delete(c.inflight, key)
		close(call.done)
		c.mu.Unlock()
	}()
	return loader()
}

func (c *dynamicCandidateCache) put(key string, value configurationcontract.ResolvedCandidates) {
	if c == nil || key == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := dynamicCandidateCacheEntry{key: key, expiresAt: c.now().Add(dynamicCandidateCacheTTL), value: cloneResolvedCandidates(value)}
	if current := c.entries[key]; current != nil {
		current.Value = entry
		c.order.MoveToFront(current)
		return
	}
	c.entries[key] = c.order.PushFront(entry)
	if c.order.Len() <= dynamicCandidateCacheEntries {
		return
	}
	oldest := c.order.Back()
	if oldest == nil {
		return
	}
	if expired, ok := oldest.Value.(dynamicCandidateCacheEntry); ok {
		delete(c.entries, expired.key)
	}
	c.order.Remove(oldest)
}

func (s *serviceImpl) ResolvePublicPurchaseCandidates(
	ctx context.Context,
	versionNo string,
	schema *capmodel.ConfigurationSchemaDeclaration,
	values []configurationcontract.ConfigurationValue,
) (configurationcontract.ResolvedCandidates, error) {
	available, err := s.CheckPublicPurchaseAvailability(ctx, versionNo)
	if err != nil || !available {
		if err != nil {
			return nil, err
		}
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	binding, err := s.loadBinding(ctx, versionNo, false)
	if err != nil {
		return nil, gerror.Wrap(err, "load dynamic candidate binding")
	}
	if binding == nil {
		return nil, gerror.New("dynamic candidate binding is unavailable")
	}
	exact, err := s.extensions.GetProviderExact(ctx, plugincap.BusinessExtensionProviderExactInput{
		PluginID: binding.PluginId, ReleaseVersion: binding.PluginReleaseVersion,
		Point: binding.ExtensionPointCode, ContractVersion: binding.ContractVersion,
		ReleaseDigest: binding.ReleaseDigest, DeclarationDigest: binding.DeclarationDigest,
	})
	if err != nil || !sameExactEvidence(bindingEvidence(binding), exact) {
		return nil, bizerr.NewCode(CodeEvidenceUnavailable)
	}
	configuration, err := s.loadVersionConfigurationRow(ctx, versionNo)
	if err != nil || configuration == nil || configuration.InterfaceInstanceNo != binding.InterfaceInstanceNo || configuration.InterfaceConfigurationRevision < 1 {
		return nil, bizerr.NewCode(CodeEvidenceUnavailable)
	}
	customerSchema, schemaErr := s.importedCustomerSchema(ctx, binding, configuration, exact)
	if schemaErr != nil || !sameExactConfiguration(schema, customerSchema) {
		return nil, bizerr.NewCode(CodeEvidenceUnavailable)
	}
	return s.resolveDynamicCandidates(ctx, exact, schema, values, configurationcontract.SurfaceCustomer, dynamicCandidateScope{
		versionNo: versionNo, interfaceInstanceNo: binding.InterfaceInstanceNo, configurationRevision: configuration.InterfaceConfigurationRevision,
	})
}

func (s *serviceImpl) resolveDynamicCandidates(
	ctx context.Context,
	exact *capmodel.BusinessExtensionProviderEvidence,
	schema *capmodel.ConfigurationSchemaDeclaration,
	values []configurationcontract.ConfigurationValue,
	surface configurationcontract.ConfigurationSurface,
	scope dynamicCandidateScope,
) (configurationcontract.ResolvedCandidates, error) {
	if exact.PluginID == portableImportProviderID && surface == configurationSurfaceCustomer && scope.versionNo != "" {
		return s.resolveImportedCandidates(ctx, exact, schema, values, scope)
	}
	batches, err := configurationcontract.BuildDynamicCandidateBatches(schema, values, surface, exact.AuthorizedMethods)
	if err != nil {
		return nil, err
	}
	resourceOwned, err := s.resourceInterfaceOwned(ctx, scope.interfaceInstanceNo)
	if err != nil {
		return nil, err
	}
	var runtimeView *supplierinterfacecap.ConfigurationView
	if !resourceOwned && len(batches) > 0 && (s.database != nil || s.runtimeConfigurationLoader != nil) {
		loader := s.runtimeConfigurationLoader
		if loader == nil {
			loader = s.loadRuntimeConfigurationView
		}
		runtimeView, err = loader(ctx, exact, scope)
		if err != nil {
			return nil, err
		}
	}
	resolved := make(configurationcontract.ResolvedCandidates)
	for _, batch := range batches {
		cacheKey := dynamicCandidateCacheKey(exact, scope, batch.Method, batch.Payload)
		batchResolved, loadErr := s.dynamicCandidates.load(ctx, cacheKey, func() (configurationcontract.ResolvedCandidates, error) {
			if resourceOwned {
				return s.resolveResourceCandidates(ctx, scope.interfaceInstanceNo, scope.resourceConfigurationRevision, batch.Method, batch.Payload, batch.Queries)
			}
			runtimeContext, contextErr := s.runtimeContexts.issue(exact, scope, capmodel.BusinessExtensionInvocationPurposeNewBusiness, batch.Method, runtimeView)
			if contextErr != nil {
				return nil, gerror.Wrap(contextErr, "issue supplier runtime context")
			}
			defer s.runtimeContexts.remove(runtimeContext.id)
			result, invokeErr := s.extensions.InvokeProviderMethod(ctx, pluginhost.BusinessExtensionMethodInvokeInput{
				PluginID: exact.PluginID, Point: exact.Point, ContractVersion: exact.ContractVersion,
				Method: batch.Method, ReleaseVersion: exact.ReleaseVersion,
				ReleaseDigest: exact.ReleaseDigest, DeclarationDigest: exact.DeclarationDigest,
				Purpose:        capmodel.BusinessExtensionInvocationPurposeNewBusiness,
				RuntimeContext: &capmodel.BusinessExtensionRuntimeContext{ContextID: runtimeContext.id}, Payload: batch.Payload,
			})
			if invokeErr != nil {
				return nil, gerror.Wrap(invokeErr, "invoke dynamic candidate method")
			}
			if result == nil || result.Version != configurationcontract.DynamicCandidateProtocolV1 {
				return nil, gerror.New("dynamic candidate method returned an invalid protocol version")
			}
			decoded, invokeErr := configurationcontract.DecodeDynamicCandidateResponse(result.Payload, batch.Queries)
			if invokeErr != nil {
				return nil, invokeErr
			}
			return decoded, nil
		})
		if loadErr != nil {
			return nil, loadErr
		}
		for fieldKey, candidates := range batchResolved {
			if _, duplicate := resolved[fieldKey]; duplicate {
				return nil, gerror.New("dynamic candidate field was resolved more than once")
			}
			resolved[fieldKey] = candidates
		}
	}
	return resolved, nil
}

func (s *serviceImpl) resourceInterfaceOwned(ctx context.Context, interfaceNo string) (bool, error) {
	if s.database == nil || interfaceNo == "" {
		return false, nil
	}
	value, err := s.database.GetValue(ctx, `SELECT interface_origin_code FROM plugin_idc_core_product_automation_interface_instance WHERE tenant_id=0 AND interface_instance_no=? LIMIT 1`, interfaceNo)
	if err != nil {
		return false, err
	}
	return value.String() == "resource", nil
}

func (s *serviceImpl) resolveResourceCandidates(ctx context.Context, interfaceNo, resourceRevision, method string, payload []byte, queries []configurationcontract.DynamicCandidateQuery) (configurationcontract.ResolvedCandidates, error) {
	if s.resourceCandidates == nil {
		return nil, gerror.New("resource interface candidate source is unavailable")
	}
	request := resourceinterfaceprojectioncap.CandidateRequest{InterfaceInstanceNo: interfaceNo, ExpectedConfigurationRevision: resourceRevision, Method: method, Values: payload}
	if resourceinterfaceprojectioncap.ValidateCandidateRequest(request) != nil {
		return nil, gerror.New("resource interface candidate request is invalid")
	}
	result, err := s.resourceCandidates.ResolveCandidates(ctx, request)
	if err != nil {
		return nil, gerror.Wrap(err, "resolve resource interface candidates")
	}
	if result == nil {
		return nil, gerror.New("resource interface candidate result is unavailable")
	}
	allowed := make(map[string]struct{}, len(queries))
	for _, query := range queries {
		allowed[query.FieldKey] = struct{}{}
	}
	resolved := make(configurationcontract.ResolvedCandidates, len(result.Fields))
	for _, field := range result.Fields {
		if _, ok := allowed[field.FieldKey]; !ok || len(field.Items) > 512 {
			return nil, gerror.New("resource interface candidate result is invalid")
		}
		if _, duplicate := resolved[field.FieldKey]; duplicate {
			return nil, gerror.New("resource interface candidate field is duplicated")
		}
		items := make([]*capmodel.ConfigurationCandidate, len(field.Items))
		seenValues := make(map[string]struct{}, len(field.Items))
		for index, item := range field.Items {
			if item.Value == "" || len(item.Value) > 256 || item.LabelZhCN == "" || len(item.LabelZhCN) > 256 || item.LabelEnUS == "" || len(item.LabelEnUS) > 256 {
				return nil, gerror.New("resource interface candidate item is invalid")
			}
			if _, duplicate := seenValues[item.Value]; duplicate {
				return nil, gerror.New("resource interface candidate value is duplicated")
			}
			seenValues[item.Value] = struct{}{}
			items[index] = &capmodel.ConfigurationCandidate{Value: item.Value, Label: &capmodel.LocalizedText{ZhCN: item.LabelZhCN, EnUS: item.LabelEnUS}}
		}
		resolved[field.FieldKey] = items
	}
	return resolved, nil
}

func dynamicCandidateCacheKey(exact *capmodel.BusinessExtensionProviderEvidence, scope dynamicCandidateScope, method string, payload []byte) string {
	digest := sha256.New()
	for _, part := range []string{
		exact.PluginID, exact.Point, exact.ContractVersion, exact.ReleaseVersion,
		exact.ReleaseDigest, exact.DeclarationDigest, method,
		scope.interfaceInstanceNo, fmt.Sprintf("%d", scope.configurationRevision),
		scope.resourceConfigurationRevision,
	} {
		_, _ = digest.Write([]byte(part))
		_, _ = digest.Write([]byte{0})
	}
	payloadDigest := sha256.Sum256(payload)
	_, _ = digest.Write(payloadDigest[:])
	return hex.EncodeToString(digest.Sum(nil))
}

func cloneResolvedCandidates(value configurationcontract.ResolvedCandidates) configurationcontract.ResolvedCandidates {
	result := make(configurationcontract.ResolvedCandidates, len(value))
	for fieldKey, candidates := range value {
		cloned := make([]*capmodel.ConfigurationCandidate, len(candidates))
		for index, candidate := range candidates {
			if candidate == nil {
				continue
			}
			copyCandidate := *candidate
			if candidate.Label != nil {
				copyLabel := *candidate.Label
				copyCandidate.Label = &copyLabel
			}
			cloned[index] = &copyCandidate
		}
		result[fieldKey] = cloned
	}
	return result
}
