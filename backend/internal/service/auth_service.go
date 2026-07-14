package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/yashs/mobile-gmail-notification/internal/auth"
	"github.com/yashs/mobile-gmail-notification/internal/domain"
	"github.com/yashs/mobile-gmail-notification/pkg/apperrors"
	"github.com/yashs/mobile-gmail-notification/pkg/crypto"
)

// AuthService handles app registration, login, and JWT refresh (not Gmail passwords).
type AuthService struct {
	users    domain.UserRepository
	refresh  domain.RefreshTokenRepository
	settings domain.SettingsRepository
	tokens   *auth.TokenManager
}

func NewAuthService(
	users domain.UserRepository,
	refresh domain.RefreshTokenRepository,
	settings domain.SettingsRepository,
	tokens *auth.TokenManager,
) *AuthService {
	return &AuthService{users: users, refresh: refresh, settings: settings, tokens: tokens}
}

type AuthResult struct {
	User         *domain.User `json:"user"`
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresAt    time.Time    `json:"expires_at"`
}

func (s *AuthService) Register(ctx context.Context, email, password, displayName string) (*AuthResult, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || len(password) < 8 {
		return nil, apperrors.Wrap(nil, "validation_error", "email required and password must be at least 8 characters", 400)
	}
	if _, err := s.users.GetByEmail(ctx, email); err == nil {
		return nil, apperrors.ErrConflict
	} else if !errors.Is(err, apperrors.ErrNotFound) {
		return nil, apperrors.Wrap(err, "internal_error", "failed to check user", 500)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, apperrors.Wrap(err, "internal_error", "failed to hash password", 500)
	}

	user := &domain.User{
		Email:        email,
		DisplayName:  displayName,
		PasswordHash: string(hash),
	}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, apperrors.Wrap(err, "internal_error", "failed to create user", 500)
	}

	_ = s.settings.Upsert(ctx, &domain.NotificationSettings{
		UserID:            user.ID,
		Enabled:           true,
		QuietHoursStart:   "22:00",
		QuietHoursEnd:     "07:00",
		OnlyPrimary:       true,
	})

	return s.issueTokens(ctx, user)
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*AuthResult, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return nil, apperrors.ErrInvalidCredentials
		}
		return nil, apperrors.Wrap(err, "internal_error", "login failed", 500)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, apperrors.ErrInvalidCredentials
	}
	return s.issueTokens(ctx, user)
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*AuthResult, error) {
	hash := crypto.HashToken(refreshToken)
	stored, err := s.refresh.GetByHash(ctx, hash)
	if err != nil {
		return nil, apperrors.ErrInvalidToken
	}
	if stored.RevokedAt != nil || time.Now().UTC().After(stored.ExpiresAt) {
		return nil, apperrors.ErrInvalidToken
	}
	_ = s.refresh.Revoke(ctx, stored.ID)

	user, err := s.users.GetByID(ctx, stored.UserID)
	if err != nil {
		return nil, apperrors.ErrInvalidToken
	}
	return s.issueTokens(ctx, user)
}

func (s *AuthService) Logout(ctx context.Context, userID uuid.UUID) error {
	return s.refresh.RevokeAllForUser(ctx, userID)
}

func (s *AuthService) Me(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	return s.users.GetByID(ctx, userID)
}

func (s *AuthService) UpdateProfile(ctx context.Context, userID uuid.UUID, displayName string, darkMode *bool) (*domain.User, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if displayName != "" {
		user.DisplayName = displayName
	}
	if darkMode != nil {
		user.DarkMode = *darkMode
	}
	if err := s.users.Update(ctx, user); err != nil {
		return nil, apperrors.Wrap(err, "internal_error", "failed to update profile", 500)
	}
	return user, nil
}

func (s *AuthService) issueTokens(ctx context.Context, user *domain.User) (*AuthResult, error) {
	access, exp, err := s.tokens.IssueAccessToken(user.ID, user.Email)
	if err != nil {
		return nil, apperrors.Wrap(err, "internal_error", "failed to issue token", 500)
	}
	rawRefresh, err := crypto.RandomToken(32)
	if err != nil {
		return nil, apperrors.Wrap(err, "internal_error", "failed to issue refresh token", 500)
	}
	rt := &domain.RefreshToken{
		UserID:    user.ID,
		TokenHash: crypto.HashToken(rawRefresh),
		ExpiresAt: time.Now().UTC().Add(s.tokens.RefreshTTL()),
	}
	if err := s.refresh.Create(ctx, rt); err != nil {
		return nil, apperrors.Wrap(err, "internal_error", "failed to store refresh token", 500)
	}
	return &AuthResult{
		User:         user,
		AccessToken:  access,
		RefreshToken: rawRefresh,
		ExpiresAt:    exp,
	}, nil
}

// Public helper for tests / password hashing validation.
func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("bcrypt: %w", err)
	}
	return string(b), nil
}
