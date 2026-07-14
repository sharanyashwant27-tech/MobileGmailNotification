package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"

	"github.com/yashs/mobile-gmail-notification/internal/auth"
	"github.com/yashs/mobile-gmail-notification/internal/domain"
	gmailclient "github.com/yashs/mobile-gmail-notification/internal/gmail"
	"github.com/yashs/mobile-gmail-notification/pkg/apperrors"
	"github.com/yashs/mobile-gmail-notification/pkg/crypto"
)

// QRLoginService creates scan-to-login sessions and completes Google OAuth from a phone scan.
type QRLoginService struct {
	sessions domain.QRLoginSessionRepository
	auth     *AuthService
	gmail    *GmailAccountService
	google   *auth.GoogleOAuth
	baseURL  string
	log      *slog.Logger
}

func NewQRLoginService(
	sessions domain.QRLoginSessionRepository,
	authSvc *AuthService,
	gmail *GmailAccountService,
	google *auth.GoogleOAuth,
	baseURL string,
	log *slog.Logger,
) *QRLoginService {
	return &QRLoginService{
		sessions: sessions,
		auth:     authSvc,
		gmail:    gmail,
		google:   google,
		baseURL:  strings.TrimRight(baseURL, "/"),
		log:      log,
	}
}

type QRSessionCreated struct {
	SessionID string    `json:"session_id"`
	ScanURL   string    `json:"scan_url"`
	ExpiresAt time.Time `json:"expires_at"`
}

