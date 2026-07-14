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

type UserRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

func (r *UserRepo) Create(ctx context.Context, user *domain.User) error {
	const q = `
		INSERT INTO users (id, email, display_name, password_hash, dark_mode, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	now := time.Now().UTC()
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	user.CreatedAt = now
	user.UpdatedAt = now
	_, err := r.pool.Exec(ctx, q, user.ID, user.Email, user.DisplayName, user.PasswordHash, user.DarkMode, user.CreatedAt, user.UpdatedAt)
	if err != nil {
		return err
	}
	return nil
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	const q = `
		SELECT id, email, display_name, password_hash, dark_mode, created_at, updated_at
		FROM users WHERE id = $1`
	return r.scanOne(ctx, q, id)
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	const q = `
		SELECT id, email, display_name, password_hash, dark_mode, created_at, updated_at
		FROM users WHERE email = $1`
	return r.scanOne(ctx, q, email)
}

func (r *UserRepo) Update(ctx context.Context, user *domain.User) error {
	const q = `
		UPDATE users SET display_name = $2, dark_mode = $3, updated_at = $4
		WHERE id = $1`
	user.UpdatedAt = time.Now().UTC()
	tag, err := r.pool.Exec(ctx, q, user.ID, user.DisplayName, user.DarkMode, user.UpdatedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *UserRepo) scanOne(ctx context.Context, q string, arg any) (*domain.User, error) {
	var u domain.User
	err := r.pool.QueryRow(ctx, q, arg).Scan(
		&u.ID, &u.Email, &u.DisplayName, &u.PasswordHash, &u.DarkMode, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}
