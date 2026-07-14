package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"

	"github.com/yashs/mobile-gmail-notification/internal/auth"
	"github.com/yashs/mobile-gmail-notification/internal/domain"
	gmailclient "github.com/yashs/mobile-gmail-notification/internal/gmail"
	"github.com/yashs/mobile-gmail-notification/pkg/apperrors"
	"github.com/yashs/mobile-gmail-notification/pkg/crypto"
)

// GmailAccountService manages OAuth-linked Gmail inboxes (tokens only).
type GmailAccountService struct {
	accounts   domain.GmailAccountRepository
	oauthState domain.OAuthStateRepository
	google     *auth.GoogleOAuth
	encKey     []byte
	topic      string
	labelIDs   []string
	log        *slog.Logger
}

func NewGmailAccountService(
	accounts domain.GmailAccountRepository,
	oauthState domain.OAuthStateRepository,
	google *auth.GoogleOAuth,
	encKey []byte,
	topic string,
	labelIDs []string,
	log *slog.Logger,
) *GmailAccountService {
	return &GmailAccountService{
		accounts: accounts, oauthState: oauthState, google: google,
		encKey: encKey, topic: topic, labelIDs: labelIDs, log: log,
	}
}

func (s *GmailAccountService) BeginLink(ctx context.Context, userID uuid.UUID) (authURL string, err error) {
	state, err := crypto.RandomToken(24)
	if err != nil {
		return "", apperrors.Wrap(err, "internal_error", "failed to create oauth state", 500)
	}
	rec := &domain.OAuthState{
		UserID:    userID,
		State:     state,
		ExpiresAt: time.Now().UTC().Add(10 * time.Minute),
	}
	if err := s.oauthState.Create(ctx, rec); err != nil {
		return "", apperrors.Wrap(err, "internal_error", "failed to store oauth state", 500)
	}
	return s.google.AuthCodeURL(state), nil
}

func (s *GmailAccountService) CompleteLink(ctx context.Context, state, code string) (*domain.GmailAccount, error) {
	st, err := s.oauthState.Consume(ctx, state)
	if err != nil {
		return nil, apperrors.ErrOAuthFailed
	}
	if time.Now().UTC().After(st.ExpiresAt) {
		return nil, apperrors.Wrap(nil, "oauth_failed", "oauth state expired", 400)
	}

	token, err := s.google.Exchange(ctx, code)
	if err != nil {
		s.log.Error("oauth exchange failed", "error", err)
		return nil, apperrors.ErrOAuthFailed
	}

	client, err := gmailclient.NewClient(ctx, s.google.TokenSource(ctx, token))
	if err != nil {
		return nil, apperrors.Wrap(err, "oauth_failed", "failed to create gmail client", 400)
	}
	profile, err := client.Profile(ctx)
	if err != nil {
		return nil, apperrors.Wrap(err, "oauth_failed", "failed to fetch gmail profile", 400)
	}

	accessEnc, err := crypto.Encrypt(s.encKey, []byte(token.AccessToken))
	if err != nil {
		return nil, apperrors.Wrap(err, "internal_error", "failed to encrypt token", 500)
	}
	refreshEnc, err := crypto.Encrypt(s.encKey, []byte(token.RefreshToken))
	if err != nil {
		return nil, apperrors.Wrap(err, "internal_error", "failed to encrypt refresh token", 500)
	}

	existing, err := s.accounts.GetByUserAndGoogleID(ctx, st.UserID, profile.EmailAddress)
	account := &domain.GmailAccount{
		UserID:          st.UserID,
		Email:           profile.EmailAddress,
		GoogleUserID:    profile.EmailAddress,
		AccessTokenEnc:  accessEnc,
		RefreshTokenEnc: refreshEnc,
		TokenExpiry:     token.Expiry.UTC(),
		HistoryID:       strconv.FormatUint(profile.HistoryId, 10),
		IsActive:        true,
		NotificationsOn: true,
	}

	if err == nil {
		account.ID = existing.ID
		account.NotificationsOn = existing.NotificationsOn
		if err := s.accounts.Update(ctx, account); err != nil {
			return nil, apperrors.Wrap(err, "internal_error", "failed to update gmail account", 500)
		}
	} else if errors.Is(err, apperrors.ErrNotFound) {
		if err := s.accounts.Create(ctx, account); err != nil {
			return nil, apperrors.Wrap(err, "internal_error", "failed to save gmail account", 500)
		}
	} else {
		return nil, apperrors.Wrap(err, "internal_error", "failed to lookup gmail account", 500)
	}

	if s.topic != "" {
		if err := s.startWatch(ctx, account); err != nil {
			s.log.Warn("failed to start gmail watch", "error", err, "email", account.Email)
		}
	}

	return s.publicAccount(account), nil
}

func (s *GmailAccountService) List(ctx context.Context, userID uuid.UUID) ([]*domain.GmailAccount, error) {
	list, err := s.accounts.ListByUser(ctx, userID)
	if err != nil {
		return nil, apperrors.Wrap(err, "internal_error", "failed to list accounts", 500)
	}
	out := make([]*domain.GmailAccount, 0, len(list))
	for _, a := range list {
		out = append(out, s.publicAccount(a))
	}
	return out, nil
}

