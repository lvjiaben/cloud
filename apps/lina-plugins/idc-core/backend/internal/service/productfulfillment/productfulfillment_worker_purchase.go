// This file reconstructs upstream purchases from immutable paid-order and
// import evidence. Credentials remain in the resource owner; public selection
// tokens are mapped explicitly rather than inferred from opaque local keys.

package productfulfillment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"slices"
	"strconv"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
	"lina-plugin-idc-core/backend/internal/service/purchaseconfigurationfile"
)

type peerCandidateMapping struct {
	FieldKey, CandidateValue, OptionToken string
}

// The same-system protocol can reconcile a write by its original request ID,
// even when the order reference response was lost. Recovery is query-only;
// an absent result after crossing the boundary never authorizes another write.
func peerRecoveryDecision(claimed *claimedFulfillment, decision workerDecision, takeover, canceled bool) workerDecision {
	if claimed.PluginID != "idc-peer" || takeover || canceled {
		return decision
	}
	// A local runtime/credential failure during reconciliation must not erase
	// the fact that a submit already crossed the boundary. Otherwise a later
	// NotFound query would incorrectly grant permission to write again.
	if claimed.PreviousPhase == "post_submit_query" && decision.PhaseCode == "pre_submit_query" {
		decision.PhaseCode = "post_submit_query"
		return decision
	}
	if decision.TaskStatus != "result_unknown" {
		return decision
	}
	if claimed.QueryAttempts >= maximumQueryAttempts {
		return workerDecision{TaskStatus: "result_unknown", IntentStatus: "manual_pending", PhaseCode: "frozen_unknown"}
	}
	return workerDecision{TaskStatus: "retry_wait", IntentStatus: "processing", PhaseCode: "post_submit_query", ScheduleNext: true}
}

func (w *fulfillmentWorker) loadPeerContract(ctx context.Context, claimed *claimedFulfillment, method string) (*channelcommercecap.ImportSnapshot, *resourceinterfaceprojectioncap.CatalogReadEvidence, error) {
	var contract *entity.CatalogImportContract
	err := w.database.Model(do.CatalogImportContract{}).Ctx(ctx).Where(do.CatalogImportContract{TenantId: 0, VersionNo: claimed.VersionNo, InterfaceInstanceNo: claimed.InterfaceInstanceNo}).Scan(&contract)
	if err != nil || contract == nil {
		return nil, nil, gerror.New("peer import contract is unavailable")
	}
	var snapshot channelcommercecap.ImportSnapshot
	if json.Unmarshal([]byte(contract.Snapshot), &snapshot) != nil || snapshot.Validate() != nil {
		return nil, nil, gerror.New("peer import contract is invalid")
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return nil, nil, err
	}
	digest := sha256.Sum256(raw)
	if hex.EncodeToString(digest[:]) != contract.SnapshotDigest {
		return nil, nil, gerror.New("peer import contract digest mismatch")
	}
	var read resourceinterfaceprojectioncap.CatalogReadEvidence
	if json.Unmarshal([]byte(contract.ReadEvidence), &read) != nil || read.Validate() != nil || read.InterfaceInstanceNo != claimed.InterfaceInstanceNo || read.InterfaceConfigurationRevision != claimed.ResourceInterfaceConfigurationRevision || read.PluginID != claimed.PluginID || read.PluginReleaseVersion != claimed.ReleaseVersion || read.ContractVersion != claimed.ContractVersion || read.ReleaseDigest != claimed.ReleaseDigest || read.DeclarationDigest != claimed.DeclarationDigest || !slices.Contains(read.AuthorizedMethods, method) {
		return nil, nil, gerror.New("peer import runtime evidence mismatch")
	}
	return &snapshot, &read, nil
}

