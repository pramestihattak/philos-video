package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
)

// BuildAuthURL returns the Google consent-screen URL with the given CSRF state.
func (s *Service) BuildAuthURL(state string) string {
	return s.cfg.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

// ExchangeAndFetchUserInfo exchanges an authorization code for tokens and
// then fetches the authenticated user's profile from Google's userinfo endpoint.
func (s *Service) ExchangeAndFetchUserInfo(ctx context.Context, code string) (*GoogleUser, error) {
	token, err := s.cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchanging oauth code: %w", err)
	}

	client := s.cfg.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		return nil, fmt.Errorf("fetching userinfo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo returned %d", resp.StatusCode)
	}

	var gu GoogleUser
	if err := json.NewDecoder(resp.Body).Decode(&gu); err != nil {
		return nil, fmt.Errorf("decoding userinfo: %w", err)
	}
	if gu.Sub == "" {
		return nil, fmt.Errorf("userinfo missing sub claim")
	}
	return &gu, nil
}
