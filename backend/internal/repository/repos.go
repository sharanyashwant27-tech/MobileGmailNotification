package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yashs/mobile-gmail-notification/internal/domain"
	"github.com/yashs/mobile-gmail-notification/pkg/apperrors"
)

type DeviceTokenRepo struct {
	pool *pgxpool.Pool
}

func NewDeviceTokenRepo(pool *pgxpool.Pool) *DeviceTokenRepo {
	return &DeviceTokenRepo{pool: pool}
}

func (r *DeviceTokenRepo) Upsert(ctx context.Context, d *domain.DeviceToken) error {
	const q = `
		INSERT INTO device_tokens (id, user_id, token, platform, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, token) DO UPDATE SET platform = EXCLUDED.platform, updated_at = EXCLUDED.updated_at`
	now := time.Now().UTC()
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	d.CreatedAt = now
	d.UpdatedAt = now
	_, err := r.pool.Exec(ctx, q, d.ID, d.UserID, d.Token, d.Platform, d.CreatedAt, d.UpdatedAt)
	return err
}

func (r *DeviceTokenRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]*domain.DeviceToken, error) {
	const q = `
		SELECT id, user_id, token, platform, created_at, updated_at
		FROM device_tokens WHERE user_id = $1`
	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*domain.DeviceToken
	for rows.Next() {
		var d domain.DeviceToken
		if err := rows.Scan(&d.ID, &d.UserID, &d.Token, &d.Platform, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, &d)
	}
	return list, rows.Err()
}

func (r *DeviceTokenRepo) Delete(ctx context.Context, userID uuid.UUID, token string) error {
	const q = `DELETE FROM device_tokens WHERE user_id = $1 AND token = $2`
	_, err := r.pool.Exec(ctx, q, userID, token)
	return err
}

func (r *DeviceTokenRepo) DeleteByToken(ctx context.Context, token string) error {
	const q = `DELETE FROM device_tokens WHERE token = $1`
	_, err := r.pool.Exec(ctx, q, token)
	return err
}

type NotificationRepo struct {
	pool *pgxpool.Pool
}

func NewNotificationRepo(pool *pgxpool.Pool) *NotificationRepo {
	return &NotificationRepo{pool: pool}
}

func (r *NotificationRepo) Create(ctx context.Context, n *domain.NotificationRecord) error {
	const q = `
		INSERT INTO notification_records (
			id, user_id, gmail_account_id, message_id, thread_id, from_address,
			subject, snippet, received_at, is_read, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (gmail_account_id, message_id) DO NOTHING`
	now := time.Now().UTC()
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	n.CreatedAt = now
	_, err := r.pool.Exec(ctx, q,
		n.ID, n.UserID, n.GmailAccountID, n.MessageID, n.ThreadID, n.FromAddress,
		n.Subject, n.Snippet, n.ReceivedAt, n.IsRead, n.CreatedAt,
	)
	return err
}

func (r *NotificationRepo) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.NotificationRecord, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	const q = `
		SELECT id, user_id, gmail_account_id, message_id, thread_id, from_address,
			subject, snippet, received_at, is_read, created_at
		FROM notification_records
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`
	rows, err := r.pool.Query(ctx, q, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*domain.NotificationRecord
	for rows.Next() {
		var n domain.NotificationRecord
		if err := rows.Scan(
			&n.ID, &n.UserID, &n.GmailAccountID, &n.MessageID, &n.ThreadID, &n.FromAddress,
			&n.Subject, &n.Snippet, &n.ReceivedAt, &n.IsRead, &n.CreatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, &n)
	}
	return list, rows.Err()
}

func (r *NotificationRepo) MarkRead(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	const q = `UPDATE notification_records SET is_read = TRUE WHERE id = $1 AND user_id = $2`
	tag, err := r.pool.Exec(ctx, q, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *NotificationRepo) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	const q = `UPDATE notification_records SET is_read = TRUE WHERE user_id = $1 AND is_read = FALSE`
	_, err := r.pool.Exec(ctx, q, userID)
	return err
}

func (r *NotificationRepo) CountUnread(ctx context.Context, userID uuid.UUID) (int, error) {
	const q = `SELECT COUNT(*) FROM notification_records WHERE user_id = $1 AND is_read = FALSE`
	var n int
	err := r.pool.QueryRow(ctx, q, userID).Scan(&n)
	return n, err
}

