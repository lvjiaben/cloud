package catalog

import "testing"

func TestConfigurationPriceSelectionDigestIsOrderIndependentAndRejectsDuplicates(t *testing.T) {
	left := []ConfigurationPriceSelection{
		{DimensionNo: "cpd_22222222222222222222222222222222", Quantity: 2},
		{DimensionNo: "cpd_11111111111111111111111111111111", Quantity: 1},
	}
	right := []ConfigurationPriceSelection{left[1], left[0]}
	leftDigest, err := ConfigurationPriceSelectionDigest(left)
	if err != nil {
		t.Fatal(err)
	}
	rightDigest, err := ConfigurationPriceSelectionDigest(right)
	if err != nil || leftDigest != rightDigest || len(leftDigest) != 64 {
		t.Fatalf("stable digest left=%q right=%q err=%v", leftDigest, rightDigest, err)
	}
	if _, err = ConfigurationPriceSelectionDigest([]ConfigurationPriceSelection{left[0], left[0]}); err == nil {
		t.Fatal("duplicate configuration price dimension was accepted")
	}
}
