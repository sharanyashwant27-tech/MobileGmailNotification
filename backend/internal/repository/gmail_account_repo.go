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

type GmailAccountRepo struct {
	pool *pgxpool.Pool
}

func NewGmailAccountRepo(pool *pgxpool.Pool) *GmailAccountRepo {
	return &GmailAccountRepo{pool: pool}
}

func (r *GmailAccountRepo) Create(ctx context.Context, a *domain.GmailAccount) error {
	const q = `
		INSERT INTO gmail_accounts (
			id, user_id, email, google_user_id, access_token_enc, refresh_token_enc,
			token_expiry, history_id, watch_expiration, is_active, notifications_on,
			last_synced_at, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`
	now := time.Now().UTC()
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	a.CreatedAt = now
	a.UpdatedAt = now
	_, err := r.pool.Exec(ctx, q,
		a.ID, a.UserID, a.Email, a.GoogleUserID, a.AccessTokenEnc, a.RefreshTokenEnc,
		a.TokenExpiry, a.HistoryID, a.WatchExpiration, a.IsActive, a.NotificationsOn,
		a.LastSyncedAt, a.CreatedAt, a.UpdatedAt,
	)
	return err
}

func (r *GmailAccountRepo) Update(ctx context.Context, a *domain.GmailAccount) error {
	const q = `
		UPDATE gmail_accounts SET
			email=$2, access_token_enc=$3, refresh_token_enc=$4, token_expiry=$5,
			history_id=$6, watch_expiration=$7, is_active=$8, notifications_on=$9,
			last_synced_at=$10, updated_at=$11
		WHERE id=$1`
	a.UpdatedAt = time.Now().UTC()
	tag, err := r.pool.Exec(ctx, q,
		a.ID, a.Email, a.AccessTokenEnc, a.RefreshTokenEnc, a.TokenExpiry,
		a.HistoryID, a.WatchExpiration, a.IsActive, a.NotificationsOn,
		a.LastSyncedAt, a.UpdatedAt,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *GmailAccountRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.GmailAccount, error) {
	const q = `
		SELECT id, user_id, email, google_user_id, access_token_enc, refresh_token_enc,
			token_expiry, history_id, watch_expiration, is_active, notifications_on,
			last_synced_at, created_at, updated_at
		FROM gmail_accounts WHERE id = $1`
	return r.scanOne(ctx, q, id)
}

func (r *GmailAccountRepo) GetByUserAndGoogleID(ctx context.Context, userID uuid.UUID, googleUserID string) (*domain.GmailAccount, error) {
	const q = `
		SELECT id, user_id, email, google_user_id, access_token_enc, refresh_token_enc,
			token_expiry, history_id, watch_expiration, is_active, notifications_on,
			last_synced_at, created_at, updated_at
		FROM gmail_accounts WHERE user_id = $1 AND google_user_id = $2`
	var a domain.GmailAccount
	err := r.pool.QueryRow(ctx, q, userID, googleUserID).Scan(
		&a.ID, &a.UserID, &a.Email, &a.GoogleUserID, &a.AccessTokenEnc, &a.RefreshTokenEnc,
		&a.TokenExpiry, &a.HistoryID, &a.WatchExpiration, &a.IsActive, &a.NotificationsOn,
		&a.LastSyncedAt, &a.CreatedAt, &a.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *GmailAccountRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]*domain.GmailAccount, error) {
	const q = `
		SELECT id, user_id, email, google_user_id, access_token_enc, refresh_token_enc,
			token_expiry, history_id, watch_expiration, is_active, notifications_on,
			last_synced_at, created_at, updated_at
		FROM gmail_accounts WHERE user_id = $1 ORDER BY created_at ASC`
	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAccounts(rows)
}

func (r *GmailAccountRepo) ListActiveWithWatch(ctx context.Context) ([]*domain.GmailAccount, error) {
	const q = `
		SELECT id, user_id, email, google_user_id, access_token_enc, refresh_token_enc,
			token_expiry, history_id, watch_expiration, is_active, notifications_on,
			last_synced_at, created_at, updated_at
		FROM gmail_accounts WHERE is_active = TRUE AND notifications_on = TRUE`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAccounts(rows)
}

func (r *GmailAccountRepo) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	const q = `DELETE FROM gmail_accounts WHERE id = $1 AND user_id = $2`
	tag, err := r.pool.Exec(ctx, q, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *GmailAccountRepo) scanOne(ctx context.Context, q string, arg any) (*domain.GmailAccount, error) {
	var a domain.GmailAccount
	err := r.pool.QueryRow(ctx, q, arg).Scan(
		&a.ID, &a.UserID, &a.Email, &a.GoogleUserID, &a.AccessTokenEnc, &a.RefreshTokenEnc,
		&a.TokenExpiry, &a.HistoryID, &a.WatchExpiration, &a.IsActive, &a.NotificationsOn,
		&a.LastSyncedAt, &a.CreatedAt, &a.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func scanAccounts(rows pgx.Rows) ([]*domain.GmailAccount, error) {
	var list []*domain.GmailAccount
	for rows.Next() {
		var a domain.GmailAccount
		if err := rows.Scan(
			&a.ID, &a.UserID, &a.Email, &a.GoogleUserID, &a.AccessTokenEnc, &a.RefreshTokenEnc,
			&a.TokenExpiry, &a.HistoryID, &a.WatchExpiration, &a.IsActive, &a.NotificationsOn,
			&a.LastSyncedAt, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, &a)
	}
	return list, rows.Err()
}