type SettingsRepo struct {
	pool *pgxpool.Pool
}

func NewSettingsRepo(pool *pgxpool.Pool) *SettingsRepo {
	return &SettingsRepo{pool: pool}
}

func (r *SettingsRepo) GetByUser(ctx context.Context, userID uuid.UUID) (*domain.NotificationSettings, error) {
	const q = `
		SELECT id, user_id, enabled, quiet_hours_enabled, quiet_hours_start, quiet_hours_end,
			only_primary, include_spam, keyword_filter, sender_allowlist, created_at, updated_at
		FROM notification_settings WHERE user_id = $1`
	var s domain.NotificationSettings
	err := r.pool.QueryRow(ctx, q, userID).Scan(
		&s.ID, &s.UserID, &s.Enabled, &s.QuietHoursEnabled, &s.QuietHoursStart, &s.QuietHoursEnd,
		&s.OnlyPrimary, &s.IncludeSpam, &s.KeywordFilter, &s.SenderAllowlist, &s.CreatedAt, &s.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SettingsRepo) Upsert(ctx context.Context, s *domain.NotificationSettings) error {
	const q = `
		INSERT INTO notification_settings (
			id, user_id, enabled, quiet_hours_enabled, quiet_hours_start, quiet_hours_end,
			only_primary, include_spam, keyword_filter, sender_allowlist, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (user_id) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			quiet_hours_enabled = EXCLUDED.quiet_hours_enabled,
			quiet_hours_start = EXCLUDED.quiet_hours_start,
			quiet_hours_end = EXCLUDED.quiet_hours_end,
			only_primary = EXCLUDED.only_primary,
			include_spam = EXCLUDED.include_spam,
			keyword_filter = EXCLUDED.keyword_filter,
			sender_allowlist = EXCLUDED.sender_allowlist,
			updated_at = EXCLUDED.updated_at`
	now := time.Now().UTC()
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = now
	}
	s.UpdatedAt = now
	_, err := r.pool.Exec(ctx, q,
		s.ID, s.UserID, s.Enabled, s.QuietHoursEnabled, s.QuietHoursStart, s.QuietHoursEnd,
		s.OnlyPrimary, s.IncludeSpam, s.KeywordFilter, s.SenderAllowlist, s.CreatedAt, s.UpdatedAt,
	)
	return err
}

type RefreshTokenRepo struct {
	pool *pgxpool.Pool
}

func NewRefreshTokenRepo(pool *pgxpool.Pool) *RefreshTokenRepo {
	return &RefreshTokenRepo{pool: pool}
}

func (r *RefreshTokenRepo) Create(ctx context.Context, t *domain.RefreshToken) error {
	const q = `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)`
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	t.CreatedAt = time.Now().UTC()
	_, err := r.pool.Exec(ctx, q, t.ID, t.UserID, t.TokenHash, t.ExpiresAt, t.CreatedAt)
	return err
}

