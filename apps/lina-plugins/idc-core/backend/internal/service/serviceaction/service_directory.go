package serviceaction

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
)

type ActionDeclaration struct {
	Key                     string
	Title                   productfulfillmentcap.ProviderCapability
	Execution, Availability string
}
type DownloadDeclaration struct {
	Key   string
	Title productfulfillmentcap.ProviderCapability
}
type Entitlement struct {
	EntitlementNo, ProductNameZhCN, ProductNameEnUS, Status, CustomerName       string
	PrimaryDomain, ValidationMethod                                             string
	StartedAt                                                                   time.Time
	ExpiresAt                                                                   *time.Time
	IssuedAt, UpdatedAt                                                         time.Time
	Validation                                                                  json.RawMessage
	AvailableActions                                                            []string
	Actions                                                                     []ActionDeclaration
	Downloads                                                                   []DownloadDeclaration
	CustomerConfiguration, LatestProjection                                     json.RawMessage
	RowID                                                                       int64
	PluginID, ReleaseVersion, ContractVersion, ReleaseDigest, DeclarationDigest string
}

func (s *service) ListEntitlements(ctx context.Context, actor Actor, beforeID int64, limit int) ([]Entitlement, int64, error) {
	if validateActor(actor) != nil || limit < 1 || limit > 100 || beforeID < 0 {
		return nil, 0, bizerr.NewCode(CodeInvalid)
	}
	var rows []Entitlement
	err := s.database.GetScan(ctx, &rows, serviceDirectorySelect+` WHERE entitlement.tenant_id=0 AND (?='admin' OR entitlement.customer_no=?) AND (?=0 OR entitlement.id<?) ORDER BY entitlement.id DESC LIMIT ?`, actor.Audience, actor.CustomerNo, beforeID, beforeID, limit)
	if err != nil {
		return nil, 0, err
	}
	if err = s.attachDeclarations(ctx, rows, actor.Audience); err != nil {
		return nil, 0, err
	}
	next := int64(0)
	if len(rows) == limit {
		next = rows[len(rows)-1].RowID
	}
	return rows, next, nil
}

func (s *service) GetEntitlement(ctx context.Context, entitlementNo string, actor Actor) (*Entitlement, error) {
	if validateActor(actor) != nil || !safeReferencePattern.MatchString(entitlementNo) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	var row *Entitlement
	err := s.database.GetScan(ctx, &row, serviceDirectorySelect+` WHERE entitlement.tenant_id=0 AND entitlement.entitlement_no=? AND (?='admin' OR entitlement.customer_no=?) LIMIT 1`, entitlementNo, actor.Audience, actor.CustomerNo)
	if err != nil || row == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	rows := []Entitlement{*row}
	if err = s.attachDeclarations(ctx, rows, actor.Audience); err != nil {
		return nil, err
	}
	return &rows[0], nil
}

