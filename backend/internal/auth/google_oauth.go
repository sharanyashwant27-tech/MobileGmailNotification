package auth

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// GoogleOAuth wraps the Google OAuth2 config for Gmail access (tokens only — never passwords).
type GoogleOAuth struct {
	config *oauth2.Config
}

func NewGoogleOAuth(clientID, clientSecret, redirectURI string, scopes []string) *GoogleOAuth {
	return &GoogleOAuth{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURI,
			Scopes:       scopes,
			Endpoint:     google.Endpoint,
		},
	}
}

func (g *GoogleOAuth) AuthCodeURL(state string) string {
	return g.config.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.ApprovalForce, // ensure refresh_token is returned when linking accounts
	)
}

func (g *GoogleOAuth) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	token, err := g.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange auth code: %w", err)
	}
	if token.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token returned; user may need to revoke prior consent and reconnect")
	}
	return token, nil
}

func (g *GoogleOAuth) TokenSource(ctx context.Context, token *oauth2.Token) oauth2.TokenSource {
	return g.config.TokenSource(ctx, token)
}

func (g *GoogleOAuth) Client(ctx context.Context, token *oauth2.Token) *http.Client {
	return g.config.Client(ctx, token)
}

func (g *GoogleOAuth) Config() *oauth2.Config {
	return g.config
}

// TokenStillValid reports whether the access token has not expired (with skew).
func TokenStillValid(expiry time.Time) bool {
	return time.Now().UTC().Before(expiry.Add(-30 * time.Second))
}
