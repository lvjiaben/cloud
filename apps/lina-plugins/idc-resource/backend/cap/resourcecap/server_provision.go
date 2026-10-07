// This file defines the secret-free server provisioning plan shared with Provider connectors.

package resourcecap

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/errors/gerror"
)

// ServerProvisionComponentType identifies one server-specific configuration
// fact. These values belong to the server resource contract and are not part
// of the generic product, cart, order, or billing model.
type ServerProvisionComponentType string

// Server provisioning component kinds.
const (
	ServerProvisionComponentRegion          ServerProvisionComponentType = "region"
	ServerProvisionComponentLine            ServerProvisionComponentType = "line"
	ServerProvisionComponentDatacenter      ServerProvisionComponentType = "datacenter"
	ServerProvisionComponentCPU             ServerProvisionComponentType = "cpu"
	ServerProvisionComponentMemory          ServerProvisionComponentType = "memory"
	ServerProvisionComponentSystemDisk      ServerProvisionComponentType = "system_disk"
	ServerProvisionComponentDataDisk        ServerProvisionComponentType = "data_disk"
	ServerProvisionComponentIP              ServerProvisionComponentType = "ip"
	ServerProvisionComponentBackup          ServerProvisionComponentType = "backup"
	ServerProvisionComponentImage           ServerProvisionComponentType = "image"
	ServerProvisionComponentOperatingSystem ServerProvisionComponentType = "operating_system"
	ServerProvisionComponentTraffic         ServerProvisionComponentType = "traffic"
	ServerProvisionComponentBandwidth       ServerProvisionComponentType = "bandwidth"
	ServerProvisionComponentOther           ServerProvisionComponentType = "other"
)

// ServerProvisionComponent is one typed server-plugin configuration fact.
// It intentionally contains no Provider field names or confidential values.
type ServerProvisionComponent struct {
	Type              ServerProvisionComponentType
	Code              string
	ModelCode         string
	Cores             int64
	Megabytes         int64
	Gigabytes         int64
	Media             string
	MegabitsPerSecond int64
	Unlimited         bool
	Count             int64
	Family            string
	Quantity          int64
	SelectionQuantity int64
}

// ServerProvisionPlan is the only input supplied to a connector parameter
// builder. Selector is connector-owned but must remain bounded and secret-free.
// Credentials are supplied later through OperationRequest.SecretBindings and
// therefore never enter this plan or a managed guest's request memory.
type ServerProvisionPlan struct {
	ExternalPrincipalRef  string                     `json:"externalPrincipalRef"`
	Hostname              string                     `json:"hostname"`
	Selector              json.RawMessage            `json:"selector"`
	ProductConfiguration  json.RawMessage            `json:"productConfiguration"`
	CustomerConfiguration json.RawMessage            `json:"customerConfiguration"`
	Components            []ServerProvisionComponent `json:"components"`
}

// ProvisionParameterBuilder maps one secret-free server plan to the
// connector's non-secret operation parameters. A source connector implements
// this callback directly; a managed connector will expose the same operation
// through its versioned guest method before the owner performs secret
// injection and outbound I/O.
type ProvisionParameterBuilder func(
	ctx context.Context,
	plan ServerProvisionPlan,
) (json.RawMessage, error)

// BuildProvisionParameters validates both sides of the connector mapping and
// returns only a detached, secret-free JSON object. It never supplies secret
// handles, accessors, HTTP clients, or raw credentials to the builder.
func BuildProvisionParameters(
	ctx context.Context,
	builder ProvisionParameterBuilder,
	plan ServerProvisionPlan,
) (json.RawMessage, error) {
	if ctx == nil || ctx.Err() != nil || builder == nil {
		return nil, gerror.New("resource Provider provisioning mapper is unavailable")
	}
	if err := plan.Validate(); err != nil {
		return nil, err
	}
	parameters, err := builder(ctx, plan)
	if err != nil {
		return nil, gerror.New("resource Provider provisioning mapper failed")
	}
	if err = validateJSONObject("provision operation parameters", parameters, false); err != nil {
		return nil, err
	}
	return append(json.RawMessage(nil), parameters...), nil
}

// Validate verifies the connector receives one bounded, secret-free server
// plan. It does not interpret connector-specific selector contents.
func (p ServerProvisionPlan) Validate() error {
	if err := validateRequiredReference("external principal reference", p.ExternalPrincipalRef); err != nil {
		return err
	}
	if err := validateOptionalText("server hostname", p.Hostname, maximumReferenceBytes); err != nil {
		return err
	}
	if err := validateJSONObject("server provision selector", p.Selector, false); err != nil {
		return err
	}
	if err := validateJSONObject("server product configuration", p.ProductConfiguration, false); err != nil {
		return err
	}
	if err := validateJSONObject("server customer configuration", p.CustomerConfiguration, false); err != nil {
		return err
	}
	if len(p.Components) == 0 || len(p.Components) > 64 {
		return gerror.New("resource Provider server provision components are invalid")
	}
	for _, component := range p.Components {
		if err := component.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Validate verifies one component uses a known server-specific kind and
// contains no credential material in its textual facts.
func (c ServerProvisionComponent) Validate() error {
	switch c.Type {
	case ServerProvisionComponentRegion,
		ServerProvisionComponentLine,
		ServerProvisionComponentDatacenter,
		ServerProvisionComponentCPU,
		ServerProvisionComponentMemory,
		ServerProvisionComponentSystemDisk,
		ServerProvisionComponentDataDisk,
		ServerProvisionComponentIP,
		ServerProvisionComponentBackup,
		ServerProvisionComponentImage,
		ServerProvisionComponentOperatingSystem,
		ServerProvisionComponentTraffic,
		ServerProvisionComponentBandwidth,
		ServerProvisionComponentOther:
	default:
		return gerror.New("resource Provider server provision component type is invalid")
	}
	for name, value := range map[string]string{
		"component code": c.Code, "component model code": c.ModelCode,
		"component media": c.Media, "component family": c.Family,
	} {
		if err := validateSafeDiagnostic(name, value, maximumNameBytes, false); err != nil {
			return err
		}
	}
	if c.Cores < 0 || c.Megabytes < 0 || c.Gigabytes < 0 ||
		c.MegabitsPerSecond < 0 || c.Count < 0 || c.Quantity < 0 ||
		c.SelectionQuantity < 1 {
		return gerror.New("resource Provider server provision component quantity is invalid")
	}
	return nil
}