func (w *fulfillmentWorker) peerPurchase(ctx context.Context, claimed *claimedFulfillment, snapshot *channelcommercecap.ImportSnapshot, read *resourceinterfaceprojectioncap.CatalogReadEvidence, method string) (*channelcommercecap.Purchase, error) {
	var term *entity.CatalogImportTerm
	err := w.database.Model(do.CatalogImportTerm{}).Ctx(ctx).Where(do.CatalogImportTerm{TenantId: 0, VersionNo: claimed.VersionNo, BillingRuleNo: claimed.BillingRuleNo, CurrencyCode: claimed.CurrencyCode}).Scan(&term)
	if err != nil || term == nil || term.UpstreamTermReference == "" {
		return nil, gerror.New("peer billing term mapping is unavailable")
	}
	values, err := configurationcontract.DecodeCompiledValues(snapshot.Schema, claimed.CustomerConfiguration)
	if err != nil {
		return nil, err
	}
	// One bounded set query supplements static mappings with selections that
	// were authenticated and frozen during dynamic configuration evaluation.
	var candidates []peerCandidateMapping
	keys, selected := peerSelectionKeys(values)
	if len(selected) > 0 {
		if err = w.database.GetScan(ctx, &candidates, `SELECT field_key,candidate_value,option_token FROM plugin_idc_core_catalog_import_candidate WHERE tenant_id=0 AND version_no=? AND field_key IN (?) AND candidate_value IN (?)`, claimed.VersionNo, keys, selected); err != nil {
			return nil, err
		}
	}
	withoutFiles := append([]configurationcontract.ConfigurationValue(nil), values...)
	for i := range withoutFiles {
		withoutFiles[i].FileTokens = nil
	}
	purchaseValues, err := mapPeerPurchaseValues(snapshot, withoutFiles, candidates)
	if err != nil {
		return nil, err
	}
	if err = w.transferPeerFiles(ctx, claimed, snapshot, read, method, values, purchaseValues); err != nil {
		return nil, err
	}
	return &channelcommercecap.Purchase{PublicSlug: snapshot.Product.PublicSlug, ConfigurationRevision: snapshot.ConfigurationRevision, BillingTermToken: term.UpstreamTermReference, CurrencyCode: claimed.CurrencyCode, PurchaseCycles: claimed.PurchaseCycles, Quantity: claimed.Quantity, Values: purchaseValues}, nil
}

func peerSelectionKeys(values []configurationcontract.ConfigurationValue) ([]string, []string) {
	keys := make([]string, 0, len(values))
	selected := make([]string, 0)
	for _, value := range values {
		keys = append(keys, value.FieldKey)
		selected = append(selected, value.OptionTokens...)
		selected = append(selected, value.TreeOptionTokens...)
		selected = append(selected, value.AddressOptionTokens...)
	}
	return keys, selected
}

func (w *fulfillmentWorker) transferPeerFiles(ctx context.Context, claimed *claimedFulfillment, snapshot *channelcommercecap.ImportSnapshot, read *resourceinterfaceprojectioncap.CatalogReadEvidence, method string, values []configurationcontract.ConfigurationValue, result []channelcommercecap.PurchaseValue) error {
	var saved []entity.PeerPurchaseFile
	if hasPeerFiles(values) {
		if err := w.database.Model(do.PeerPurchaseFile{}).Ctx(ctx).Where(do.PeerPurchaseFile{TenantId: 0, IntentNo: claimed.IntentNo}).Scan(&saved); err != nil {
			return err
		}
	}
	cache := make(map[string]entity.PeerPurchaseFile, len(saved))
	for _, file := range saved {
		cache[file.FieldKey+"\x00"+strconv.Itoa(file.FileSequence)] = file
	}
	for index, value := range values {
		if len(value.FileTokens) == 0 {
			continue
		}
		transfer, ok := w.peerOutbound.(resourceinterfaceprojectioncap.PurchaseFileTransferSource)
		if !ok || w.consumedFiles == nil {
			return gerror.New("peer file transfer capability is unavailable")
		}
		scope := purchaseconfigurationfile.ConsumedFileScope{IntentNo: claimed.IntentNo, OrderNo: claimed.OrderNo, OrderItemNo: claimed.OrderItemNo, ConfigurationSnapshotNo: claimed.ConfigurationSnapshotNo, PluginID: claimed.PluginID, ReleaseVersion: claimed.ReleaseVersion, ContractVersion: claimed.ContractVersion, ReleaseDigest: claimed.ReleaseDigest, DeclarationDigest: claimed.DeclarationDigest, Method: method, InterfaceInstanceNo: claimed.InterfaceInstanceNo, InterfaceConfigurationRevision: claimed.InterfaceConfigurationRevision}
		evidence := resourceinterfaceprojectioncap.ExistingBusinessOutboundRequest{InterfaceInstanceNo: claimed.InterfaceInstanceNo, InterfaceConfigurationRevision: read.InterfaceConfigurationRevision, ResourceChannelRef: read.ResourceChannelRef, PluginID: claimed.PluginID, PluginReleaseVersion: claimed.ReleaseVersion, ContractVersion: claimed.ContractVersion, ReleaseDigest: claimed.ReleaseDigest, DeclarationDigest: claimed.DeclarationDigest, Method: method}
		err := w.consumedFiles.VisitConsumedField(ctx, scope, value.FieldKey, func(file purchaseconfigurationfile.ConsumedFile, body io.Reader) error {
			if prior, found := cache[value.FieldKey+"\x00"+strconv.Itoa(file.Sequence)]; found {
				if prior.Sha256Hex != file.SHA256Hex || prior.UpstreamFileToken == "" {
					return gerror.New("peer file transfer evidence mismatch")
				}
				result[index].FileTokens = append(result[index].FileTokens, prior.UpstreamFileToken)
				return nil
			}
			token, err := transfer.TransferPurchaseFile(ctx, evidence, resourceinterfaceprojectioncap.PurchaseFileTransfer{PublicSlug: snapshot.Product.PublicSlug, ConfigurationRevision: snapshot.ConfigurationRevision, FieldToken: result[index].FieldToken, FileName: file.FileName, MediaType: file.MediaType, SHA256Hex: file.SHA256Hex, SizeBytes: file.SizeBytes, Body: body})
			if err != nil {
				return err
			}
			if token == "" {
				return gerror.New("peer upload returned an empty token")
			}
			if _, err = w.database.Model(do.PeerPurchaseFile{}).Ctx(ctx).Data(do.PeerPurchaseFile{TenantId: 0, IntentNo: claimed.IntentNo, FieldKey: value.FieldKey, FileSequence: file.Sequence, Sha256Hex: file.SHA256Hex, UpstreamFileToken: token}).Insert(); err != nil {
				return err
			}
			result[index].FileTokens = append(result[index].FileTokens, token)
			return nil
		})
		if err != nil {
			return err
		}
		if len(result[index].FileTokens) != len(value.FileTokens) {
			return gerror.New("peer file snapshot count mismatch")
		}
	}
	return nil
}

