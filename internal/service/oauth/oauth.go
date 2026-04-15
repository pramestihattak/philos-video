package oauth

import (
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"philos-video/internal/config"
)

// GoogleUser holds the fields we read from the Google userinfo endpoint.
type GoogleUser struct {
	Sub     string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

// Service wraps the Google OAuth2 config and provides helpers for the
// authorization-code flow.
type Service struct {
	cfg *oauth2.Config
}

// New creates an OAuth Service.
func New(c *config.Config) *Service {
	return &Service{
		cfg: &oauth2.Config{
			ClientID:     c.GoogleClientID,
			ClientSecret: c.GoogleClientSecret,
			RedirectURL:  c.OAuthRedirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		},
	}
}
