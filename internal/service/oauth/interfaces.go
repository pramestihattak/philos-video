package oauth

import "context"

// Servicer is the interface for Google OAuth operations.
type Servicer interface {
	BuildAuthURL(state string) string
	ExchangeAndFetchUserInfo(ctx context.Context, code string) (*GoogleUser, error)
}