func hasPeerFiles(values []configurationcontract.ConfigurationValue) bool {
	for _, value := range values {
		if len(value.FileTokens) > 0 {
			return true
		}
	}
	return false
}

func mapPeerPurchaseValues(snapshot *channelcommercecap.ImportSnapshot, values []configurationcontract.ConfigurationValue, candidates []peerCandidateMapping) ([]channelcommercecap.PurchaseValue, error) {
	fields := make(map[string]string)
	hidden := make(map[string]bool)
	types := make(map[string]string)
	options := make(map[string]string)
	for _, mapping := range snapshot.Mappings {
		fields[mapping.FieldKey] = mapping.FieldToken
		hidden[mapping.FieldKey] = mapping.CustomerVisible != nil && !*mapping.CustomerVisible
		for _, option := range mapping.Options {
			options[mapping.FieldKey+"\x00"+option.Value] = option.OptionToken
		}
	}
	if snapshot.Schema != nil {
		for _, field := range snapshot.Schema.Fields {
			types[field.Key] = string(field.Type)
		}
	}
	for _, candidate := range candidates {
		key := candidate.FieldKey + "\x00" + candidate.CandidateValue
		if fields[candidate.FieldKey] == "" || candidate.OptionToken == "" || (options[key] != "" && options[key] != candidate.OptionToken) {
			return nil, gerror.New("peer candidate mapping conflicts with import")
		}
		options[key] = candidate.OptionToken
	}
	result := make([]channelcommercecap.PurchaseValue, 0, len(values))
	for _, value := range values {
		if hidden[value.FieldKey] {
			continue
		} // The source owns and reapplies this immutable default.
		mapped, err := mapPeerPurchaseValue(value, fields[value.FieldKey], types[value.FieldKey], options)
		if err != nil {
			return nil, err
		}
		result = append(result, mapped)
	}
	return result, nil
}

func mapPeerPurchaseValue(value configurationcontract.ConfigurationValue, fieldToken, fieldType string, options map[string]string) (channelcommercecap.PurchaseValue, error) {
	if fieldToken == "" || fieldType == "" {
		return channelcommercecap.PurchaseValue{}, gerror.New("peer field mapping is unavailable")
	}
	result := channelcommercecap.PurchaseValue{FieldToken: fieldToken, Type: fieldType, TextValue: value.TextValue, IntegerValue: value.IntegerTextValue, DecimalValue: value.DecimalValue, BooleanValue: value.BooleanValue, DateValue: value.DateValue, DateTimeValue: value.DateTimeValue}
	if value.IntegerValue != nil {
		text := strconv.FormatInt(*value.IntegerValue, 10)
		result.IntegerValue = &text
	}
	for _, arm := range []struct {
		input  []string
		output *[]string
	}{{value.OptionTokens, &result.OptionTokens}, {value.TreeOptionTokens, &result.TreeOptionTokens}, {value.AddressOptionTokens, &result.AddressOptionTokens}} {
		for _, selected := range arm.input {
			token := options[value.FieldKey+"\x00"+selected]
			if token == "" {
				return channelcommercecap.PurchaseValue{}, gerror.New("peer selected option mapping is unavailable")
			}
			*arm.output = append(*arm.output, token)
		}
	}
	if len(value.FileTokens) > 0 {
		return channelcommercecap.PurchaseValue{}, gerror.New("peer configuration files require governed transfer")
	}
	return result, nil
}

