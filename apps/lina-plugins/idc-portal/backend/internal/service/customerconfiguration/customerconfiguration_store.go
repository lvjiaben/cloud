// This file validates provider content and atomically stores template settings.

package customerconfiguration

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-portal/backend/cap/customertemplatecap"
	"lina-plugin-idc-portal/backend/cap/templatecap"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

// providers lists all supported installed and enabled templates in bounded pages.
func (s *serviceImpl) providers(ctx context.Context) ([]*capmodel.BusinessExtensionProviderEvidence, error) {
	result := make([]*capmodel.BusinessExtensionProviderEvidence, 0)
	for page := 1; page <= 5; page++ {
		p, err := s.extensions.ListProviders(ctx, plugincap.BusinessExtensionProviderListInput{Point: customertemplatecap.Point, ContractVersion: customertemplatecap.Version, Page: capmodel.PageRequest{PageNum: page, PageSize: 100, Limit: 100}})
		if err != nil {
			return nil, err
		}
		if p == nil {
			return nil, gerror.New("customer template plugin registry is unavailable")
		}
		result = append(result, p.Items...)
		if len(p.Items) < 100 || len(result) >= p.Total {
			return result, nil
		}
	}
	return nil, gerror.New("too many customer templates")
}

// Get returns the saved settings and authoritative available template choices.
func (s *serviceImpl) Get(ctx context.Context) (result View, err error) {
	defer func() { err = publicError(err) }()
	items, err := s.providers(ctx)
	if err != nil {
		return View{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	settings, err := s.read()
	if err != nil {
		return View{}, err
	}
	candidates := make([]Candidate, 0, len(items))
	for _, item := range items {
		pages, pageErr := customertemplatecap.Pages(item.ProviderMetadata)
		if pageErr != nil {
			return View{}, pageErr
		}
		entry, entryErr := templatecap.Resolve(item)
		if entryErr != nil {
			return View{}, entryErr
		}
		candidates = append(candidates, Candidate{Entry: entry, Pages: pages, PluginID: item.PluginID, Name: item.DisplayName, Schema: item.InterfaceConfiguration, Renderer: renderer(item)})
	}
	return View{Settings: settings, Plugins: candidates}, nil
}

// Save validates only declared fields and commits under the host provider lifecycle lock.
func (s *serviceImpl) Save(ctx context.Context, input Settings) (err error) {
	defer func() { err = publicError(err) }()
	items, err := s.providers(ctx)
	if err != nil {
		return err
	}
	var chosen *capmodel.BusinessExtensionProviderEvidence
	for _, item := range items {
		if item.PluginID == input.SelectedPlugin {
			chosen = item
			break
		}
	}
	if chosen == nil {
		return bizerr.NewCode(codeUnavailable)
	}

	if err = validate(input, chosen.InterfaceConfiguration); err != nil {
		return err
	}
	return s.extensions.WithNewBusinessAdmission(ctx, pluginhost.BusinessExtensionAdmissionInput{PluginID: chosen.PluginID, Point: customertemplatecap.Point, ContractVersion: customertemplatecap.Version, ReleaseVersion: chosen.ReleaseVersion, ReleaseDigest: chosen.ReleaseDigest, DeclarationDigest: chosen.DeclarationDigest}, func(_ context.Context, _ *capmodel.BusinessExtensionProviderEvidence) error {
		s.mu.Lock()
		defer s.mu.Unlock()
		previous, readErr := s.read()
		if readErr != nil {
			return readErr
		}
		if previous.Configurations == nil {
			previous.Configurations = map[string]map[string]any{}
		}
		if previous.SelectedPlugin != "" {
			previous.Configurations[previous.SelectedPlugin] = previous.Config
		}
		previous.Configurations[input.SelectedPlugin] = input.Config
		input.Configurations = previous.Configurations
		data, marshalErr := json.Marshal(struct {
			Settings
			Configurations map[string]map[string]any `json:"configurations"`
		}{input, input.Configurations})
		if marshalErr != nil {
			return marshalErr
		}
		file, createErr := os.CreateTemp(filepath.Dir(s.path), ".website-*")
		if createErr != nil {
			return createErr
		}
		name := file.Name()
		defer func() {
			if removeErr := os.Remove(name); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) { /* Best-effort cleanup after the atomic commit. */
			}
		}()
		if _, writeErr := file.Write(data); writeErr != nil {
			if closeErr := file.Close(); closeErr != nil {
				return closeErr
			}
			return writeErr
		}
		if closeErr := file.Close(); closeErr != nil {
			return closeErr
		}
		return os.Rename(name, s.path)
	})
}

// read loads a snapshot without generating implicit template installation or selection.
func (s *serviceImpl) read() (Settings, error) {
	var stored struct {
		Settings
		Configurations map[string]map[string]any `json:"configurations"`
	}
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Settings{Config: map[string]any{}}, nil
	}
	if err != nil {
		return Settings{}, err
	}
	if err = json.Unmarshal(data, &stored); err != nil {
		return Settings{}, err
	}
	stored.Settings.Configurations = stored.Configurations
	return stored.Settings, nil
}

