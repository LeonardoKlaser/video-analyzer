package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/leoklaser/video-analyzer/api/internal/models"
)

func CreateUser(ctx context.Context, d *DB, email, passwordHash string) (*models.User, error) {
	var u models.User
	var bcRaw []byte
	err := d.QueryRow(ctx, `
		INSERT INTO users (email, password_hash)
		VALUES ($1, $2)
		RETURNING id, email, business_context, created_at, updated_at`,
		email, passwordHash,
	).Scan(&u.ID, &u.Email, &bcRaw, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &u, nil
}

func GetUserByEmail(ctx context.Context, d *DB, email string) (*models.User, string, error) {
	var u models.User
	var passwordHash string
	var bcRaw []byte
	err := d.QueryRow(ctx, `
		SELECT id, email, password_hash, business_context, created_at, updated_at
		FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Email, &passwordHash, &bcRaw, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, "", err
	}
	if len(bcRaw) > 0 {
		u.BusinessContext = &models.BusinessContext{}
		if err := json.Unmarshal(bcRaw, u.BusinessContext); err != nil {
			return nil, "", fmt.Errorf("unmarshal business_context: %w", err)
		}
	}
	return &u, passwordHash, nil
}

func GetUserByID(ctx context.Context, d *DB, id uuid.UUID) (*models.User, error) {
	var u models.User
	var bcRaw []byte
	err := d.QueryRow(ctx, `
		SELECT id, email, business_context, created_at, updated_at
		FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Email, &bcRaw, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if len(bcRaw) > 0 {
		u.BusinessContext = &models.BusinessContext{}
		if err := json.Unmarshal(bcRaw, u.BusinessContext); err != nil {
			return nil, fmt.Errorf("unmarshal business_context: %w", err)
		}
	}
	return &u, nil
}

func UpdateUserBusinessContext(ctx context.Context, d *DB, id uuid.UUID, bc *models.BusinessContext) error {
	bcJSON, err := json.Marshal(bc)
	if err != nil {
		return fmt.Errorf("marshal business_context: %w", err)
	}
	_, err = d.Exec(ctx, `
		UPDATE users SET business_context = $2, updated_at = now() WHERE id = $1`,
		id, bcJSON,
	)
	return err
}
