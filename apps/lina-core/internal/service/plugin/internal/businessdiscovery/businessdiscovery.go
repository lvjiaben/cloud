// Package businessdiscovery owns authoritative, bounded reads of release-frozen
// business-extension declarations. It never exposes persisted manifest snapshots.
package businessdiscovery

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"gopkg.in/yaml.v3"

	"lina-core/internal/service/plugin/internal/catalog"
	"lina-core/internal/service/plugin/internal/plugintypes"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/statusflag"
)

const candidateReadLimit = plugincap.MaxBusinessExtensionProviderScan + 1

type activeReleaseRow struct {
	PluginID       string `json:"plugin_id"`
	DisplayName    string `json:"display_name"`
	ReleaseVersion string `json:"release_version"`
	ReleaseDigest  string `json:"release_digest"`
	Snapshot       string `json:"manifest_snapshot"`
}

type historicalReleaseRow struct {
	PluginID       string `json:"plugin_id"`
	ReleaseVersion string `json:"release_version"`
	ReleaseDigest  string `json:"release_digest"`
	Snapshot       string `json:"manifest_snapshot"`
}

type activeReleaseLoader func(context.Context, int) ([]*activeReleaseRow, error)
type historicalReleaseLoader func(context.Context, []historicalReleaseKey) ([]*historicalReleaseRow, error)

type historicalReleaseKey struct {
	pluginID       string
	releaseVersion string
	releaseDigest  string
}

type manifestSnapshot struct {
	ID                           string                                   `yaml:"id"`
	Name                         string                                   `yaml:"name"`
	Dependencies                 *plugintypes.DependencySpec              `yaml:"dependencies,omitempty"`
	AuthorizedBusinessExtensions []*capmodel.BusinessExtensionDeclaration `yaml:"businessExtensions,omitempty"`
}

// ListNewBindingProviders performs one collection query for authoritative
// registry plus active-release snapshots, then filters the frozen declarations.
func ListNewBindingProviders(
	ctx context.Context,
	ownerPluginID string,
	input plugincap.BusinessExtensionProviderListInput,
) (*capmodel.PageResult[*capmodel.BusinessExtensionProviderEvidence], error) {
	return listNewBindingProviders(ctx, ownerPluginID, input, loadActiveReleaseRows)
}

func listNewBindingProviders(
	ctx context.Context,
	ownerPluginID string,
	input plugincap.BusinessExtensionProviderListInput,
	load activeReleaseLoader,
) (*capmodel.PageResult[*capmodel.BusinessExtensionProviderEvidence], error) {
	owner := strings.TrimSpace(ownerPluginID)
	point := strings.TrimSpace(input.Point)
	version := strings.TrimSpace(input.ContractVersion)
	if owner == "" || point == "" {
		return nil, gerror.New("business extension owner and point are required")
	}
	rows, err := load(ctx, candidateReadLimit)
	if err != nil {
		return nil, gerror.Wrap(err, "list business extension provider snapshots failed")
	}
	if len(rows) > plugincap.MaxBusinessExtensionProviderScan {
		return nil, gerror.New("business extension provider snapshot exceeds bounded scan limit")
	}
	items := make([]*capmodel.BusinessExtensionProviderEvidence, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			return nil, gerror.New("business extension provider snapshot row is missing")
		}
		matches, parseErr := evidenceFromSnapshot(row.PluginID, row.DisplayName, row.ReleaseVersion, row.ReleaseDigest, row.Snapshot, owner, point, version)
		if parseErr != nil {
			return nil, parseErr
		}
		items = append(items, matches...)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].PluginID != items[j].PluginID {
			return items[i].PluginID < items[j].PluginID
		}
		return items[i].ContractVersion < items[j].ContractVersion
	})
	pageNum, pageSize := input.Page.Normalize()
	start := len(items)
	if pageNum <= len(items)/pageSize+1 {
		start = (pageNum - 1) * pageSize
	}
	if start > len(items) {
		start = len(items)
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return &capmodel.PageResult[*capmodel.BusinessExtensionProviderEvidence]{
		Items: append([]*capmodel.BusinessExtensionProviderEvidence(nil), items[start:end]...),
		Total: len(items),
	}, nil
}

