// This file provides the fail-closed validation boundary that a later
// fulfillment worker can call before creating or delivering a service. It
// validates frozen generic customer configuration only; it does not implement
// the phase-0.11 task state machine.

package productautomation

import (
	"context"
	"encoding/hex"
	"encoding/json"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
	"lina-plugin-idc-core/backend/internal/service/purchaseconfigurationfile"
)

// FrozenCustomerConfigurationInput is immutable evidence copied by the
// checkout/order boundary. Dynamic candidates are snapshots, never refreshed
// during fulfillment, so stopping new sales cannot change an existing order.
type FrozenCustomerConfigurationInput struct {
	OrderNo                        string
	CustomerNo                     string
	UploaderUserID                 int
	VersionNo                      string
	InterfaceInstanceNo            string
	InterfaceConfigurationRevision int64
	CustomerDeclarationDigest      string
	CandidateSnapshotDigest        string
	CustomerValueDigest            string
	ConfigurationRevision          string
	Values                         []ConfigurationValue
	Candidates                     configurationcontract.ResolvedCandidates
}

type FrozenCustomerConfigurationEvidence struct {
	CustomerValueDigest     string
	CandidateSnapshotDigest string
	VerifiedFiles           []purchaseconfigurationfile.VerifiedField
}

type FrozenCustomerConfigurationValidator interface {
	// Validate rechecks exact version, interface revision, declaration and
	// typed values, then delegates all file-token ownership checks in one batch.
	Validate(context.Context, FrozenCustomerConfigurationInput) (*FrozenCustomerConfigurationEvidence, error)
	ValidateBatch(context.Context, []FrozenCustomerConfigurationInput) ([]FrozenCustomerConfigurationEvidence, error)
}

type frozenCustomerConfigurationValidator struct {
	database   gdb.DB
	extensions frozenProviderEvidenceReader
	files      purchaseconfigurationfile.OrderEvidenceVerifier
}

type frozenProviderEvidenceReader interface {
	GetProvidersExact(context.Context, []plugincap.BusinessExtensionProviderExactInput) ([]*capmodel.BusinessExtensionProviderEvidence, error)
}

func NewFrozenCustomerConfigurationValidator(
	database gdb.DB,
	extensions frozenProviderEvidenceReader,
	files purchaseconfigurationfile.OrderEvidenceVerifier,
) (FrozenCustomerConfigurationValidator, error) {
	if nilDependency(database) || nilDependency(extensions) || nilDependency(files) {
		return nil, gerror.New("frozen customer configuration validation requires database, extension registry, and file verifier")
	}
	return &frozenCustomerConfigurationValidator{database: database, extensions: extensions, files: files}, nil
}

func (s *frozenCustomerConfigurationValidator) Validate(
	ctx context.Context,
	input FrozenCustomerConfigurationInput,
) (*FrozenCustomerConfigurationEvidence, error) {
	result, err := s.ValidateBatch(ctx, []FrozenCustomerConfigurationInput{input})
	if err != nil {
		return nil, err
	}
	if len(result) != 1 {
		return nil, gerror.New("frozen customer configuration evidence is unavailable")
	}
	return &result[0], nil
}

func configurationDeclarationDigest(schema *capmodel.ConfigurationSchemaDeclaration) string {
	encoded, err := configurationDeclarationJSON(schema)
	if err != nil {
		return ""
	}
	return configurationDigest(encoded)
}

func configurationStoredJSONDigest(value []byte) string {
	var document any
	if err := json.Unmarshal(value, &document); err != nil {
		return ""
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return ""
	}
	return configurationDigest(encoded)
}

func frozenCustomerFileInputs(
	input FrozenCustomerConfigurationInput,
	schema *capmodel.ConfigurationSchemaDeclaration,
) ([]purchaseconfigurationfile.VerifyInput, error) {
	fields := make(map[string]*capmodel.ConfigurationFieldDeclaration)
	policies := make(map[string]*capmodel.ConfigurationUploadPolicyDeclaration)
	if schema != nil {
		for _, field := range schema.Fields {
			if field != nil {
				fields[field.Key] = field
			}
		}
		for _, policy := range schema.UploadPolicies {
			if policy != nil {
				policies[policy.ID] = policy
			}
		}
	}
	result := make([]purchaseconfigurationfile.VerifyInput, 0)
	for _, value := range input.Values {
		if value.FileTokens == nil {
			continue
		}
		field := fields[value.FieldKey]
		if field == nil || field.Type != capmodel.ConfigurationFieldFile || policies[field.UploadPolicy] == nil ||
			input.ConfigurationRevision == "" || len(input.ConfigurationRevision) > 256 {
			return nil, gerror.New("frozen customer file evidence is invalid")
		}
		result = append(result, purchaseconfigurationfile.VerifyInput{
			Target: &purchaseconfigurationfile.Target{
				ProductVersionNo: input.VersionNo, ConfigurationRevision: input.ConfigurationRevision,
				FieldKey: field.Key, Policy: policies[field.UploadPolicy],
			},
			FileTokens: append([]string(nil), value.FileTokens...),
		})
	}
	return result, nil
}

func validConfigurationDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func FrozenCustomerCandidateSnapshotDigest(candidates configurationcontract.ResolvedCandidates) (string, error) {
	encoded, err := json.Marshal(candidates)
	if err != nil {
		return "", gerror.Wrap(err, "encode frozen customer candidate snapshot")
	}
	return configurationDigest(encoded), nil
}
