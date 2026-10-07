package challenge

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type turnstileResponse struct {
	Success bool   `json:"success"`
	Action  string `json:"action"`
}

func (s *serviceImpl) verifyTurnstile(ctx context.Context, token string, expectedAction string) (bool, error) {
	values := url.Values{}
	values.Set("secret", s.config.TurnstileSecret)
	values.Set("response", token)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.config.TurnstileURL, strings.NewReader(values.Encode()))
	if err != nil {
		return false, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := s.httpClient.Do(request)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return false, nil
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 4097))
	if err != nil {
		return false, err
	}
	if len(body) > 4096 {
		return false, errors.New("turnstile verification response exceeds the bounded limit")
	}
	var result turnstileResponse
	if err = json.Unmarshal(body, &result); err != nil {
		return false, err
	}
	return result.Success && result.Action == expectedAction, nil
}

func turnstileAction(caller string, purpose string) string {
	// Turnstile action values are intentionally short and opaque. The binding
	// remains enforced by the owner database rather than exposed to the widget.
	return "shared_" + digest(caller, purpose)[:20]
}
