package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// User is an application account authenticated via JWT (not a Gmail password).
type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	DisplayName  string    `json:"display_name"`
	PasswordHash string    `json:"-"`
	DarkMode     bool      `json:"dark_mode"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// GmailAccount is a linked Gmail inbox using OAuth tokens only.
type GmailAccount struct {
	ID                uuid.UUID  `json:"id"`
	UserID            uuid.UUID  `json:"user_id"`
	Email             string     `json:"email"`
	GoogleUserID      string     `json:"google_user_id"`
	AccessTokenEnc    []byte     `json:"-"`
	RefreshTokenEnc   []byte     `json:"-"`
	TokenExpiry       time.Time  `json:"token_expiry"`
	HistoryID         string     `json:"history_id,omitempty"`
	WatchExpiration   *time.Time `json:"watch_expiration,omitempty"`
	IsActive          bool       `json:"is_active"`
	NotificationsOn   bool       `json:"notifications_on"`
	LastSyncedAt      *time.Time `json:"last_synced_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// DeviceToken stores an FCM registration token for a mobile device.
type DeviceToken struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Token     string    `json:"token"`
	Platform  string    `json:"platform"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NotificationRecord is a stored push notification event for history.
type NotificationRecord struct {
	ID             uuid.UUID `json:"id"`
	UserID         uuid.UUID `json:"user_id"`
	GmailAccountID uuid.UUID `json:"gmail_account_id"`
	MessageID      string    `json:"message_id"`
	ThreadID       string    `json:"thread_id,omitempty"`
	FromAddress    string    `json:"from_address"`
	Subject        string    `json:"subject"`
	Snippet        string    `json:"snippet"`
	ReceivedAt     time.Time `json:"received_at"`
	IsRead         bool      `json:"is_read"`
	CreatedAt      time.Time `json:"created_at"`
}

// NotificationSettings controls which emails trigger pushes.
type NotificationSettings struct {
	ID                   uuid.UUID `json:"id"`
	UserID               uuid.UUID `json:"user_id"`
	Enabled              bool      `json:"enabled"`
	QuietHoursEnabled    bool      `json:"quiet_hours_enabled"`
	QuietHoursStart      string    `json:"quiet_hours_start"` // HH:MM
	QuietHoursEnd        string    `json:"quiet_hours_end"`
	OnlyPrimary          bool      `json:"only_primary"`
	IncludeSpam          bool      `json:"include_spam"`
	KeywordFilter        string    `json:"keyword_filter,omitempty"`
	SenderAllowlist      string    `json:"sender_allowlist,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// RefreshToken persists refresh tokens for rotation.
type RefreshToken struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"user_id"`
	TokenHash string     `json:"-"`
	ExpiresAt time.Time  `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// OAuthState holds short-lived CSRF state for Google OAuth.
type OAuthState struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	State     string    `json:"state"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// QRLoginSession pairs a desktop waiting browser with a phone Google OAuth scan.
type QRLoginSession struct {
	ID           uuid.UUID  `json:"id"`
	OAuthState   string     `json:"-"`
	Status       string     `json:"status"` // pending | approved | expired | failed
	UserID       *uuid.UUID `json:"user_id,omitempty"`
	AccessToken  string     `json:"-"`
	RefreshToken string     `json:"-"`
	ExpiresAt    time.Time  `json:"expires_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

const (
	QRStatusPending  = "pending"
	QRStatusApproved = "approved"
	QRStatusExpired  = "expired"
	QRStatusFailed   = "failed"
)

// Repository interfaces (ports) — implementations live in repository package.

type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	Update(ctx context.Context, user *User) error
}

type GmailAccountRepository interface {
	Create(ctx context.Context, account *GmailAccount) error
	Update(ctx context.Context, account *GmailAccount) error
	GetByID(ctx context.Context, id uuid.UUID) (*GmailAccount, error)
	GetByUserAndGoogleID(ctx context.Context, userID uuid.UUID, googleUserID string) (*GmailAccount, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]*GmailAccount, error)
	ListActiveWithWatch(ctx context.Context) ([]*GmailAccount, error)
	Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
}

type DeviceTokenRepository interface {
	Upsert(ctx context.Context, device *DeviceToken) error
	ListByUser(ctx context.Context, userID uuid.UUID) ([]*DeviceToken, error)
	Delete(ctx context.Context, userID uuid.UUID, token string) error
	DeleteByToken(ctx context.Context, token string) error
}

type NotificationRepository interface {
	Create(ctx context.Context, n *NotificationRecord) error
	ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*NotificationRecord, error)
	MarkRead(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	MarkAllRead(ctx context.Context, userID uuid.UUID) error
	CountUnread(ctx context.Context, userID uuid.UUID) (int, error)
}

type SettingsRepository interface {
	GetByUser(ctx context.Context, userID uuid.UUID) (*NotificationSettings, error)
	Upsert(ctx context.Context, s *NotificationSettings) error
}

type RefreshTokenRepository interface {
	Create(ctx context.Context, t *RefreshToken) error
	GetByHash(ctx context.Context, hash string) (*RefreshToken, error)
	Revoke(ctx context.Context, id uuid.UUID) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error
}

type OAuthStateRepository interface {
	Create(ctx context.Context, s *OAuthState) error
	Consume(ctx context.Context, state string) (*OAuthState, error)
}

type QRLoginSessionRepository interface {
	Create(ctx context.Context, s *QRLoginSession) error
	GetByID(ctx context.Context, id uuid.UUID) (*QRLoginSession, error)
	GetByOAuthState(ctx context.Context, state string) (*QRLoginSession, error)
	Approve(ctx context.Context, id uuid.UUID, userID uuid.UUID, accessToken, refreshToken string) error
	MarkFailed(ctx context.Context, id uuid.UUID) error
	ConsumeApproved(ctx context.Context, id uuid.UUID) (*QRLoginSession, error)
}
