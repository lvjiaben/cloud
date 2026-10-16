// This file defines safe dynamic import candidates with both portable schema
// values and purchase tokens. Internal provider keys never cross this boundary.

package channelcommercecap

import "context"

type ImportCandidatesInput struct {
	CustomerNo            string          `json:"-"`
	PublicSlug            string          `json:"publicSlug"`
	ConfigurationRevision string          `json:"configurationRevision"`
	Values                []PurchaseValue `json:"values"`
}

type ImportCandidates struct {
	ConfigurationRevision string                 `json:"configurationRevision"`
	Fields                []ImportCandidateField `json:"fields"`
}

type ImportCandidateField struct {
	FieldKey   string            `json:"fieldKey"`
	FieldToken string            `json:"fieldToken"`
	Options    []ImportCandidate `json:"options"`
}

type ImportCandidate struct {
	Value       string        `json:"value"`
	OptionToken string        `json:"optionToken"`
	ParentValue string        `json:"parentValue"`
	Label       LocalizedText `json:"label"`
}

type ImportCandidatesRuntime interface {
	ReadImportCandidates(context.Context, ImportCandidatesInput) (*ImportCandidates, error)
}
