package catalog

import (
	"testing"

	"lina-core/pkg/bizerr"
)

func TestValidateCategoryParentChainRejectsSelfDescendantAndExistingCycle(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		parents  map[string]string
		category string
		parent   string
	}{
		{name: "self", parents: map[string]string{"a": ""}, category: "a", parent: "a"},
		{name: "descendant", parents: map[string]string{"a": "", "b": "a", "c": "b"}, category: "a", parent: "c"},
		{name: "existing cycle", parents: map[string]string{"a": "b", "b": "a", "c": ""}, category: "c", parent: "a"},
	}
	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if err := validateCategoryParentChain(testCase.parents, testCase.category, testCase.parent); !bizerr.Is(err, CodeCategoryParentCycle) {
				t.Fatalf("expected category-cycle error, got %v", err)
			}
		})
	}
}

func TestValidateCategoryParentChainAcceptsBoundedAncestorChain(t *testing.T) {
	t.Parallel()
	parents := map[string]string{"root": "", "branch": "root", "leaf": "branch", "moving": "root"}
	if err := validateCategoryParentChain(parents, "moving", "leaf"); err != nil {
		t.Fatalf("valid parent chain rejected: %v", err)
	}
}

func TestValidateCategoryParentChainRejectsMissingParent(t *testing.T) {
	t.Parallel()
	if err := validateCategoryParentChain(map[string]string{"root": ""}, "root", "missing"); !bizerr.Is(err, CodeManagementNotAccessible) {
		t.Fatalf("expected inaccessible-parent error, got %v", err)
	}
}