// validate restricts public content size and accepts only declared field types.
func validate(input Settings, schema *capmodel.ConfigurationSchemaDeclaration) error {
	if strings.TrimSpace(input.Common.SiteName) == "" {
		return bizerr.NewCode(codeNameRequired)
	}
	data, err := json.Marshal(input)
	if err != nil {
		return err
	}
	if len(data) > 65536 {
		return bizerr.NewCode(codeTooLong)
	}
	if !safeImage(input.Common.LogoURL) {
		return bizerr.NewCode(codeImageInvalid)
	}
	if schema == nil {
		return bizerr.NewCode(codeSchemaInvalid)
	}
	allowed := map[string]bool{}
	for _, field := range schema.Fields {
		allowed[field.Key] = true
		value, exists := input.Config[field.Key]
		if !exists || value == nil || emptyText(value) {
			if field.Required {
				return bizerr.NewCode(codeRequired)
			}
			continue
		}
		switch field.Type {
		case capmodel.ConfigurationFieldSingleLine, capmodel.ConfigurationFieldMultiLine:
			text, ok := value.(string)
			if !ok || utf8.RuneCountInString(text) > maxLength(field.MaximumLength) {
				return bizerr.NewCode(codeTextInvalid)
			}
		case capmodel.ConfigurationFieldBoolean:
			if _, ok := value.(bool); !ok {
				return bizerr.NewCode(codeOptionInvalid)
			}
		case capmodel.ConfigurationFieldInteger:
			number, ok := value.(float64)
			if !ok || number != float64(int64(number)) {
				return bizerr.NewCode(codeIntegerInvalid)
			}
		}
	}
	if raw, ok := input.Config["heroimage"].(string); ok && !safeImage(raw) {
		return bizerr.NewCode(codeImageInvalid)
	}
	if raw, ok := input.Config["primarycolor"].(string); ok && raw != "" && !regexp.MustCompile(`^#[0-9a-fA-F]{6}$`).MatchString(raw) {
		return bizerr.NewCode(codeColorInvalid)
	}
	for key := range input.Config {
		if !allowed[key] {
			return bizerr.NewCode(codeFieldsChanged)
		}
	}
	return nil
}

// maxLength provides a bounded default for public text.
func maxLength(value int) int {
	if value <= 0 || value > 8192 {
		return 8192
	}
	return value
}

// Public returns content only when the selected provider is still installed and enabled.
func (s *serviceImpl) Public(ctx context.Context) (Settings, bool, error) {
	view, err := s.Get(ctx)
	if err != nil {
		return Settings{}, false, err
	}
	for _, plugin := range view.Plugins {
		if plugin.PluginID == view.SelectedPlugin {
			view.Settings.Entry = plugin.Entry
			view.Settings.Renderer = plugin.Renderer
			view.Settings.Pages = plugin.Pages
			return view.Settings, true, nil
		}
	}
	return Settings{}, false, nil
}

// safeImage permits only local absolute paths and public HTTP images.
func safeImage(value string) bool {
	if value == "" {
		return true
	}
	if strings.HasPrefix(value, "/") {
		return !strings.HasPrefix(value, "//") && !strings.ContainsAny(value, "\\\r\n")
	}
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil
}

// renderer reads the owner-validated renderer from provider evidence.
func renderer(item *capmodel.BusinessExtensionProviderEvidence) string {
	if item.ProviderMetadata == nil {
		return ""
	}
	value, _ := item.ProviderMetadata.Values["renderer"].(string)
	return value
}

// emptyText checks optional strings without comparing uncomparable values.
func emptyText(value any) bool { text, ok := value.(string); return ok && text == "" }

// publicError preserves validation identities and hides technical storage failures.
func publicError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := bizerr.As(err); ok {
		return err
	}
	return bizerr.WrapCode(err, codeFailure)
}
