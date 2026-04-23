package promo

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khfcre-afk/pterodactyl-billing/backend/internal/httpx"
)

type AdminRepo struct{ db *pgxpool.Pool }

func NewAdminRepo(db *pgxpool.Pool) *AdminRepo { return &AdminRepo{db: db} }

type CreateInput struct {
	Code         string     `json:"code" validate:"required,min=3,max=64"`
	Kind         Kind       `json:"kind" validate:"required,oneof=balance tariff_grant percent_off"`
	BalanceCents *int64     `json:"balance_cents"`
	TariffID     *uuid.UUID `json:"tariff_id"`
	GrantDays    *int       `json:"grant_days"`
	PercentOff   *int       `json:"percent_off"`
	MaxUses      *int       `json:"max_uses"`
	PerUserLimit int        `json:"per_user_limit"`
	ExpiresAt    *time.Time `json:"expires_at"`
}

func (r *AdminRepo) Create(ctx context.Context, in CreateInput) (*Code, error) {
	if in.PerUserLimit <= 0 {
		in.PerUserLimit = 1
	}
	var id uuid.UUID
	err := r.db.QueryRow(ctx, `
		INSERT INTO promo_codes (code, kind, balance_cents, tariff_id, grant_days, percent_off,
		                         max_uses, per_user_limit, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`,
		in.Code, in.Kind, in.BalanceCents, in.TariffID, in.GrantDays, in.PercentOff,
		in.MaxUses, in.PerUserLimit, in.ExpiresAt,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, id)
}

func (r *AdminRepo) Get(ctx context.Context, id uuid.UUID) (*Code, error) {
	var c Code
	err := r.db.QueryRow(ctx, `
		SELECT id, code, kind, balance_cents, tariff_id, grant_days, percent_off,
		       max_uses, uses_count, per_user_limit, expires_at, is_active, created_at
		FROM promo_codes WHERE id = $1`, id,
	).Scan(&c.ID, &c.Code, &c.Kind, &c.BalanceCents, &c.TariffID, &c.GrantDays, &c.PercentOff,
		&c.MaxUses, &c.UsesCount, &c.PerUserLimit, &c.ExpiresAt, &c.IsActive, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *AdminRepo) List(ctx context.Context) ([]Code, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, code, kind, balance_cents, tariff_id, grant_days, percent_off,
		       max_uses, uses_count, per_user_limit, expires_at, is_active, created_at
		FROM promo_codes ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Code{}
	for rows.Next() {
		var c Code
		if err := rows.Scan(&c.ID, &c.Code, &c.Kind, &c.BalanceCents, &c.TariffID, &c.GrantDays, &c.PercentOff,
			&c.MaxUses, &c.UsesCount, &c.PerUserLimit, &c.ExpiresAt, &c.IsActive, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *AdminRepo) SetActive(ctx context.Context, id uuid.UUID, active bool) error {
	_, err := r.db.Exec(ctx, `UPDATE promo_codes SET is_active = $2 WHERE id = $1`, id, active)
	return err
}

type AdminHandler struct{ repo *AdminRepo }

func NewAdminHandler(repo *AdminRepo) *AdminHandler { return &AdminHandler{repo: repo} }

func (h *AdminHandler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Patch("/{id}/activate", func(w http.ResponseWriter, r *http.Request) { h.setActive(w, r, true) })
	r.Patch("/{id}/deactivate", func(w http.ResponseWriter, r *http.Request) { h.setActive(w, r, false) })
	return r
}

func (h *AdminHandler) list(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.List(r.Context())
	if err != nil {
		httpx.WriteProblem(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

func (h *AdminHandler) create(w http.ResponseWriter, r *http.Request) {
	var in CreateInput
	if err := httpx.DecodeAndValidate(r, &in); err != nil {
		httpx.WriteProblem(w, err)
		return
	}
	c, err := h.repo.Create(r.Context(), in)
	if err != nil {
		httpx.WriteProblem(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, c)
}

func (h *AdminHandler) setActive(w http.ResponseWriter, r *http.Request, active bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteProblem(w, httpx.ErrBadRequest)
		return
	}
	if err := h.repo.SetActive(r.Context(), id, active); err != nil {
		httpx.WriteProblem(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
