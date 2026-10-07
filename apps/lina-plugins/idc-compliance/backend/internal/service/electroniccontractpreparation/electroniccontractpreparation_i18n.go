// This file loads the exact contract-artifact language bundle from the
// plugin manifest so the frozen contract language never depends on the
// administrator's current interface language.

package electroniccontractpreparation

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/manifestcap"
)

const (
	artifactLocaleEnglish = "en-US"
	artifactLocaleChinese = "zh-CN"
	artifactTextPrefix    = "plugin.idc-compliance.contract.artifact."
)

var artifactTextKeys = []string{
	artifactTextPrefix + "serviceTable.headers.service",
	artifactTextPrefix + "serviceTable.headers.term",
	artifactTextPrefix + "serviceTable.headers.quantity",
	artifactTextPrefix + "serviceTable.headers.lineAmount",
	artifactTextPrefix + "serviceTerm.once.one",
	artifactTextPrefix + "serviceTerm.once.other",
	artifactTextPrefix + "serviceTerm.hour.one",
	artifactTextPrefix + "serviceTerm.hour.other",
	artifactTextPrefix + "serviceTerm.day.one",
	artifactTextPrefix + "serviceTerm.day.other",
	artifactTextPrefix + "serviceTerm.month.one",
	artifactTextPrefix + "serviceTerm.month.other",
	artifactTextPrefix + "serviceTerm.year.one",
	artifactTextPrefix + "serviceTerm.year.other",
}

// ArtifactTextCatalog is the immutable, exact-locale text used in the
// provider-bound contract artifact. It intentionally does not follow the
// current request locale.
type ArtifactTextCatalog struct {
	values map[string]map[string]string
}

// LoadArtifactTextCatalog reads both supported contract languages through the
// plugin-scoped manifest capability and fails closed when any required text is
// absent.
func LoadArtifactTextCatalog(ctx context.Context, manifest manifestcap.Service) (ArtifactTextCatalog, error) {
	if ctx == nil || ctx.Err() != nil || manifest == nil {
		return ArtifactTextCatalog{}, gerror.New("contract artifact translations are unavailable")
	}
	paths := []string{
		"i18n/en-US/electronic-contract.json",
		"i18n/zh-CN/electronic-contract.json",
	}
	output, err := manifest.GetMany(ctx, manifestcap.GetManyInput{Paths: paths})
	if err != nil {
		return ArtifactTextCatalog{}, gerror.Wrap(err, "load contract artifact translations")
	}
	if output == nil || len(output.MissingPaths) != 0 || len(output.Resources) != len(paths) {
		return ArtifactTextCatalog{}, gerror.New("contract artifact translation resources are incomplete")
	}
	resources := make(map[string][]byte, len(output.Resources))
	for _, resource := range output.Resources {
		if resource == nil || len(resource.Body) == 0 {
			return ArtifactTextCatalog{}, gerror.New("contract artifact translation resource is empty")
		}
		resources[resource.Path] = resource.Body
	}
	return parseArtifactTextCatalog(resources)
}

func parseArtifactTextCatalog(resources map[string][]byte) (ArtifactTextCatalog, error) {
	values := make(map[string]map[string]string, 2)
	for _, locale := range []string{artifactLocaleEnglish, artifactLocaleChinese} {
		path := "i18n/" + locale + "/electronic-contract.json"
		var catalog map[string]string
		if json.Unmarshal(resources[path], &catalog) != nil {
			return ArtifactTextCatalog{}, gerror.New("contract artifact translation resource is invalid")
		}
		selected := make(map[string]string, len(artifactTextKeys))
		for _, key := range artifactTextKeys {
			value := strings.TrimSpace(catalog[key])
			if value == "" {
				return ArtifactTextCatalog{}, gerror.New("contract artifact translation is incomplete")
			}
			selected[key] = value
		}
		values[locale] = selected
	}
	return ArtifactTextCatalog{values: values}, nil
}

func (c ArtifactTextCatalog) valid() bool {
	if len(c.values) != 2 {
		return false
	}
	for _, locale := range []string{artifactLocaleEnglish, artifactLocaleChinese} {
		if len(c.values[locale]) != len(artifactTextKeys) {
			return false
		}
		for _, key := range artifactTextKeys {
			if strings.TrimSpace(c.values[locale][key]) == "" {
				return false
			}
		}
	}
	return true
}

func (c ArtifactTextCatalog) text(locale, key string) (string, error) {
	if !c.valid() || strings.TrimSpace(c.values[locale][key]) == "" {
		return "", gerror.New("contract artifact translation is unavailable")
	}
	return c.values[locale][key], nil
}
