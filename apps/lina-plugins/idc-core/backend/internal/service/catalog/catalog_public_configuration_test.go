// This file verifies public revision opacity and authoritative selection semantics.

package catalog

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func testPublicConfigurationGraph(t *testing.T) PublicConfigurationGraph {
	t.Helper()
	maximum := 2
	codec, err := newPublicConfigurationRevisionCodec(bytes.Repeat([]byte{0x71}, 32))
	if err != nil {
		t.Fatalf("create public configuration revision codec: %v", err)
	}
	revision, err := codec.encode("starter-cloud", "ver_private_1")
	if err != nil {
		t.Fatalf("encode public configuration revision: %v", err)
	}
	return PublicConfigurationGraph{
		Revision: revision, ConfigurationMode: ConfigurationConfigurable,
		Groups: []PublicConfigurationGroup{
			{
				GroupNo: "group-cpu", SelectionMode: SelectionSingle,
				NameZhCN: "处理器", NameEnUS: "CPU", MinimumSelected: 1, MaximumSelected: 1,
				Options: []PublicConfigurationOption{
					{OptionNo: "cpu-basic", ComponentType: "cpu", NameZhCN: "2 核", NameEnUS: "2 cores", MinimumQuantity: 1, MaximumQuantity: 1, DefaultQuantity: 1},
					{OptionNo: "cpu-pro", ComponentType: "cpu", NameZhCN: "4 核", NameEnUS: "4 cores", MinimumQuantity: 0, MaximumQuantity: 1},
				},
			},
			{
				GroupNo: "group-memory", SelectionMode: SelectionSingle,
				NameZhCN: "内存", NameEnUS: "Memory", MinimumSelected: 1, MaximumSelected: 1,
				Options: []PublicConfigurationOption{
					{OptionNo: "memory-basic", ComponentType: "memory", NameZhCN: "4 GB", NameEnUS: "4 GB", MinimumQuantity: 1, MaximumQuantity: 1, DefaultQuantity: 1},
					{OptionNo: "memory-pro", ComponentType: "memory", NameZhCN: "8 GB", NameEnUS: "8 GB", MinimumQuantity: 0, MaximumQuantity: 2},
				},
			},
		},
		Rules: []PublicCompatibilityRule{
			{RuleType: CompatibilityRequires, SourceOptionNo: "cpu-pro", TargetOptionNo: "memory-pro", MinimumTargetQuantity: 1, MaximumTargetQuantity: &maximum},
			{RuleType: CompatibilityExcludes, SourceOptionNo: "cpu-basic", TargetOptionNo: "memory-pro"},
		},
	}
}

func TestPublicConfigurationCacheIsBounded(t *testing.T) {
	cache := newPublicConfigurationCache()
	for index := 0; index <= maximumPublicConfigurationCacheEntries; index++ {
		cache.put(PublicConfigurationGraph{Revision: fmt.Sprintf("revision-%03d", index)})
	}
	if _, exists := cache.get("revision-000"); exists {
		t.Fatal("public configuration cache retained its oldest entry beyond the bound")
	}
	if graph, exists := cache.get(fmt.Sprintf("revision-%03d", maximumPublicConfigurationCacheEntries)); !exists || graph.Revision == "" {
		t.Fatal("public configuration cache lost its newest entry")
	}
	metrics := cache.snapshotMetrics()
	if metrics.Hits != 1 || metrics.Misses != 1 ||
		metrics.Writes != maximumPublicConfigurationCacheEntries+1 || metrics.Evictions != 1 {
		t.Fatalf("public configuration cache metrics=%+v", metrics)
	}
}

func TestPublicConfigurationCacheDropsMalformedEntries(t *testing.T) {
	cache := newPublicConfigurationCache()
	malformed := cache.order.PushFront("malformed")
	cache.entries["malformed"] = malformed

	if _, exists := cache.get("malformed"); exists {
		t.Fatal("public configuration cache exposed a malformed entry")
	}
	if cache.order.Len() != 0 || len(cache.entries) != 0 {
		t.Fatal("public configuration cache did not remove a malformed entry")
	}
	if metrics := cache.snapshotMetrics(); metrics.Misses != 1 || metrics.Corruptions != 1 {
		t.Fatalf("malformed public configuration cache metrics=%+v", metrics)
	}
}