type QRSessionStatus struct {
	Status       string       `json:"status"`
	User         *domain.User `json:"user,omitempty"`
	AccessToken  string       `json:"access_token,omitempty"`
	RefreshToken string       `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time    `json:"expires_at,omitempty"`
}

func (s *QRLoginService) CreateSession(ctx context.Context) (*QRSessionCreated, error) {
	oauthState, err := crypto.RandomToken(24)
	if err != nil {
		return nil, apperrors.Wrap(err, "internal_error", "failed to create qr session", 500)
	}
	sess := &domain.QRLoginSession{
		OAuthState: oauthState,
		Status:     domain.QRStatusPending,
		ExpiresAt:  time.Now().UTC().Add(5 * time.Minute),
	}
	if err := s.sessions.Create(ctx, sess); err != nil {
		return nil, apperrors.Wrap(err, "internal_error", "failed to store qr session", 500)
	}
	scanURL := fmt.Sprintf("%s/api/v1/auth/qr/%s/start", s.baseURL, sess.ID.String())
	return &QRSessionCreated{
		SessionID: sess.ID.String(),
		ScanURL:   scanURL,
		ExpiresAt: sess.ExpiresAt,
	}, nil
}

func (s *QRLoginService) StartURL(ctx context.Context, sessionID uuid.UUID) (string, error) {
	sess, err := s.sessions.GetByID(ctx, sessionID)
	if err != nil {
		return "", err
	}
	if sess.Status != domain.QRStatusPending || time.Now().UTC().After(sess.ExpiresAt) {
		return "", apperrors.Wrap(nil, "oauth_failed", "qr session expired", 400)
	}
	return s.google.AuthCodeURL(sess.OAuthState), nil
}

func (s *QRLoginService) CompleteFromOAuth(ctx context.Context, state, code string) error {
	sess, err := s.sessions.GetByOAuthState(ctx, state)
	if err != nil {
		return err
	}
	if sess.Status != domain.QRStatusPending || time.Now().UTC().After(sess.ExpiresAt) {
		_ = s.sessions.MarkFailed(ctx, sess.ID)
		return apperrors.Wrap(nil, "oauth_failed", "qr session expired", 400)
	}

	token, err := s.google.Exchange(ctx, code)
	if err != nil {
		_ = s.sessions.MarkFailed(ctx, sess.ID)
		s.log.Error("qr oauth exchange failed", "error", err)
		return apperrors.ErrOAuthFailed
	}

	client, err := gmailclient.NewClient(ctx, s.google.TokenSource(ctx, token))
	if err != nil {
		_ = s.sessions.MarkFailed(ctx, sess.ID)
		return apperrors.Wrap(err, "oauth_failed", "failed to create gmail client", 400)
	}
	profile, err := client.Profile(ctx)
	if err != nil {
		_ = s.sessions.MarkFailed(ctx, sess.ID)
		return apperrors.Wrap(err, "oauth_failed", "failed to fetch gmail profile", 400)
	}

	authRes, err := s.auth.LoginOrCreateGoogleUser(ctx, profile.EmailAddress, displayNameFromEmail(profile.EmailAddress))
	if err != nil {
		_ = s.sessions.MarkFailed(ctx, sess.ID)
		return err
	}

	// Best-effort: also link/upsert this Gmail mailbox for the logged-in user.
	if err := s.gmail.UpsertFromOAuthToken(ctx, authRes.User.ID, profile.EmailAddress, token); err != nil {
		s.log.Warn("qr login linked user but gmail upsert failed", "error", err, "email", profile.EmailAddress)
	}

	if err := s.sessions.Approve(ctx, sess.ID, authRes.User.ID, authRes.AccessToken, authRes.RefreshToken); err != nil {
		return apperrors.Wrap(err, "internal_error", "failed to approve qr session", 500)
	}
	return nil
}

func (s *QRLoginService) Status(ctx context.Context, sessionID uuid.UUID) (*QRSessionStatus, error) {
	sess, err := s.sessions.GetByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if sess.Status == domain.QRStatusPending && time.Now().UTC().After(sess.ExpiresAt) {
		_ = s.sessions.MarkFailed(ctx, sess.ID)
		return &QRSessionStatus{Status: domain.QRStatusExpired}, nil
	}
	if sess.Status != domain.QRStatusApproved {
		return &QRSessionStatus{Status: sess.Status}, nil
	}

	consumed, err := s.sessions.ConsumeApproved(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if consumed.Status != domain.QRStatusApproved {
		return &QRSessionStatus{Status: consumed.Status}, nil
	}

	user, err := s.auth.Me(ctx, *consumed.UserID)
	if err != nil {
		return nil, err
	}
	return &QRSessionStatus{
		Status:       domain.QRStatusApproved,
		User:         user,
		AccessToken:  consumed.AccessToken,
		RefreshToken: consumed.RefreshToken,
	}, nil
}

// TryCompleteQR returns true when state belongs to a QR login session.
func (s *QRLoginService) TryCompleteQR(ctx context.Context, state, code string) (handled bool, err error) {
	_, err = s.sessions.GetByOAuthState(ctx, state)
	if errors.Is(err, apperrors.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return true, err
	}
	return true, s.CompleteFromOAuth(ctx, state, code)
}

func displayNameFromEmail(email string) string {
	if i := strings.IndexByte(email, '@'); i > 0 {
		return email[:i]
	}
	return email
}

// LoginOrCreateGoogleUser signs in an existing app user or creates a passwordless Google-backed account.
func (s *AuthService) LoginOrCreateGoogleUser(ctx context.Context, email, displayName string) (*AuthResult, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || !strings.Contains(email, "@") {
		return nil, apperrors.Wrap(nil, "validation_error", "invalid google email", 400)
	}
	user, err := s.users.GetByEmail(ctx, email)
	if err == nil {
		return s.issueTokens(ctx, user)
	}
	if !errors.Is(err, apperrors.ErrNotFound) {
		return nil, apperrors.Wrap(err, "internal_error", "failed to lookup user", 500)
	}

	randPass, err := crypto.RandomToken(32)
	if err != nil {
		return nil, apperrors.Wrap(err, "internal_error", "failed to provision user", 500)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(randPass), bcrypt.DefaultCost)
	if err != nil {
		return nil, apperrors.Wrap(err, "internal_error", "failed to hash password", 500)
	}
	user = &domain.User{
		Email:        email,
		DisplayName:  displayName,
		PasswordHash: string(hash),
	}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, apperrors.Wrap(err, "internal_error", "failed to create user", 500)
	}
	_ = s.settings.Upsert(ctx, &domain.NotificationSettings{
		UserID:          user.ID,
		Enabled:         true,
		QuietHoursStart: "22:00",
		QuietHoursEnd:   "07:00",
		OnlyPrimary:     true,
	})
	return s.issueTokens(ctx, user)
}

// UpsertFromOAuthToken stores/updates a linked Gmail account from an OAuth token (tokens only).
func (s *GmailAccountService) UpsertFromOAuthToken(ctx context.Context, userID uuid.UUID, email string, token *oauth2.Token) error {
	if token == nil || token.AccessToken == "" {
		return fmt.Errorf("missing access token")
	}
	accessEnc, err := crypto.Encrypt(s.encKey, []byte(token.AccessToken))
	if err != nil {
		return err
	}
	refresh := token.RefreshToken
	if refresh == "" {
		existing, err := s.accounts.GetByUserAndGoogleID(ctx, userID, email)
		if err == nil {
			raw, decErr := crypto.Decrypt(s.encKey, existing.RefreshTokenEnc)
			if decErr == nil {
				refresh = string(raw)
			}
		}
	}
	if refresh == "" {
		return fmt.Errorf("no refresh token available")
	}
	refreshEnc, err := crypto.Encrypt(s.encKey, []byte(refresh))
	if err != nil {
		return err
	}

	client, err := gmailclient.NewClient(ctx, s.google.TokenSource(ctx, token))
	if err != nil {
		return err
	}
	profile, err := client.Profile(ctx)
	if err != nil {
		return err
	}

	account := &domain.GmailAccount{
		UserID:          userID,
		Email:           email,
		GoogleUserID:    email,
		AccessTokenEnc:  accessEnc,
		RefreshTokenEnc: refreshEnc,
		TokenExpiry:     token.Expiry.UTC(),
		HistoryID:       fmt.Sprintf("%d", profile.HistoryId),
		IsActive:        true,
		NotificationsOn: true,
	}

	existing, err := s.accounts.GetByUserAndGoogleID(ctx, userID, email)
	if err == nil {
		account.ID = existing.ID
		account.NotificationsOn = existing.NotificationsOn
		if err := s.accounts.Update(ctx, account); err != nil {
			return err
		}
	} else if errors.Is(err, apperrors.ErrNotFound) {
		if err := s.accounts.Create(ctx, account); err != nil {
			return err
		}
	} else {
		return err
	}

	if s.topic != "" {
		_ = s.startWatch(ctx, account)
	}
	return nil
}

// PublicScanHost helps warn when QR uses localhost (phones cannot open it).
func PublicScanHost(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host != "localhost" && host != "127.0.0.1"
}
