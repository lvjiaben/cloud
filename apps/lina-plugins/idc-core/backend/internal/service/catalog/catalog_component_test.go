package catalog

import (
	"testing"

	"lina-core/pkg/bizerr"
)

func TestNormalizeComponentValueCoversFirstReleaseTypes(t *testing.T) {
	t.Parallel()
	one := 1
	two := 2
	memory := 2048
	disk := 40
	bandwidth := 100
	traffic := 1000
	unlimited := true
	cases := []struct {
		component ComponentType
		input     ComponentValueInput
		expected  string
	}{
		{ComponentRegion, ComponentValueInput{Code: "ap-east-1"}, `{"code":"ap-east-1"}`},
		{ComponentLine, ComponentValueInput{Code: "cn2"}, `{"code":"cn2"}`},
		{ComponentDatacenter, ComponentValueInput{Code: "hk-01"}, `{"code":"hk-01"}`},
		{ComponentCPU, ComponentValueInput{Code: "epyc-9654", Cores: &two}, `{"cores":2,"model_code":"epyc-9654"}`},
		{ComponentMemory, ComponentValueInput{Megabytes: &memory}, `{"megabytes":2048}`},
		{ComponentSystemDisk, ComponentValueInput{Gigabytes: &disk, Media: "nvme"}, `{"gigabytes":40,"media":"nvme"}`},
		{ComponentDataDisk, ComponentValueInput{Gigabytes: &disk, Media: "ssd"}, `{"gigabytes":40,"media":"ssd"}`},
		{ComponentBandwidth, ComponentValueInput{MegabitsPerSecond: &bandwidth}, `{"megabits_per_second":100}`},
		{ComponentTraffic, ComponentValueInput{Gigabytes: &traffic}, `{"gigabytes":1000}`},
		{ComponentTraffic, ComponentValueInput{Unlimited: &unlimited}, `{"unlimited":true}`},
		{ComponentIP, ComponentValueInput{Count: &one, Family: "dual"}, `{"count":1,"family":"dual"}`},
		{ComponentBackup, ComponentValueInput{Count: &two}, `{"count":2}`},
		{ComponentImage, ComponentValueInput{Code: "ubuntu-24.04"}, `{"code":"ubuntu-24.04"}`},
		{ComponentOther, ComponentValueInput{Code: "gpu", Quantity: &one}, `{"code":"gpu","quantity":1}`},
	}
	for _, testCase := range cases {
		actual, err := NormalizeComponentValue(testCase.component, testCase.input)
		if err != nil || actual != testCase.expected {
			t.Fatalf("component=%s actual=%q expected=%q err=%v",
				testCase.component, actual, testCase.expected, err)
		}
		if err = ValidateNormalizedComponentValue(testCase.component, actual); err != nil {
			t.Fatalf("normalized component=%s failed stored validation: %v", testCase.component, err)
		}
	}
}

func TestNormalizeComponentValueRejectsAmbiguousOrOutOfRangeInput(t *testing.T) {
	t.Parallel()
	zero := 0
	one := 1
	falseValue := false
	cases := []struct {
		component ComponentType
		input     ComponentValueInput
	}{
		{"unknown", ComponentValueInput{Code: "x"}},
		{ComponentRegion, ComponentValueInput{Code: "Hong Kong"}},
		{ComponentCPU, ComponentValueInput{Cores: &zero}},
		{ComponentMemory, ComponentValueInput{Megabytes: &one}},
		{ComponentSystemDisk, ComponentValueInput{Gigabytes: &one, Media: "tape"}},
		{ComponentTraffic, ComponentValueInput{Unlimited: &falseValue}},
		{ComponentTraffic, ComponentValueInput{Gigabytes: &one, Unlimited: &falseValue}},
		{ComponentIP, ComponentValueInput{Count: &one, Family: "any"}},
		{ComponentImage, ComponentValueInput{Code: "ubuntu", Count: &one}},
		{ComponentOther, ComponentValueInput{Code: "gpu"}},
	}
	for index, testCase := range cases {
		if _, err := NormalizeComponentValue(testCase.component, testCase.input); !bizerr.Is(err, CodeComponentInvalid) {
			t.Fatalf("case=%d component=%s returned %v", index, testCase.component, err)
		}
	}
	for index, stored := range []string{
		`{"megabytes":2048,"password":"secret"}`,
		`{"megabytes":"2048"}`,
		`{"megabytes":2048} trailing`,
	} {
		if err := ValidateNormalizedComponentValue(ComponentMemory, stored); !bizerr.Is(err, CodeComponentInvalid) {
			t.Fatalf("stored case=%d returned %v", index, err)
		}
	}
}
