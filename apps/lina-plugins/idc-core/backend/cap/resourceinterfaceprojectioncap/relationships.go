// Choice relationships describe a bounded, secret-free directory graph. They
// belong to commerce's projection contract, not to a fixed cloud product type.
package resourceinterfaceprojectioncap

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gogf/gf/v2/errors/gerror"
)

// ChoiceRelationship limits one field's choices using another declared field.
// Links are a complete relation for the returned child directory. A missing
// parent value has no permitted children; multiple parents intersect their
// permitted choices. Absence of the relationship does not assert a relation.
// ParentValue is deliberately separate from the
// same-field tree hierarchy represented by Candidate.ParentValue.
type ChoiceRelationship struct {
	ParentFieldKey string
	ChildFieldKey  string
	Links          []ChoiceLink
}

// ChoiceLink identifies one allowed parent/child pair using opaque directory
// values. Labels, endpoints and authentication material are not part of a link.
type ChoiceLink struct {
	ParentValue string
	ChildValue  string
}

// ValidateRelationships checks bounded graph shape and returned choice
// membership without network access. A parent directory may be absent from a
// partial query; child membership must always be provable in this result.
// Legacy results with no relationships remain valid.
func (result *CandidateResult) ValidateRelationships() error {
	if result == nil || len(result.Fields) > 128 || len(result.Relationships) > 16 {
		return gerror.New("candidate relationship graph exceeds its bound")
	}
	fields := make(map[string]map[string]bool, len(result.Fields))
	for _, field := range result.Fields {
		values := make(map[string]bool, len(field.Items))
		for _, item := range field.Items {
			values[item.Value] = true
		}
		fields[field.FieldKey] = values
	}
	var (
		children = make(map[string][]string, len(result.Relationships))
		pairs    = make(map[[2]string]bool, len(result.Relationships))
		total    int
	)
	for _, relation := range result.Relationships {
		if !validRelationshipText(relation.ParentFieldKey) || !validRelationshipText(relation.ChildFieldKey) || relation.ParentFieldKey == relation.ChildFieldKey || fields[relation.ChildFieldKey] == nil || len(relation.Links) == 0 {
			return gerror.New("candidate relationship identity is invalid")
		}
		pair := [2]string{relation.ParentFieldKey, relation.ChildFieldKey}
		if pairs[pair] {
			return gerror.New("candidate relationship is duplicated")
		}
		pairs[pair] = true
		children[relation.ChildFieldKey] = append(children[relation.ChildFieldKey], relation.ParentFieldKey)
		seen := make(map[ChoiceLink]bool, len(relation.Links))
		for _, link := range relation.Links {
			total++
			if total > 8192 || !validRelationshipText(link.ParentValue) || !validRelationshipText(link.ChildValue) || !fields[relation.ChildFieldKey][link.ChildValue] || seen[link] {
				return gerror.New("candidate relationship link is invalid")
			}
			if parents, included := fields[relation.ParentFieldKey]; included && !parents[link.ParentValue] {
				return gerror.New("candidate relationship parent is outside the returned directory")
			}
			seen[link] = true
		}
	}
	var visit func(string, map[string]bool) bool
	visit = func(current string, path map[string]bool) bool {
		if path[current] {
			return false
		}
		path[current] = true
		for _, parent := range children[current] {
			if !visit(parent, path) {
				return false
			}
		}
		delete(path, current)
		return true
	}
	for child := range children {
		if !visit(child, make(map[string]bool, len(children))) {
			return gerror.New("candidate relationship graph contains a cycle")
		}
	}
	return nil
}

func validRelationshipText(value string) bool {
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > 256 || strings.TrimSpace(value) != value {
		return false
	}
	for _, char := range value {
		if unicode.IsControl(char) {
			return false
		}
	}
	return true
}
