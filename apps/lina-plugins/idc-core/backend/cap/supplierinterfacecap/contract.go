// Package supplierinterfacecap publishes the narrow runtime view of one
// host-validated supplier interface context. It never exposes secret handles.
package supplierinterfacecap

import (
	"context"
	"regexp"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
)

var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

const (
	OwnerPluginID           = "idc-core"
	ServiceName             = "supplier-interface-runtime"
	SourceContractPackage   = "lina-plugin-idc-core/backend/cap/supplierinterfacecap"
	DynamicContractPackage  = SourceContractPackage + "/bridge"
	ContractVersion         = "v1"
	MethodConfigurationGet  = "configuration.get"
	MethodFilesList         = "configuration.files.list"
	MethodFileRead          = "configuration.file.read"
	MethodOutboundHTTP      = "outbound.http.execute"
	MethodOutboundSession   = "outbound.session.execute"
	MaximumPayloadBytes     = 64 * 1024
	MaximumFileChunkBytes   = 1024 * 1024
	MaximumFilePayloadBytes = 2 * 1024 * 1024
	MaximumOutboundPayload  = 1024 * 1024
)

// ConfigurationValue is one non-secret value compiled by the owner's
// thirteen-type configuration compiler. Exactly one value arm is populated.
type ConfigurationValue struct {
	FieldKey            string   `json:"fieldKey"`
	TextValue           *string  `json:"textValue,omitempty"`
	IntegerValue        *int64   `json:"integerValue,omitempty"`
	DecimalValue        *string  `json:"decimalValue,omitempty"`
	BooleanValue        *bool    `json:"booleanValue,omitempty"`
	DateValue           *string  `json:"dateValue,omitempty"`
	DateTimeValue       *string  `json:"dateTimeValue,omitempty"`
	OptionTokens        []string `json:"optionTokens,omitempty"`
	TreeOptionTokens    []string `json:"treeOptionTokens,omitempty"`
	AddressOptionTokens []string `json:"addressOptionTokens,omitempty"`
}

// ConfigurationView is detached from the browser-facing management DTO and
// intentionally omits interface numbers, revisions, digests and secrets.
type ConfigurationView struct {
	Values []ConfigurationValue `json:"values"`
}

// ConfigurationFile is safe immutable order evidence. Sequence is scoped to
// one exact field; no upload token, object path, URL, or storage credential is
// exposed to a provider.
type ConfigurationFile struct {
	Sequence  int    `json:"sequence"`
	FileName  string `json:"fileName"`
	MediaType string `json:"mediaType"`
	SizeBytes int64  `json:"sizeBytes"`
	SHA256Hex string `json:"sha256Hex"`
}

type ConfigurationFileListRequest struct {
	FieldKey string `json:"fieldKey"`
}

type ConfigurationFileList struct {
	Files []ConfigurationFile `json:"files"`
}

type ConfigurationFileReadRequest struct {
	FieldKey     string `json:"fieldKey"`
	Sequence     int    `json:"sequence"`
	Offset       int64  `json:"offset"`
	MaximumBytes int    `json:"maximumBytes"`
}

type ConfigurationFileChunk struct {
	Sequence int    `json:"sequence"`
	Offset   int64  `json:"offset"`
	Data     []byte `json:"data"`
	EOF      bool   `json:"eof"`
}

// Service resolves only the context already attached to the current governed
// provider invocation.
type Service interface {
	GetConfiguration(context.Context, string) (*ConfigurationView, error)
	ListConfigurationFiles(context.Context, string, string) (*ConfigurationFileList, error)
	ReadConfigurationFile(context.Context, string, ConfigurationFileReadRequest) (*ConfigurationFileChunk, error)
	// ExecuteOutboundHTTP performs one exact-origin supplier request inside the
	// owner process. Dynamic providers supply only configuration field names;
	// endpoint and secret values never cross the WASM boundary.
	ExecuteOutboundHTTP(context.Context, string, OutboundHTTPRequest) (*OutboundHTTPResponse, error)
	// ExecuteOutboundSession performs one form login and up to eight exact-origin
	// requests without returning or retaining the supplier authentication token.
	ExecuteOutboundSession(context.Context, string, OutboundSessionRequest) (*OutboundSessionResponse, error)
	// AccessSecret is available only to source providers inside a host-wrapped
	// business-extension invocation. Dynamic clients fail closed.
	AccessSecret(context.Context, string, string, func([]byte) error) error
}

