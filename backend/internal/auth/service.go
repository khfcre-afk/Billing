package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/khfcre-afk/pterodactyl-billing/backend/internal/httpx"
)

type Service struct {
	db     *pgxpool.Pool
	issuer *Issuer
}

func NewService(db *pgxpool.Pool, issuer *Issuer) *Service {
	return &Service{db: db, issuer: issuer}
}

type RegisterInput struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8,max=128"`
}

type LoginInput struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	UserID       string    `json:"user_id"`
	Role         string    `json:"role"`
}

func (s *Service) Register(ctx context.Context, in RegisterInput) (*TokenPair, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	var pair *TokenPair
	err = pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		var id uuid.UUID
		if err := tx.QueryRow(ctx,
			`INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`,
			in.Email, string(hash),
		).Scan(&id); err != nil {
			if isUniqueViolation(err) {
				return httpx.WithDetail(httpx.ErrConflict, "email already registered")
			}
			return err
		}
		p, err := s.issueTx(ctx, tx, id.String(), "user")
		if err != nil {
			return err
		}
		pair = p
		return nil
	})
	if err != nil {
		return nil, err
	}
	return pair, nil
}

func (s *Service) Login(ctx context.Context, in LoginInput) (*TokenPair, error) {
	var pair *TokenPair
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		var (
			id   uuid.UUID
			hash string
			role string
		)
		if err := tx.QueryRow(ctx,
			`SELECT id, password_hash, role FROM users WHERE email = $1`, in.Email,
		).Scan(&id, &hash, &role); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httpx.WithDetail(httpx.ErrUnauthorized, "invalid credentials")
			}
			return err
		}
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) != nil {
			return httpx.WithDetail(httpx.ErrUnauthorized, "invalid credentials")
		}
		p, err := s.issueTx(ctx, tx, id.String(), role)
		if err != nil {
			return err
		}
		pair = p
		return nil
	})
	if err != nil {
		return nil, err
	}
	return pair, nil
}

// Refresh rotates a refresh token atomically: it locks the existing token row,
// verifies it is still valid, revokes it and issues a new pair inside one
// transaction to prevent concurrent replay of the same refresh token.
func (s *Service) Refresh(ctx context.Context, refresh string) (*TokenPair, error) {
	hash := hashToken(refresh)
	var pair *TokenPair
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		var (
			id        uuid.UUID
			userID    uuid.UUID
			expiresAt time.Time
			revokedAt *time.Time
			role      string
		)
		if err := tx.QueryRow(ctx, `
			SELECT rt.id, rt.user_id, rt.expires_at, rt.revoked_at, u.role
			FROM refresh_tokens rt JOIN users u ON u.id = rt.user_id
			WHERE rt.token_hash = $1
			FOR UPDATE OF rt`, hash,
		).Scan(&id, &userID, &expiresAt, &revokedAt, &role); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httpx.ErrUnauthorized
			}
			return err
		}
		if revokedAt != nil || time.Now().After(expiresAt) {
			return httpx.ErrUnauthorized
		}
		if _, err := tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE id = $1`, id); err != nil {
			return err
		}
		p, err := s.issueTx(ctx, tx, userID.String(), role)
		if err != nil {
			return err
		}
		pair = p
		return nil
	})
	if err != nil {
		return nil, err
	}
	return pair, nil
}

func (s *Service) Logout(ctx context.Context, refresh string) error {
	_, err := s.db.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL`, hashToken(refresh))
	return err
}

func (s *Service) issueTx(ctx context.Context, tx pgx.Tx, userID, role string) (*TokenPair, error) {
	access, exp, err := s.issuer.Access(userID, role)
	if err != nil {
		return nil, err
	}
	refresh, err := randomToken()
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		userID, hashToken(refresh), time.Now().Add(s.issuer.RefreshTTL()),
	); err != nil {
		return nil, err
	}
	return &TokenPair{AccessToken: access, RefreshToken: refresh, ExpiresAt: exp, UserID: userID, Role: role}, nil
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashToken(t string) string {
	h := sha256.Sum256([]byte(t))
	return hex.EncodeToString(h[:])
}

func isUniqueViolation(err error) bool {
	return err != nil && errContains(err.Error(), "duplicate key")
}

func errContains(s, sub string) bool {
	return len(s) >= len(sub) && (indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
