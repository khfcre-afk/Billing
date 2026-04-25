package users

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khfcre-afk/pterodactyl-billing/backend/internal/httpx"
)

type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	Role         string    `json:"role"`
	BalanceCents int64     `json:"balance_cents"`
	CreatedAt    time.Time `json:"created_at"`
}

type Repo struct{ db *pgxpool.Pool }

func NewRepo(db *pgxpool.Pool) *Repo { return &Repo{db: db} }

func (r *Repo) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	var u User
	err := r.db.QueryRow(ctx,
		`SELECT id, email, role, balance_cents, created_at FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Email, &u.Role, &u.BalanceCents, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	return &u, err
}

func (r *Repo) SetPteroUserID(ctx context.Context, id uuid.UUID, pteroID int) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET ptero_user_id = $2, updated_at = now() WHERE id = $1`, id, pteroID)
	return err
}

type Handler struct{ repo *Repo }

func NewHandler(repo *Repo) *Handler { return &Handler{repo: repo} }

func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/me", h.me)
	return r
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	idStr, _ := httpx.UserID(r.Context())
	id, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteProblem(w, httpx.ErrUnauthorized)
		return
	}
	u, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		httpx.WriteProblem(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, u)
}
