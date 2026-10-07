package challenge

import (
	"os"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
)

const (
	providerEnvironment         = "LINAPRO_SHARED_CHALLENGE_PROVIDER"
	turnstileSiteKeyEnvironment = "LINAPRO_SHARED_TURNSTILE_SITE_KEY"
	turnstileSecretEnvironment  = "LINAPRO_SHARED_TURNSTILE_SECRET"
	turnstileVerifyURL          = "https://challenges.cloudflare.com/turnstile/v0/siteverify"
	challengeLifetime           = 5 * time.Minute
	proofLifetime               = 2 * time.Minute
	maximumAttempts             = 5
)

type Config struct {
	Provider         string
	TurnstileSiteKey string
	TurnstileSecret  string
	TurnstileURL     string
	RequestTimeout   time.Duration
}

func LoadConfig() (Config, error) {
	provider := strings.TrimSpace(os.Getenv(providerEnvironment))
	if provider == "" {
		provider = "local"
	}
	config := Config{Provider: provider, TurnstileURL: turnstileVerifyURL, RequestTimeout: 10 * time.Second}
	switch provider {
	case "local":
		return config, nil
	case "turnstile":
		config.TurnstileSiteKey = strings.TrimSpace(os.Getenv(turnstileSiteKeyEnvironment))
		config.TurnstileSecret = strings.TrimSpace(os.Getenv(turnstileSecretEnvironment))
		if config.TurnstileSiteKey == "" || len(config.TurnstileSiteKey) > 256 ||
			config.TurnstileSecret == "" || len(config.TurnstileSecret) > 512 {
			return Config{}, gerror.New("shared challenge Turnstile configuration is incomplete")
		}
		return config, nil
	default:
		return Config{}, gerror.New("shared challenge provider is unsupported")
	}
}
