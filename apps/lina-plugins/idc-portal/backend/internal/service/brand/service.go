package brand

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/gogf/gf/v2/errors/gerror"
)

// LocalizedText is the bilingual text used by the public website contract.
type LocalizedText struct {
	ENUS string `json:"enUS"`
	ZHCN string `json:"zhCN"`
}

// Config is the merchant-editable, code-free website brand configuration.
type Config struct {
	Revision       string          `json:"revision"`
	SiteName       LocalizedText   `json:"siteName"`
	LogoURL        string          `json:"logoUrl,omitempty"`
	PrimaryColor   string          `json:"primaryColor"`
	ContactEmail   string          `json:"contactEmail,omitempty"`
	ContactPhone   string          `json:"contactPhone,omitempty"`
	Navigation     []NavItem       `json:"navigation"`
	HomepageBlocks []HomepageBlock `json:"homepageBlocks"`
	Footer         LocalizedText   `json:"footer"`
	TermsURL       string          `json:"termsUrl,omitempty"`
	SEOTitle       LocalizedText   `json:"seoTitle"`
	SEODescription LocalizedText   `json:"seoDescription"`
}

// HomepageBlock is a small, code-free content block rendered on the public home page.
// Links are optional and are restricted to public website routes by Validate.
type HomepageBlock struct {
	Title       LocalizedText `json:"title"`
	Description LocalizedText `json:"description"`
	Href        string        `json:"href,omitempty"`
}

// NavItem is one safe public navigation link.
type NavItem struct {
	Label LocalizedText `json:"label"`
	Href  string        `json:"href"`
}

type published struct {
	Config
	Previous *Config `json:"previous,omitempty"`
}

// Service stores immutable brand revisions and an atomically published pointer.
type Service struct {
	root string
	mu   sync.RWMutex
}

// New uses LINA_SITE_STORAGE, matching the site-packager runtime. The default is local and explicit.
func New(root string) (*Service, error) {
	if strings.TrimSpace(root) == "" {
		root = os.Getenv("LINA_SITE_STORAGE")
	}
	if strings.TrimSpace(root) == "" {
		root = ".site-runtime"
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(root, "brand", "revisions"), 0o750); err != nil {
		return nil, err
	}
	return &Service{root: root}, nil
}

// Save validates and stages a revision without changing the live website.
func (s *Service) Save(config Config) (Config, error) {
	if err := Validate(config); err != nil {
		return Config{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.root, "brand", "revisions", config.Revision+".json")
	if _, err := os.Stat(path); err == nil {
		return config, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}
	if err := writeJSON(path, config); err != nil {
		return Config{}, err
	}
	return config, nil
}

// Preview returns a previously staged immutable revision.
func (s *Service) Preview(revision string) (Config, error) {
	if !safeRevision(revision) {
		return Config{}, gerror.New("brand revision is invalid")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var config Config
	if err := readJSON(filepath.Join(s.root, "brand", "revisions", revision+".json"), &config); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, gerror.New("brand revision is not staged")
		}
		return Config{}, err
	}
	return config, nil
}

// Current returns the live configuration, if one has been published.
func (s *Service) Current() (Config, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var current published
	if err := readJSON(filepath.Join(s.root, "brand", "current.json"), &current); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, false, nil
		}
		return Config{}, false, err
	}
	return current.Config, true, nil
}

// Publish atomically makes a staged revision live and preserves the prior one.
func (s *Service) Publish(revision string) error {
	config, err := s.Preview(revision)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var current published
	if readErr := readJSON(filepath.Join(s.root, "brand", "current.json"), &current); readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return readErr
	}
	var previous *Config
	if current.Revision != "" {
		previous = &current.Config
	}
	return writeJSON(filepath.Join(s.root, "brand", "current.json"), published{Config: config, Previous: previous})
}

// Rollback restores the immediately previous live revision.
func (s *Service) Rollback() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var current published
	if err := readJSON(filepath.Join(s.root, "brand", "current.json"), &current); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return gerror.New("no published brand")
		}
		return err
	}
	if current.Previous == nil {
		return gerror.New("no previous brand revision")
	}
	return writeJSON(filepath.Join(s.root, "brand", "current.json"), *current.Previous)
}

var revisionPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// Validate enforces the same code-free and route-safe contract as site-contracts.
func Validate(config Config) error {
	if !safeRevision(config.Revision) || strings.TrimSpace(config.SiteName.ENUS) == "" || strings.TrimSpace(config.SiteName.ZHCN) == "" {
		return gerror.New("brand revision and bilingual site name are required")
	}
	if matched, _ := regexp.MatchString(`^#[0-9a-fA-F]{6}$`, config.PrimaryColor); !matched {
		return gerror.New("brand primary color must be a six-digit hex color")
	}
	if config.Navigation == nil || config.HomepageBlocks == nil || len(config.Navigation) > 20 || len(config.HomepageBlocks) > 12 || strings.TrimSpace(config.Footer.ENUS) == "" || strings.TrimSpace(config.Footer.ZHCN) == "" || strings.TrimSpace(config.SEOTitle.ENUS) == "" || strings.TrimSpace(config.SEOTitle.ZHCN) == "" || strings.TrimSpace(config.SEODescription.ENUS) == "" || strings.TrimSpace(config.SEODescription.ZHCN) == "" {
		return gerror.New("brand navigation, footer and SEO translations are incomplete")
	}
	for _, item := range config.Navigation {
		if strings.TrimSpace(item.Label.ENUS) == "" || strings.TrimSpace(item.Label.ZHCN) == "" || !safePublicPath(item.Href) {
			return gerror.New("brand navigation contains an unsafe link")
		}
	}
	for _, block := range config.HomepageBlocks {
		if strings.TrimSpace(block.Title.ENUS) == "" || strings.TrimSpace(block.Title.ZHCN) == "" || strings.TrimSpace(block.Description.ENUS) == "" || strings.TrimSpace(block.Description.ZHCN) == "" || (block.Href != "" && !safePublicPath(block.Href)) {
			return gerror.New("brand homepage block is incomplete or unsafe")
		}
	}
	serialized, err := json.Marshal(config)
	if err != nil {
		return err
	}
	unsafeText := strings.ToLower(string(serialized)) + strings.ToLower(config.SiteName.ENUS+config.SiteName.ZHCN+config.Footer.ENUS+config.Footer.ZHCN+config.SEOTitle.ENUS+config.SEOTitle.ZHCN+config.SEODescription.ENUS+config.SEODescription.ZHCN+config.LogoURL+config.TermsURL+config.ContactEmail+config.ContactPhone)
	if strings.Contains(unsafeText, "<script") || strings.Contains(unsafeText, "javascript:") || strings.Contains(unsafeText, "<iframe") || strings.Contains(unsafeText, "onerror=") {
		return gerror.New("brand configuration cannot contain code")
	}
	return nil
}

func safeRevision(value string) bool { return revisionPattern.MatchString(value) }

func safePublicPath(value string) bool {
	return strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") && !strings.HasPrefix(value, "/admin") && !strings.HasPrefix(value, "/api") && !strings.HasPrefix(value, "/x") && !strings.HasPrefix(value, "/account") && !strings.HasPrefix(value, "/auth") && !strings.HasPrefix(value, "/cart") && !strings.HasPrefix(value, "/checkout")
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmp := fmt.Sprintf("%s.tmp-%d", path, os.Getpid())
	if err := os.WriteFile(tmp, data, 0o640); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func readJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}
