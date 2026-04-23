package billing

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khfcre-afk/pterodactyl-billing/backend/internal/httpx"
)

type Transaction struct {
	ID           uuid.UUID  `json:"id"`
	Kind         string     `json:"kind"`
	AmountCents  int64      `json:"amount_cents"`
	BalanceAfter int64      `json:"balance_after"`
	Description  string     `json:"description"`
	CreatedAt    time.Time  `json:"created_at"`
	TariffID     *uuid.UUID `json:"tariff_id,omitempty"`
	ServerID     *uuid.UUID `json:"server_id,omitempty"`
}

type Repo struct{ db *pgxpool.Pool }

func NewRepo(db *pgxpool.Pool) *Repo { return &Repo{db: db} }

func (r *Repo) ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]Transaction, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, kind::text, amount_cents, balance_after, description, created_at, tariff_id, server_id
		FROM transactions WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Transaction{}
	for rows.Next() {
		var t Transaction
		if err := rows.Scan(&t.ID, &t.Kind, &t.AmountCents, &t.BalanceAfter, &t.Description, &t.CreatedAt, &t.TariffID, &t.ServerID); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

type Handler struct{ repo *Repo }

func NewHandler(repo *Repo) *Handler { return &Handler{repo: repo} }

func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/", h.list)
	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	idStr, _ := httpx.UserID(r.Context())
	uid, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteProblem(w, httpx.ErrUnauthorized)
		return
	}
	limit := 50
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil {
			limit = n
		}
	}
	items, err := h.repo.ListByUser(r.Context(), uid, limit)
	if err != nil {
		httpx.WriteProblem(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, items)
}