func loadActiveReleaseRows(ctx context.Context, limit int) ([]*activeReleaseRow, error) {
	rows := make([]*activeReleaseRow, 0, limit)
	err := g.DB().GetScan(ctx, &rows, `
SELECT p.plugin_id, p.name AS display_name, r.release_version, r.checksum AS release_digest, r.manifest_snapshot
FROM sys_plugin p
INNER JOIN sys_plugin_release r ON r.id = p.release_id AND r.plugin_id = p.plugin_id
WHERE p.deleted_at IS NULL AND r.deleted_at IS NULL
  AND p.installed = ? AND p.status = ? AND p.current_state = ?
  AND p.new_business_state = ? AND r.status = ?
ORDER BY p.plugin_id ASC
LIMIT ?`,
		statusflag.Installed.Int(),
		statusflag.EnabledValue.Int(),
		plugintypes.HostStateEnabled.String(),
		plugintypes.NewBusinessStateAccepting.String(),
		plugintypes.ReleaseStatusActive.String(),
		limit,
	)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// GetExactProvider verifies one retained historical release and declaration.
// Current install, enablement, and new-business state intentionally do not apply.
func GetExactProvider(ctx context.Context, ownerPluginID string, input plugincap.BusinessExtensionProviderExactInput) (*capmodel.BusinessExtensionProviderEvidence, error) {
	items, err := GetExactProviders(ctx, ownerPluginID, []plugincap.BusinessExtensionProviderExactInput{input})
	if err != nil {
		return nil, err
	}
	return items[0], nil
}

// GetExactProviders verifies a bounded exact-evidence collection with one
// collection query, while preserving the owner constraint and input order.
func GetExactProviders(ctx context.Context, ownerPluginID string, inputs []plugincap.BusinessExtensionProviderExactInput) ([]*capmodel.BusinessExtensionProviderEvidence, error) {
	return getExactProviders(ctx, ownerPluginID, inputs, loadHistoricalReleaseRows)
}

func getExactProviders(ctx context.Context, ownerPluginID string, inputs []plugincap.BusinessExtensionProviderExactInput, load historicalReleaseLoader) ([]*capmodel.BusinessExtensionProviderEvidence, error) {
	owner := strings.TrimSpace(ownerPluginID)
	if owner == "" || len(inputs) < 1 || len(inputs) > plugincap.MaxBusinessExtensionProviderExactBatch {
		return nil, gerror.New("bounded business extension historical evidence is required")
	}
	keys := make([]historicalReleaseKey, 0, len(inputs))
	seen := make(map[historicalReleaseKey]struct{}, len(inputs))
	for _, input := range inputs {
		if !exactInputComplete(input) {
			return nil, gerror.New("complete business extension historical evidence is required")
		}
		key := historicalReleaseKey{pluginID: strings.TrimSpace(input.PluginID), releaseVersion: strings.TrimSpace(input.ReleaseVersion), releaseDigest: strings.TrimSpace(input.ReleaseDigest)}
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			keys = append(keys, key)
		}
	}
	rows, err := load(ctx, keys)
	if err != nil {
		return nil, gerror.Wrap(err, "get historical business extension provider snapshots failed")
	}
	if len(rows) > len(keys) {
		return nil, gerror.New("historical business extension provider snapshot result is invalid")
	}
	byRelease := make(map[historicalReleaseKey]*historicalReleaseRow, len(rows))
	for _, row := range rows {
		if row == nil {
			return nil, gerror.New("historical business extension provider snapshot row is missing")
		}
		key := historicalReleaseKey{pluginID: strings.TrimSpace(row.PluginID), releaseVersion: strings.TrimSpace(row.ReleaseVersion), releaseDigest: strings.TrimSpace(row.ReleaseDigest)}
		if _, requested := seen[key]; !requested || byRelease[key] != nil {
			return nil, gerror.New("historical business extension provider snapshot result is invalid")
		}
		byRelease[key] = row
	}
	result := make([]*capmodel.BusinessExtensionProviderEvidence, len(inputs))
	for index, input := range inputs {
		key := historicalReleaseKey{pluginID: strings.TrimSpace(input.PluginID), releaseVersion: strings.TrimSpace(input.ReleaseVersion), releaseDigest: strings.TrimSpace(input.ReleaseDigest)}
		row := byRelease[key]
		if row == nil {
			continue
		}
		items, parseErr := evidenceFromSnapshot(row.PluginID, "", row.ReleaseVersion, row.ReleaseDigest, row.Snapshot, owner, strings.TrimSpace(input.Point), strings.TrimSpace(input.ContractVersion))
		if parseErr != nil {
			return nil, parseErr
		}
		for _, item := range items {
			if item.DeclarationDigest == strings.TrimSpace(input.DeclarationDigest) {
				result[index] = item
				break
			}
		}
	}
	return result, nil
}

