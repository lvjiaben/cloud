package productautomation

import (
	"bytes"
	"errors"
	"strings"
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

func TestConfigurationPriceCandidatesExposeQuantityWithoutProviderKeys(t *testing.T) {
	label := &capmodel.LocalizedText{ZhCN: "数据盘容量", EnUS: "Data disk size"}
	configuration := &VersionConfiguration{Version: 3, CustomerDeclaration: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "private_disk", Type: capmodel.ConfigurationFieldInteger, Label: label, Minimum: "0", Maximum: "1000"}}}}
	candidates, err := configurationPriceCandidates("ver_11111111111111111111111111111111", configuration)
	if err != nil || len(candidates) != 1 || !candidates[0].Quantity || candidates[0].CandidateLabel.ZhCN != "数据盘容量" {
		t.Fatalf("invalid quantity candidate: %+v %v", candidates, err)
	}
	if strings.Contains(candidates[0].CandidateToken, "private_disk") {
		t.Fatal("provider field key leaked")
	}
	if candidates[0].candidateValue != configurationQuantityPriceIdentity {
		t.Fatal("quantity identity unstable")
	}
}

func TestBooleanPriceCandidatesUsePackedLabels(t *testing.T) {
	configuration := &VersionConfiguration{Version: 1, CustomerDeclaration: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "local_support", Type: capmodel.ConfigurationFieldBoolean, Label: &capmodel.LocalizedText{ZhCN: "支持", EnUS: "Support"}}}}}
	if _, err := configurationPriceCandidates("version", configuration); err == nil {
		t.Fatal("missing label resources were silently replaced")
	}
	labels := map[bool]capmodel.LocalizedText{false: {ZhCN: "否", EnUS: "No"}, true: {ZhCN: "是", EnUS: "Yes"}}
	candidates, err := configurationPriceCandidates("version", configuration, labels)
	if err != nil || len(candidates) != 2 || candidates[0].Quantity || candidates[0].candidateValue != "false" || candidates[0].CandidateLabel.ZhCN != "否" || candidates[1].CandidateLabel.EnUS != "Yes" {
		t.Fatalf("boolean prices lost typed choices or labels: %+v %v", candidates, err)
	}
}