func (s *GmailAccountService) SetNotifications(ctx context.Context, userID, accountID uuid.UUID, enabled bool) (*domain.GmailAccount, error) {
	a, err := s.accounts.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if a.UserID != userID {
		return nil, apperrors.ErrForbidden
	}
	a.NotificationsOn = enabled
	if err := s.accounts.Update(ctx, a); err != nil {
		return nil, err
	}
	if enabled && s.topic != "" {
		_ = s.startWatch(ctx, a)
	}
	return s.publicAccount(a), nil
}

func (s *GmailAccountService) Unlink(ctx context.Context, userID, accountID uuid.UUID) error {
	a, err := s.accounts.GetByID(ctx, accountID)
	if err != nil {
		return err
	}
	if a.UserID != userID {
		return apperrors.ErrForbidden
	}
	client, err := s.gmailClientFor(ctx, a)
	if err == nil {
		_ = client.StopWatch(ctx)
	}
	return s.accounts.Delete(ctx, accountID, userID)
}

func (s *GmailAccountService) RenewWatch(ctx context.Context, account *domain.GmailAccount) error {
	return s.startWatch(ctx, account)
}

func (s *GmailAccountService) TokenSource(ctx context.Context, account *domain.GmailAccount) (oauth2.TokenSource, error) {
	access, err := crypto.Decrypt(s.encKey, account.AccessTokenEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypt access token: %w", err)
	}
	refresh, err := crypto.Decrypt(s.encKey, account.RefreshTokenEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypt refresh token: %w", err)
	}
	tok := &oauth2.Token{
		AccessToken:  string(access),
		RefreshToken: string(refresh),
		Expiry:       account.TokenExpiry,
		TokenType:    "Bearer",
	}
	ts := s.google.TokenSource(ctx, tok)
	return oauth2.ReuseTokenSource(tok, &persistingTokenSource{
		inner:   ts,
		account: account,
		svc:     s,
	}), nil
}

func (s *GmailAccountService) gmailClientFor(ctx context.Context, account *domain.GmailAccount) (*gmailclient.Client, error) {
	ts, err := s.TokenSource(ctx, account)
	if err != nil {
		return nil, err
	}
	return gmailclient.NewClient(ctx, ts)
}

func (s *GmailAccountService) startWatch(ctx context.Context, account *domain.GmailAccount) error {
	client, err := s.gmailClientFor(ctx, account)
	if err != nil {
		return err
	}
	res, err := client.StartWatch(ctx, gmailclient.WatchRequest{
		TopicName: s.topic,
		LabelIDs:  s.labelIDs,
	})
	if err != nil {
		return err
	}
	account.HistoryID = res.HistoryID
	account.WatchExpiration = &res.Expiration
	now := time.Now().UTC()
	account.LastSyncedAt = &now
	return s.accounts.Update(ctx, account)
}

func (s *GmailAccountService) publicAccount(a *domain.GmailAccount) *domain.GmailAccount {
	cp := *a
	cp.AccessTokenEnc = nil
	cp.RefreshTokenEnc = nil
	return &cp
}

func (s *GmailAccountService) GetByID(ctx context.Context, id uuid.UUID) (*domain.GmailAccount, error) {
	return s.accounts.GetByID(ctx, id)
}

func (s *GmailAccountService) UpdateAccount(ctx context.Context, account *domain.GmailAccount) error {
	return s.accounts.Update(ctx, account)
}

func (s *GmailAccountService) ListActive(ctx context.Context) ([]*domain.GmailAccount, error) {
	return s.accounts.ListActiveWithWatch(ctx)
}

func (s *GmailAccountService) FindByEmail(ctx context.Context, email string) (*domain.GmailAccount, error) {
	list, err := s.accounts.ListActiveWithWatch(ctx)
	if err != nil {
		return nil, err
	}
	email = strings.ToLower(email)
	for _, a := range list {
		if strings.ToLower(a.Email) == email {
			return a, nil
		}
	}
	return nil, apperrors.ErrNotFound
}

func (s *GmailAccountService) PersistTokens(ctx context.Context, account *domain.GmailAccount, tok *oauth2.Token) error {
	accessEnc, err := crypto.Encrypt(s.encKey, []byte(tok.AccessToken))
	if err != nil {
		return err
	}
	account.AccessTokenEnc = accessEnc
	if tok.RefreshToken != "" {
		refreshEnc, err := crypto.Encrypt(s.encKey, []byte(tok.RefreshToken))
		if err != nil {
			return err
		}
		account.RefreshTokenEnc = refreshEnc
	}
	account.TokenExpiry = tok.Expiry.UTC()
	return s.accounts.Update(ctx, account)
}

type persistingTokenSource struct {
	inner   oauth2.TokenSource
	account *domain.GmailAccount
	svc     *GmailAccountService
}

func (p *persistingTokenSource) Token() (*oauth2.Token, error) {
	tok, err := p.inner.Token()
	if err != nil {
		return nil, err
	}
	// Best-effort persist refreshed access tokens.
	_ = p.svc.PersistTokens(context.Background(), p.account, tok)
	return tok, nil
}