func loadHistoricalReleaseRows(ctx context.Context, keys []historicalReleaseKey) ([]*historicalReleaseRow, error) {
	var predicate strings.Builder
	args := make([]any, 0, len(keys)*3)
	for index, key := range keys {
		if index > 0 {
			predicate.WriteString(" OR ")
		}
		predicate.WriteString("(plugin_id = ? AND release_version = ? AND checksum = ?)")
		args = append(args, key.pluginID, key.releaseVersion, key.releaseDigest)
	}
	rows := make([]*historicalReleaseRow, 0, len(keys))
	err := g.DB().GetScan(ctx, &rows, `SELECT plugin_id, release_version, checksum AS release_digest, manifest_snapshot
FROM sys_plugin_release WHERE deleted_at IS NULL AND (`+predicate.String()+`)`, args...)
	return rows, err
}

// GetActiveProviderForAdmission reloads one exact active provider while its
// lifecycle lock is held. Optional evidence fields, when supplied, must match.
func GetActiveProviderForAdmission(ctx context.Context, ownerPluginID string, input plugincap.BusinessExtensionProviderExactInput, method string) (*capmodel.BusinessExtensionProviderEvidence, error) {
	owner := strings.TrimSpace(ownerPluginID)
	if owner == "" || strings.TrimSpace(input.PluginID) == "" || strings.TrimSpace(input.Point) == "" || strings.TrimSpace(input.ContractVersion) == "" {
		return nil, gerror.New("complete business extension admission identity is required")
	}
	rows := make([]*activeReleaseRow, 0, 1)
	err := g.DB().GetScan(ctx, &rows, `
SELECT p.plugin_id, p.name AS display_name, r.release_version, r.checksum AS release_digest, r.manifest_snapshot
FROM sys_plugin p
INNER JOIN sys_plugin_release r ON r.id = p.release_id AND r.plugin_id = p.plugin_id
WHERE p.deleted_at IS NULL AND r.deleted_at IS NULL AND p.plugin_id = ?
  AND p.installed = ? AND p.status = ? AND p.current_state = ?
  AND p.new_business_state = ? AND r.status = ?
LIMIT 1`, strings.TrimSpace(input.PluginID), statusflag.Installed.Int(), statusflag.EnabledValue.Int(), plugintypes.HostStateEnabled.String(), plugintypes.NewBusinessStateAccepting.String(), plugintypes.ReleaseStatusActive.String())
	if err != nil {
		return nil, gerror.Wrap(err, "get active business extension provider snapshot failed")
	}
	if len(rows) != 1 {
		return nil, nil
	}
	items, err := evidenceFromSnapshot(rows[0].PluginID, rows[0].DisplayName, rows[0].ReleaseVersion, rows[0].ReleaseDigest, rows[0].Snapshot, owner, strings.TrimSpace(input.Point), strings.TrimSpace(input.ContractVersion))
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if strings.TrimSpace(method) != "" && !contains(item.AuthorizedMethods, strings.TrimSpace(method)) {
			continue
		}
		if strings.TrimSpace(input.ReleaseVersion) != "" && item.ReleaseVersion != strings.TrimSpace(input.ReleaseVersion) {
			continue
		}
		if strings.TrimSpace(input.ReleaseDigest) != "" && item.ReleaseDigest != strings.TrimSpace(input.ReleaseDigest) {
			continue
		}
		if strings.TrimSpace(input.DeclarationDigest) != "" && item.DeclarationDigest != strings.TrimSpace(input.DeclarationDigest) {
			continue
		}
		return item, nil
	}
	return nil, nil
}

