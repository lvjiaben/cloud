package productautomation

import (
	"bytes"
	"errors"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

func TestConfigurationPriceCandidateTokenBindsConfigurationVersion(t *testing.T) {
	first := configurationPriceCandidateToken("ver_11111111111111111111111111111111", 1, "field", "value")
	second := configurationPriceCandidateToken("ver_11111111111111111111111111111111", 2, "field", "value")
	if first == second || len(first) != len("candidate_")+64 {
		t.Fatalf("candidate tokens are not version-bound: %q %q", first, second)
	}
}

func TestConfigurationPriceCandidatesFreezeResolvedDynamicLabel(t *testing.T) {
	fieldLabel := &capmodel.LocalizedText{ZhCN: "地区", EnUS: "Region"}
	candidateLabel := &capmodel.LocalizedText{ZhCN: "华东", EnUS: "East China"}
	configuration := &VersionConfiguration{
		Version: 3,
		CustomerDeclaration: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{
			Key: "region", Type: capmodel.ConfigurationFieldSingleSelect, Label: fieldLabel,
		}}},
		CustomerResolvedCandidates: configurationcontract.ResolvedCandidates{
			"region": []*capmodel.ConfigurationCandidate{{Value: "east", Label: candidateLabel}},
		},
	}
	candidates, err := configurationPriceCandidates("ver_11111111111111111111111111111111", configuration)
	if err != nil || len(candidates) != 1 || candidates[0].CandidateLabel.ZhCN != "华东" || candidates[0].candidateValue != "east" {
		t.Fatalf("dynamic price candidates=%+v err=%v", candidates, err)
	}
}

type partialEntropyReader struct{ used bool }

func (reader *partialEntropyReader) Read(target []byte) (int, error) {
	if reader.used {
		return 0, errors.New("entropy ended")
	}
	reader.used = true
	return copy(target, bytes.Repeat([]byte{1}, 8)), nil
}

func TestConfigurationPriceDimensionNumberRequiresCompleteEntropy(t *testing.T) {
	if _, err := newConfigurationPriceDimensionNo(&partialEntropyReader{}); err == nil {
		t.Fatal("partial entropy was accepted")
	}
}
