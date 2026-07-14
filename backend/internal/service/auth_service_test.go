package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/yashs/mobile-gmail-notification/internal/auth"
	"github.com/yashs/mobile-gmail-notification/internal/domain"
	"github.com/yashs/mobile-gmail-notification/internal/service"
	"github.com/yashs/mobile-gmail-notification/pkg/apperrors"
)

type memUsers struct {
	byEmail map[string]*domain.User
	byID    map[uuid.UUID]*domain.User
}

func newMemUsers() *memUsers {
	return &memUsers{byEmail: map[string]*domain.User{}, byID: map[uuid.UUID]*domain.User{}}
}

func (m *memUsers) Create(_ context.Context, user *domain.User) error {
	if _, ok := m.byEmail[user.Email]; ok {
		return errors.New("exists")
	}
	cp := *user
	m.byEmail[user.Email] = &cp
	m.byID[user.ID] = &cp
	return nil
}

func (m *memUsers) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	u, ok := m.byID[id]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	cp := *u
	return &cp, nil
}

func (m *memUsers) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	u, ok := m.byEmail[email]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	cp := *u
	return &cp, nil
}

func (m *memUsers) Update(_ context.Context, user *domain.User) error {
	m.byEmail[user.Email] = user
	m.byID[user.ID] = user
	return nil
}

type memRefresh struct {
	items map[string]*domain.RefreshToken
}

func newMemRefresh() *memRefresh {
	return &memRefresh{items: map[string]*domain.RefreshToken{}}
}

func (m *memRefresh) Create(_ context.Context, t *domain.RefreshToken) error {
	cp := *t
	m.items[t.TokenHash] = &cp
	return nil
}

func (m *memRefresh) GetByHash(_ context.Context, hash string) (*domain.RefreshToken, error) {
	t, ok := m.items[hash]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	cp := *t
	return &cp, nil
}

func (m *memRefresh) Revoke(_ context.Context, id uuid.UUID) error {
	for _, t := range m.items {
		if t.ID == id {
			now := time.Now().UTC()
			t.RevokedAt = &now
		}
	}
	return nil
}

func (m *memRefresh) RevokeAllForUser(_ context.Context, userID uuid.UUID) error {
	now := time.Now().UTC()
	for _, t := range m.items {
		if t.UserID == userID {
			t.RevokedAt = &now
		}
	}
	return nil
}

type memSettings struct{}

func (memSettings) GetByUser(context.Context, uuid.UUID) (*domain.NotificationSettings, error) {
	return nil, apperrors.ErrNotFound
}

func (memSettings) Upsert(context.Context, *domain.NotificationSettings) error { return nil }

func TestAuthRegisterLoginRoundTrip(t *testing.T) {
	tm := auth.NewTokenManager("01234567890123456789012345678901", time.Minute, time.Hour)
	svc := service.NewAuthService(newMemUsers(), newMemRefresh(), memSettings{}, tm)

	reg, err := svc.Register(context.Background(), "User@Example.com", "password123", "Ada")
	if err != nil {
		t.Fatal(err)
	}
	if reg.AccessToken == "" || reg.RefreshToken == "" {
		t.Fatal("missing tokens")
	}
	if reg.User.Email != "user@example.com" {
		t.Fatalf("email normalized: %s", reg.User.Email)
	}

	login, err := svc.Login(context.Background(), "user@example.com", "password123")
	if err != nil {
		t.Fatal(err)
	}
	if login.User.ID != reg.User.ID {
		t.Fatal("user id mismatch")
	}

	_, err = svc.Login(context.Background(), "user@example.com", "wrong-password")
	if !errors.Is(err, apperrors.ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
}