func TestPublicConfigurationRevisionIsOpaqueAndVersionBound(t *testing.T) {
	codec, err := newPublicConfigurationRevisionCodec(bytes.Repeat([]byte{0x71}, 32))
	if err != nil {
		t.Fatalf("create public configuration revision codec: %v", err)
	}
	revision, err := codec.encode("starter-cloud", "ver_private_1")
	if err != nil {
		t.Fatalf("encode public configuration revision: %v", err)
	}
	if strings.Contains(revision, "private") || !codec.verify(revision, "starter-cloud", "ver_private_1") {
		t.Fatalf("public configuration revision is not opaque and valid: %q", revision)
	}
	if codec.verify(revision, "starter-cloud", "ver_private_2") ||
		codec.verify(revision, "other-cloud", "ver_private_1") {
		t.Fatal("public configuration revision was reusable across product or version")
	}
	term, err := codec.encodeQuoteTerm("starter-cloud", "ver_private_1", "billing_private_1")
	if err != nil || strings.Contains(term, "private") || !strings.HasPrefix(term, "term_") {
		t.Fatalf("public quote term is not opaque: term=%q err=%v", term, err)
	}
	other, err := codec.encodeQuoteTerm("starter-cloud", "ver_private_2", "billing_private_1")
	if err != nil || other == term {
		t.Fatal("public quote term was reusable across versions")
	}
}

func TestPublicConfigurationSelectionValidation(t *testing.T) {
	graph := testPublicConfigurationGraph(t)
	if err := validatePublicConfigurationGraph(&graph); err != nil {
		t.Fatalf("valid public configuration graph rejected: %v", err)
	}
	valid, err := evaluatePublicConfiguration(graph, map[string]int{
		"cpu-basic": 1, "memory-basic": 1,
	})
	if err != nil || !valid.Valid || len(valid.Violations) != 0 {
		t.Fatalf("valid public configuration result=%+v err=%v", valid, err)
	}
	requires, err := evaluatePublicConfiguration(graph, map[string]int{
		"cpu-pro": 1, "memory-basic": 1,
	})
	if err != nil || requires.Valid || len(requires.Violations) != 1 ||
		requires.Violations[0].Code != PublicConfigurationViolationRequiresTarget {
		t.Fatalf("requires result=%+v err=%v", requires, err)
	}
	excludes, err := evaluatePublicConfiguration(graph, map[string]int{
		"cpu-basic": 1, "memory-pro": 1,
	})
	if err != nil || excludes.Valid || len(excludes.Violations) != 1 ||
		excludes.Violations[0].Code != PublicConfigurationViolationExcludesTarget {
		t.Fatalf("excludes result=%+v err=%v", excludes, err)
	}
	group, err := evaluatePublicConfiguration(graph, map[string]int{"cpu-basic": 1})
	if err != nil || group.Valid || len(group.Violations) != 1 ||
		group.Violations[0].Code != PublicConfigurationViolationGroupSelection {
		t.Fatalf("group selection result=%+v err=%v", group, err)
	}
}

func TestPublicConfigurationSelectionRejectsMalformedInput(t *testing.T) {
	for _, input := range [][]PublicConfigurationSelection{
		{{OptionNo: "cpu-basic", Quantity: 0}},
		{{OptionNo: "?", Quantity: 1}},
		{{OptionNo: "cpu-basic", Quantity: 1}, {OptionNo: "cpu-basic", Quantity: 1}},
	} {
		if _, err := normalizePublicConfigurationSelections(input); err == nil {
			t.Fatalf("malformed public configuration selection was accepted: %+v", input)
		}
	}
}

func TestPublicConfigurationProjectionRejectsUnsafeLabels(t *testing.T) {
	graph := testPublicConfigurationGraph(t)
	graph.Groups[0].Options[0].NameEnUS = "\x00"
	if err := validatePublicConfigurationGraph(&graph); err == nil {
		t.Fatal("public configuration projection accepted an unsafe label")
	}
}