func (s *service) attachDeclarations(ctx context.Context, rows []Entitlement, audience string) error {
	inputs := make([]plugincap.BusinessExtensionProviderExactInput, 0, len(rows))
	inputKeys := make([]string, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for index := range rows {
		row := &rows[index]
		key := serviceDeclarationKey(row)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		inputKeys = append(inputKeys, key)
		inputs = append(inputs, plugincap.BusinessExtensionProviderExactInput{PluginID: row.PluginID, ReleaseVersion: row.ReleaseVersion, Point: capmodel.BusinessExtensionProductFulfillment, ContractVersion: row.ContractVersion, ReleaseDigest: row.ReleaseDigest, DeclarationDigest: row.DeclarationDigest})
	}
	if len(inputs) == 0 {
		return nil
	}
	exactItems, err := s.extensions.GetProvidersExact(ctx, inputs)
	if err != nil || len(exactItems) != len(inputs) {
		return bizerr.NewCode(CodeUnavailable)
	}
	declarations := make(map[string]*productfulfillmentcap.ProviderDeclaration, len(inputs))
	for index, exact := range exactItems {
		if exact == nil {
			return bizerr.NewCode(CodeUnavailable)
		}
		declaration, parseErr := productfulfillmentcap.ParseProviderDeclaration(exact.ProviderMetadata, exact.AuthorizedMethods)
		if parseErr != nil {
			return bizerr.NewCode(CodeUnavailable)
		}
		declarations[inputKeys[index]] = declaration
	}
	for index := range rows {
		row := &rows[index]
		declaration := declarations[serviceDeclarationKey(row)]
		if declaration == nil {
			return bizerr.NewCode(CodeUnavailable)
		}
		attachSafeServiceProjection(row, declaration)
		for _, action := range declaration.ServiceActions {
			if contains(action.Audiences, audience) && (action.Availability != productfulfillmentcap.ServiceActionAvailabilityBeforeFinal || !managedServiceFinal(row.Status)) {
				row.Actions = append(row.Actions, ActionDeclaration{Key: action.Key, Title: action.Title, Execution: action.Execution, Availability: action.Availability})
				row.AvailableActions = append(row.AvailableActions, action.Key)
			}
		}
		for _, download := range declaration.ServiceDownloads {
			if contains(download.Audiences, audience) && managedServiceIssued(row.Status) {
				row.Downloads = append(row.Downloads, DownloadDeclaration{Key: download.Key, Title: download.Title})
				row.AvailableActions = append(row.AvailableActions, "download")
			}
		}
	}
	return nil
}

func serviceDeclarationKey(row *Entitlement) string {
	if row == nil {
		return ""
	}
	return row.PluginID + "\x00" + row.ReleaseVersion + "\x00" + row.ContractVersion + "\x00" + row.ReleaseDigest + "\x00" + row.DeclarationDigest
}

const serviceDirectorySelect = `SELECT entitlement.id AS row_id,entitlement.entitlement_no,intent.product_name_zh_cn,intent.product_name_en_us,
entitlement.status_code AS status,subject.display_name AS customer_name,entitlement.started_at,entitlement.expires_at,
entitlement.updated_at,entitlement.plugin_id,entitlement.plugin_release_version AS release_version,entitlement.contract_version,
intent.release_digest,entitlement.declaration_digest,snapshot.typed_value_snapshot AS customer_configuration,
COALESCE(latest.safe_projection,'{}'::jsonb) AS latest_projection
FROM plugin_idc_core_service_entitlement entitlement
JOIN plugin_idc_core_product_fulfillment_intent intent ON intent.tenant_id=entitlement.tenant_id AND intent.intent_no=entitlement.source_intent_no
JOIN plugin_idc_core_commerce_structured_configuration_snapshot snapshot ON snapshot.tenant_id=intent.tenant_id AND snapshot.snapshot_no=intent.configuration_snapshot_no
JOIN plugin_idc_core_customer_subject subject ON subject.tenant_id=entitlement.tenant_id AND subject.customer_no=entitlement.customer_no
LEFT JOIN LATERAL (SELECT task.safe_projection FROM plugin_idc_core_managed_service_action_task task WHERE task.tenant_id=entitlement.tenant_id AND task.entitlement_no=entitlement.entitlement_no AND task.safe_projection<>'{}'::jsonb ORDER BY task.id DESC LIMIT 1) latest ON TRUE`

func attachSafeServiceProjection(row *Entitlement, declaration *productfulfillmentcap.ProviderDeclaration) {
	if row == nil || declaration == nil || declaration.ServiceProjection == nil {
		return
	}
	var customer, projection map[string]json.RawMessage
	if json.Unmarshal(row.CustomerConfiguration, &customer) != nil {
		customer = map[string]json.RawMessage{}
	}
	if json.Unmarshal(row.LatestProjection, &projection) != nil {
		projection = map[string]json.RawMessage{}
	}
	readString := func(values map[string]json.RawMessage, key string) string {
		var value string
		_ = json.Unmarshal(values[key], &value)
		return strings.TrimSpace(value)
	}
	mapping := declaration.ServiceProjection
	row.PrimaryDomain = readString(customer, mapping.PrimaryReferenceCustomerField)
	row.ValidationMethod = readString(customer, mapping.ValidationMethodCustomerField)
	if status := readString(projection, mapping.StatusField); status != "" {
		row.Status = status
	}
	parseTime := func(key string) time.Time {
		value := readString(projection, key)
		parsed, _ := time.Parse(time.RFC3339, value)
		return parsed
	}
	row.IssuedAt = parseTime(mapping.IssuedAtField)
	if projectedExpiry := parseTime(mapping.ExpiresAtField); !projectedExpiry.IsZero() {
		row.ExpiresAt = &projectedExpiry
	}
	if value := projection[mapping.ValidationField]; len(value) > 0 && json.Valid(value) {
		row.Validation = append(json.RawMessage(nil), value...)
	} else {
		row.Validation = json.RawMessage(`[]`)
	}
}

func managedServiceFinal(status string) bool {
	return managedServiceIssued(status) || strings.EqualFold(strings.TrimSpace(status), "cancelled") || strings.EqualFold(strings.TrimSpace(status), "canceled")
}

func managedServiceIssued(status string) bool {
	normalized := strings.ToLower(strings.TrimSpace(status))
	return normalized == "issued" || normalized == "active"
}