// SourceSecretAccessInvoker is explicitly injected into the source client;
// it is never published as an ordinary payload-returning capability method.
type SourceSecretAccessInvoker interface {
	AccessSecret(context.Context, string, string, func([]byte) error) error
}

func ValidateConfigurationView(view *ConfigurationView) error {
	if view == nil || len(view.Values) > 128 {
		return gerror.New("supplier interface configuration view is invalid")
	}
	seen := make(map[string]struct{}, len(view.Values))
	for _, value := range view.Values {
		if value.FieldKey == "" || value.FieldKey != strings.TrimSpace(value.FieldKey) || len(value.FieldKey) > 64 {
			return gerror.New("supplier interface configuration field is invalid")
		}
		if _, exists := seen[value.FieldKey]; exists {
			return gerror.New("supplier interface configuration field is duplicated")
		}
		seen[value.FieldKey] = struct{}{}
		arms := 0
		for _, present := range []bool{
			value.TextValue != nil, value.IntegerValue != nil, value.DecimalValue != nil,
			value.BooleanValue != nil, value.DateValue != nil, value.DateTimeValue != nil,
			value.OptionTokens != nil, value.TreeOptionTokens != nil, value.AddressOptionTokens != nil,
		} {
			if present {
				arms++
			}
		}
		if arms != 1 || !validConfigurationStrings(value) {
			return gerror.New("supplier interface configuration value is invalid")
		}
	}
	return nil
}

func ValidateConfigurationFileList(list *ConfigurationFileList) error {
	if list == nil || len(list.Files) > 100 {
		return gerror.New("supplier interface file list is invalid")
	}
	for index, file := range list.Files {
		if file.Sequence != index+1 || file.FileName == "" || file.FileName != strings.TrimSpace(file.FileName) ||
			len(file.FileName) > 255 || strings.ContainsAny(file.FileName, "\x00\r\n") ||
			file.MediaType == "" || len(file.MediaType) > 128 || file.SizeBytes < 1 || !sha256Pattern.MatchString(file.SHA256Hex) {
			return gerror.New("supplier interface file evidence is invalid")
		}
	}
	return nil
}

func ValidateConfigurationFileReadRequest(request ConfigurationFileReadRequest) error {
	if request.FieldKey == "" || request.FieldKey != strings.TrimSpace(request.FieldKey) || len(request.FieldKey) > 64 ||
		request.Sequence < 1 || request.Sequence > 100 || request.Offset < 0 ||
		request.MaximumBytes < 1 || request.MaximumBytes > MaximumFileChunkBytes {
		return gerror.New("supplier interface file read request is invalid")
	}
	return nil
}

func ValidateConfigurationFileChunk(request ConfigurationFileReadRequest, chunk *ConfigurationFileChunk) error {
	if chunk == nil || chunk.Sequence != request.Sequence || chunk.Offset != request.Offset || len(chunk.Data) > request.MaximumBytes || len(chunk.Data) > MaximumFileChunkBytes || (!chunk.EOF && len(chunk.Data) == 0) {
		return gerror.New("supplier interface file chunk is invalid")
	}
	return nil
}

func validConfigurationStrings(value ConfigurationValue) bool {
	for _, item := range []*string{value.TextValue, value.DecimalValue, value.DateValue, value.DateTimeValue} {
		if item != nil && len(*item) > 8192 {
			return false
		}
	}
	for _, items := range [][]string{value.OptionTokens, value.TreeOptionTokens, value.AddressOptionTokens} {
		if len(items) > 512 {
			return false
		}
		for _, item := range items {
			if item == "" || len(item) > 8192 {
				return false
			}
		}
	}
	return true
}