func (w *fulfillmentWorker) invokePeerProvider(ctx context.Context, claimed *claimedFulfillment, request *productfulfillmentcap.Request, method string) (*productfulfillmentcap.Result, []byte, bool, error) {
	if w.peerOutbound == nil {
		return nil, nil, false, gerror.New("peer outbound source is unavailable")
	}
	snapshot, read, err := w.loadPeerContract(ctx, claimed, method)
	if err != nil {
		return nil, nil, false, err
	}
	var purchase *channelcommercecap.Purchase
	if claimed.Action == productfulfillmentcap.ActionSubmit {
		purchase, err = w.peerPurchase(ctx, claimed, snapshot, read, method)
		if err != nil {
			return nil, nil, false, err
		}
	}
	payload, err := json.Marshal(struct {
		*productfulfillmentcap.Request
		PeerPurchase *channelcommercecap.Purchase `json:"peerPurchase,omitempty"`
	}{request, purchase})
	if err != nil {
		return nil, nil, false, err
	}
	var hostResult *capmodel.BusinessExtensionMethodResult
	crossed := false
	err = w.peerOutbound.WithOutbound(ctx, resourceinterfaceprojectioncap.ExistingBusinessOutboundRequest{InterfaceInstanceNo: claimed.InterfaceInstanceNo, InterfaceConfigurationRevision: read.InterfaceConfigurationRevision, ResourceChannelRef: read.ResourceChannelRef, PluginID: claimed.PluginID, PluginReleaseVersion: claimed.ReleaseVersion, ContractVersion: claimed.ContractVersion, ReleaseDigest: claimed.ReleaseDigest, DeclarationDigest: claimed.DeclarationDigest, Method: method}, func(handler pluginhost.ConfidentialOutboundHandler) error {
		return w.extensions.WithConfidentialOutboundContext(ctx, pluginhost.ConfidentialOutboundContextInput{ProviderPluginID: claimed.PluginID, Point: capmodel.BusinessExtensionProductFulfillment, ContractVersion: claimed.ContractVersion, ProviderMethod: method, OutboundMethod: "resource-provider." + method, ReleaseVersion: claimed.ReleaseVersion, ReleaseDigest: claimed.ReleaseDigest, DeclarationDigest: claimed.DeclarationDigest, InterfaceInstanceID: claimed.InterfaceInstanceNo, Purpose: capmodel.BusinessExtensionInvocationPurposeExistingBusiness, ExpiresAfter: 30 * time.Second, AllowedResultFields: []string{"StatusCode", "Data", "Authenticated"}, Handler: handler}, func(callCtx context.Context, runtime *capmodel.BusinessExtensionRuntimeContext) error {
			crossed = true
			var invokeErr error
			hostResult, invokeErr = w.extensions.InvokeProviderMethod(callCtx, pluginhost.BusinessExtensionMethodInvokeInput{PluginID: claimed.PluginID, Point: capmodel.BusinessExtensionProductFulfillment, ContractVersion: claimed.ContractVersion, Method: method, ReleaseVersion: claimed.ReleaseVersion, ReleaseDigest: claimed.ReleaseDigest, DeclarationDigest: claimed.DeclarationDigest, Purpose: capmodel.BusinessExtensionInvocationPurposeExistingBusiness, RuntimeContext: runtime, Payload: payload})
			return invokeErr
		})
	})
	if err != nil {
		return nil, nil, crossed, err
	}
	if hostResult == nil || hostResult.Version != capmodel.BusinessExtensionInvocationProtocolV1 {
		return nil, nil, crossed, gerror.New("peer returned an invalid fulfillment envelope")
	}
	raw := append([]byte(nil), hostResult.Payload...)
	var result *productfulfillmentcap.Result
	if json.Unmarshal(raw, &result) != nil || productfulfillmentcap.ValidateResult(claimed.Action, result) != nil || (claimed.Action == productfulfillmentcap.ActionQuery && claimed.PreviousExternalReference != "" && result.ExternalReference != "" && result.ExternalReference != claimed.PreviousExternalReference) {
		return nil, raw, crossed, gerror.New("peer returned an invalid fulfillment result")
	}
	return result, raw, crossed, nil
}