func evidenceFromSnapshot(pluginID, displayName, releaseVersion, releaseDigest, content, owner, point, version string) ([]*capmodel.BusinessExtensionProviderEvidence, error) {
	if strings.TrimSpace(releaseDigest) == "" || strings.TrimSpace(content) == "" {
		return nil, gerror.Newf("business extension release snapshot is incomplete: %s", strings.TrimSpace(pluginID))
	}
	snapshot := &manifestSnapshot{}
	if err := catalog.ValidateYAMLBusinessExtensionSnapshotSchema([]byte(content)); err != nil {
		return nil, gerror.Wrapf(err, "business extension release snapshot contains unsupported fields: %s", strings.TrimSpace(pluginID))
	}
	if err := yaml.Unmarshal([]byte(content), snapshot); err != nil {
		return nil, gerror.Wrapf(err, "parse business extension release snapshot failed: %s", strings.TrimSpace(pluginID))
	}
	if pluginID != strings.TrimSpace(pluginID) || plugintypes.ValidatePluginID(pluginID) != nil || snapshot.ID != pluginID {
		return nil, gerror.Newf("business extension snapshot plugin identity is invalid: %s", strings.TrimSpace(pluginID))
	}
	if releaseVersion != strings.TrimSpace(releaseVersion) || plugintypes.ValidateManifestSemanticVersion(releaseVersion) != nil {
		return nil, gerror.Newf("business extension snapshot release version is invalid: %s", strings.TrimSpace(pluginID))
	}
	validated := catalog.CloneBusinessExtensionDeclarations(snapshot.AuthorizedBusinessExtensions)
	if err := catalog.ValidateBusinessExtensions(&catalog.Manifest{ID: snapshot.ID, Dependencies: plugintypes.CloneDependencySpec(snapshot.Dependencies), BusinessExtensions: validated}); err != nil {
		return nil, gerror.Wrapf(err, "business extension snapshot declaration is invalid: %s", pluginID)
	}
	if !businessExtensionDeclarationsAlreadyCanonical(snapshot.AuthorizedBusinessExtensions, validated) {
		return nil, gerror.Newf("business extension snapshot declaration is not canonical: %s", pluginID)
	}
	if strings.TrimSpace(displayName) == "" {
		displayName = strings.TrimSpace(snapshot.Name)
	}
	if strings.TrimSpace(displayName) == "" {
		return nil, gerror.Newf("business extension provider display name is missing: %s", strings.TrimSpace(pluginID))
	}
	out := make([]*capmodel.BusinessExtensionProviderEvidence, 0, 1)
	for _, declaration := range snapshot.AuthorizedBusinessExtensions {
		if declaration == nil || declaration.Role != capmodel.BusinessExtensionRoleProvider ||
			strings.TrimSpace(declaration.Owner) != owner || strings.TrimSpace(declaration.Point) != point ||
			(version != "" && strings.TrimSpace(declaration.Version) != version) {
			continue
		}
		methods := make([]string, len(declaration.Methods))
		copy(methods, declaration.Methods)
		sort.Strings(methods)
		normalizedDeclaration := catalog.CloneBusinessExtensionDeclarations([]*capmodel.BusinessExtensionDeclaration{declaration})[0]
		normalizedDeclaration.Methods = methods
		digest := declarationDigest(normalizedDeclaration)
		if digest == "" {
			return nil, gerror.Newf("build business extension declaration digest failed: %s", strings.TrimSpace(pluginID))
		}
		out = append(out, &capmodel.BusinessExtensionProviderEvidence{
			PluginID: strings.TrimSpace(pluginID), DisplayName: strings.TrimSpace(displayName), ReleaseVersion: strings.TrimSpace(releaseVersion),
			Point: point, ContractVersion: strings.TrimSpace(declaration.Version),
			AuthorizedMethods: methods, ReleaseDigest: strings.TrimSpace(releaseDigest),
			DeclarationDigest:                 digest,
			InterfaceConfiguration:            normalizedDeclaration.InterfaceConfiguration,
			InterfaceConfigurationCardinality: normalizedDeclaration.InterfaceConfigurationCardinality,
			ProductConfiguration:              normalizedDeclaration.ProductConfiguration,
			CustomerConfiguration:             normalizedDeclaration.CustomerConfiguration,
			ProviderMetadata:                  normalizedDeclaration.ProviderMetadata,
		})
	}
	return out, nil
}

func businessExtensionDeclarationsAlreadyCanonical(original, validated []*capmodel.BusinessExtensionDeclaration) bool {
	if len(original) != len(validated) {
		return false
	}
	for index := range original {
		left, right := original[index], validated[index]
		leftContent, leftErr := json.Marshal(left)
		rightContent, rightErr := json.Marshal(right)
		if left == nil || right == nil || leftErr != nil || rightErr != nil || !bytes.Equal(leftContent, rightContent) {
			return false
		}
	}
	return true
}

func declarationDigest(declaration *capmodel.BusinessExtensionDeclaration) string {
	payload, err := json.Marshal(declaration)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

func exactInputComplete(input plugincap.BusinessExtensionProviderExactInput) bool {
	return strings.TrimSpace(input.PluginID) != "" && strings.TrimSpace(input.ReleaseVersion) != "" && strings.TrimSpace(input.Point) != "" && strings.TrimSpace(input.ContractVersion) != "" && strings.TrimSpace(input.ReleaseDigest) != "" && strings.TrimSpace(input.DeclarationDigest) != ""
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
