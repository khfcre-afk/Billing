// Package promo implements redemption of promo codes. Activation is race-safe:
// each redemption opens a transaction, re-checks availability under row locks,
// and inserts into promo_redemptions whose (promo_id,user_id) UNIQUE index
// guarantees a user cannot activate the same code twice, even under concurrency.
package promo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khfcre-afk/pterodactyl-billing/backend/internal/httpx"
)

type Kind string

const (
	KindBalance     Kind = "balance"
	KindTariffGrant Kind = "tariff_grant"
	KindPercentOff  Kind = "percent_off"
)

type Code struct {
	ID            uuid.UUID  `json:"id"`
	Code          string     `json:"code"`
	Kind          Kind       `json:"kind"`
	BalanceCents  *int64     `json:"balance_cents,omitempty"`
	TariffID      *uuid.UUID `json:"tariff_id,omitempty"`
	GrantDays     *int       `json:"grant_days,omitempty"`
	PercentOff    *int       `json:"percent_off,omitempty"`
	MaxUses       *int       `json:"max_uses,omitempty"`
	UsesCount     int        `json:"uses_count"`
	PerUserLimit  int        `json:"per_user_limit"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	IsActive      bool       `json:"is_active"`
	CreatedAt     time.Time  `json:"created_at"`
}

type RedemptionResult struct {
	Kind          Kind   `json:"kind"`
	Message       string `json:"message"`
	BalanceCents  int64  `json:"balance_cents"`
	GrantedServer *uuid.UUID `json:"granted_server_id,omitempty"`
}

type Service struct {
	db          *pgxpool.Pool
	provisioner ServerProvisioner
}

// ServerProvisioner is implemented by the tariffs package to avoid import cycles.
type ServerProvisioner interface {
	ProvisionFromGrant(ctx context.Context, tx pgx.Tx, userID, tariffID uuid.UUID, grantDays int, promoID uuid.UUID) (uuid.UUID, error)
}

func NewService(db *pgxpool.Pool, p ServerProvisioner) *Service {
	return &Service{db: db, provisioner: p}
}

var (
	ErrCodeNotFound   = httpx.WithDetail(httpx.ErrNotFound, "promo code not found")
	ErrCodeInactive   = httpx.WithDetail(httpx.ErrBadRequest, "promo code is inactive")
	ErrCodeExpired    = httpx.WithDetail(httpx.ErrBadRequest, "promo code is expired")
	ErrCodeExhausted  = httpx.WithDetail(httpx.ErrBadRequest, "promo code is fully redeemed")
	ErrAlreadyUsed    = httpx.WithDetail(httpx.ErrConflict, "you already activated this code")
)

func (s *Service) Redeem(ctx context.Context, userID uuid.UUID, rawCode string) (*RedemptionResult, error) {
	if rawCode == "" {
		return nil, httpx.WithDetail(httpx.ErrBadRequest, "code is required")
	}

	var result *RedemptionResult
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		var c Code
		row := tx.QueryRow(ctx, `
			SELECT id, code, kind, balance_cents, tariff_id, grant_days, percent_off,
			       max_uses, uses_count, per_user_limit, expires_at, is_active, created_at
			FROM promo_codes WHERE code = $1 FOR UPDATE`, rawCode)
		err := row.Scan(&c.ID, &c.Code, &c.Kind, &c.BalanceCents, &c.TariffID, &c.GrantDays, &c.PercentOff,
			&c.MaxUses, &c.UsesCount, &c.PerUserLimit, &c.ExpiresAt, &c.IsActive, &c.CreatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrCodeNotFound
		}
		if err != nil {
			return err
		}
		if !c.IsActive {
			return ErrCodeInactive
		}
		if c.ExpiresAt != nil && time.Now().After(*c.ExpiresAt) {
			return ErrCodeExpired
		}
		if c.MaxUses != nil && c.UsesCount >= *c.MaxUses {
			return ErrCodeExhausted
		}

		if _, err := tx.Exec(ctx,
			`INSERT INTO promo_redemptions (promo_id, user_id) VALUES ($1, $2)`,
			c.ID, userID,
		); err != nil {
			if errContains(err.Error(), "duplicate key") {
				return ErrAlreadyUsed
			}
			return err
		}

		if _, err := tx.Exec(ctx,
			`UPDATE promo_codes SET uses_count = uses_count + 1 WHERE id = $1`, c.ID,
		); err != nil {
			return err
		}

		switch c.Kind {
		case KindBalance:
			if c.BalanceCents == nil {
				return errors.New("promo balance misconfigured")
			}
			var newBal int64
			if err := tx.QueryRow(ctx,
				`UPDATE users SET balance_cents = balance_cents + $2, updated_at = now()
				 WHERE id = $1 RETURNING balance_cents`,
				userID, *c.BalanceCents,
			).Scan(&newBal); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO transactions (user_id, kind, amount_cents, balance_after, description, promo_id)
				VALUES ($1, 'promo_deposit', $2, $3, $4, $5)`,
				userID, *c.BalanceCents, newBal, "Promo deposit: "+c.Code, c.ID,
			); err != nil {
				return err
			}
			result = &RedemptionResult{Kind: c.Kind, Message: "Balance credited", BalanceCents: newBal}

		case KindTariffGrant:
			if c.TariffID == nil || c.GrantDays == nil {
				return errors.New("promo grant misconfigured")
			}
			serverID, err := s.provisioner.ProvisionFromGrant(ctx, tx, userID, *c.TariffID, *c.GrantDays, c.ID)
			if err != nil {
				return err
			}
			result = &RedemptionResult{Kind: c.Kind, Message: "Server granted", GrantedServer: &serverID}

		case KindPercentOff:
			result = &RedemptionResult{Kind: c.Kind, Message: "Discount code stored; apply at purchase"}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func errContains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