func (r *RefreshTokenRepo) GetByHash(ctx context.Context, hash string) (*domain.RefreshToken, error) {
	const q = `
		SELECT id, user_id, token_hash, expires_at, revoked_at, created_at
		FROM refresh_tokens WHERE token_hash = $1`
	var t domain.RefreshToken
	err := r.pool.QueryRow(ctx, q, hash).Scan(&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.RevokedAt, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *RefreshTokenRepo) Revoke(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE refresh_tokens SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL`
	_, err := r.pool.Exec(ctx, q, id)
	return err
}

func (r *RefreshTokenRepo) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	const q = `UPDATE refresh_tokens SET revoked_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL`
	_, err := r.pool.Exec(ctx, q, userID)
	return err
}

type OAuthStateRepo struct {
	pool *pgxpool.Pool
}

func NewOAuthStateRepo(pool *pgxpool.Pool) *OAuthStateRepo {
	return &OAuthStateRepo{pool: pool}
}

func (r *OAuthStateRepo) Create(ctx context.Context, s *domain.OAuthState) error {
	const q = `
		INSERT INTO oauth_states (id, user_id, state, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)`
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	s.CreatedAt = time.Now().UTC()
	_, err := r.pool.Exec(ctx, q, s.ID, s.UserID, s.State, s.ExpiresAt, s.CreatedAt)
	return err
}

func (r *OAuthStateRepo) Consume(ctx context.Context, state string) (*domain.OAuthState, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	const sel = `
		SELECT id, user_id, state, expires_at, created_at
		FROM oauth_states WHERE state = $1 FOR UPDATE`
	var s domain.OAuthState
	err = tx.QueryRow(ctx, sel, state).Scan(&s.ID, &s.UserID, &s.State, &s.ExpiresAt, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM oauth_states WHERE id = $1`, s.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &s, nil
}

type QRLoginSessionRepo struct {
	pool *pgxpool.Pool
}

func NewQRLoginSessionRepo(pool *pgxpool.Pool) *QRLoginSessionRepo {
	return &QRLoginSessionRepo{pool: pool}
}

func (r *QRLoginSessionRepo) Create(ctx context.Context, s *domain.QRLoginSession) error {
	const q = `
		INSERT INTO qr_login_sessions (id, oauth_state, status, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)`
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	s.CreatedAt = time.Now().UTC()
	if s.Status == "" {
		s.Status = domain.QRStatusPending
	}
	_, err := r.pool.Exec(ctx, q, s.ID, s.OAuthState, s.Status, s.ExpiresAt, s.CreatedAt)
	return err
}

func (r *QRLoginSessionRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.QRLoginSession, error) {
	const q = `
		SELECT id, oauth_state, status, user_id, access_token, refresh_token, expires_at, created_at
		FROM qr_login_sessions WHERE id = $1`
	return r.scanOne(ctx, q, id)
}

func (r *QRLoginSessionRepo) GetByOAuthState(ctx context.Context, state string) (*domain.QRLoginSession, error) {
	const q = `
		SELECT id, oauth_state, status, user_id, access_token, refresh_token, expires_at, created_at
		FROM qr_login_sessions WHERE oauth_state = $1`
	return r.scanOne(ctx, q, state)
}

func (r *QRLoginSessionRepo) Approve(ctx context.Context, id uuid.UUID, userID uuid.UUID, accessToken, refreshToken string) error {
	const q = `
		UPDATE qr_login_sessions
		SET status = $2, user_id = $3, access_token = $4, refresh_token = $5
		WHERE id = $1 AND status = $6`
	tag, err := r.pool.Exec(ctx, q, id, domain.QRStatusApproved, userID, accessToken, refreshToken, domain.QRStatusPending)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrConflict
	}
	return nil
}

func (r *QRLoginSessionRepo) MarkFailed(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE qr_login_sessions SET status = $2 WHERE id = $1 AND status = $3`
	_, err := r.pool.Exec(ctx, q, id, domain.QRStatusFailed, domain.QRStatusPending)
	return err
}

func (r *QRLoginSessionRepo) ConsumeApproved(ctx context.Context, id uuid.UUID) (*domain.QRLoginSession, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	const sel = `
		SELECT id, oauth_state, status, user_id, access_token, refresh_token, expires_at, created_at
		FROM qr_login_sessions WHERE id = $1 FOR UPDATE`
	var s domain.QRLoginSession
	err = tx.QueryRow(ctx, sel, id).Scan(
		&s.ID, &s.OAuthState, &s.Status, &s.UserID, &s.AccessToken, &s.RefreshToken, &s.ExpiresAt, &s.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if s.Status != domain.QRStatusApproved {
		return &s, nil
	}
	// One-time token handoff — clear secrets after read.
	if _, err := tx.Exec(ctx, `
		UPDATE qr_login_sessions
		SET access_token = NULL, refresh_token = NULL, status = 'consumed'
		WHERE id = $1`, id); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *QRLoginSessionRepo) scanOne(ctx context.Context, q string, arg any) (*domain.QRLoginSession, error) {
	var s domain.QRLoginSession
	var access, refresh *string
	err := r.pool.QueryRow(ctx, q, arg).Scan(
		&s.ID, &s.OAuthState, &s.Status, &s.UserID, &access, &refresh, &s.ExpiresAt, &s.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if access != nil {
		s.AccessToken = *access
	}
	if refresh != nil {
		s.RefreshToken = *refresh
	}
	return &s, nil
}
