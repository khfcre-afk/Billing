package servers

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
	"github.com/khfcre-afk/pterodactyl-billing/backend/internal/pterodactyl"
)

type Server struct {
	ID              uuid.UUID `json:"id"`
	UserID          uuid.UUID `json:"user_id"`
	TariffID        uuid.UUID `json:"tariff_id"`
	Name            string    `json:"name"`
	Status          string    `json:"status"`
	PaidUntil       time.Time `json:"paid_until"`
	AllocationIP    *string   `json:"allocation_ip,omitempty"`
	AllocationPort  *int      `json:"allocation_port,omitempty"`
	PteroIdentifier *string   `json:"ptero_identifier,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

type Repo struct{ db *pgxpool.Pool }

func NewRepo(db *pgxpool.Pool) *Repo { return &Repo{db: db} }

func (r *Repo) ListByUser(ctx context.Context, userID uuid.UUID) ([]Server, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, tariff_id, name, status::text, paid_until,
		       allocation_ip, allocation_port, ptero_identifier, created_at
		FROM servers WHERE user_id = $1 AND status <> 'deleted' ORDER BY created_at DESC`,
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Server{}
	for rows.Next() {
		var s Server
		if err := rows.Scan(&s.ID, &s.UserID, &s.TariffID, &s.Name, &s.Status, &s.PaidUntil,
			&s.AllocationIP, &s.AllocationPort, &s.PteroIdentifier, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *Repo) Get(ctx context.Context, id uuid.UUID) (*Server, error) {
	var s Server
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, tariff_id, name, status::text, paid_until,
		       allocation_ip, allocation_port, ptero_identifier, created_at
		FROM servers WHERE id = $1`, id,
	).Scan(&s.ID, &s.UserID, &s.TariffID, &s.Name, &s.Status, &s.PaidUntil,
		&s.AllocationIP, &s.AllocationPort, &s.PteroIdentifier, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	return &s, err
}

type Handler struct {
	repo  *Repo
	ptero *pterodactyl.Client
}

func NewHandler(repo *Repo, ptero *pterodactyl.Client) *Handler {
	return &Handler{repo: repo, ptero: ptero}
}

func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Get("/{id}", h.get)
	r.Post("/{id}/power", h.power)
	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	uid, err := currentUser(r)
	if err != nil {
		httpx.WriteProblem(w, err)
		return
	}
	list, err := h.repo.ListByUser(r.Context(), uid)
	if err != nil {
		httpx.WriteProblem(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	s, uid, err := h.fetchOwned(r)
	if err != nil {
		httpx.WriteProblem(w, err)
		return
	}
	if s.UserID != uid {
		httpx.WriteProblem(w, httpx.ErrForbidden)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, s)
}

type powerIn struct {
	Signal string `json:"signal" validate:"required,oneof=start stop restart kill"`
}

func (h *Handler) power(w http.ResponseWriter, r *http.Request) {
	var in powerIn
	if err := httpx.DecodeAndValidate(r, &in); err != nil {
		httpx.WriteProblem(w, err)
		return
	}
	s, uid, err := h.fetchOwned(r)
	if err != nil {
		httpx.WriteProblem(w, err)
		return
	}
	if s.UserID != uid {
		httpx.WriteProblem(w, httpx.ErrForbidden)
		return
	}
	if s.PteroIdentifier == nil {
		httpx.WriteProblem(w, httpx.WithDetail(httpx.ErrBadRequest, "server is not provisioned yet"))
		return
	}
	if s.Status != "active" {
		httpx.WriteProblem(w, httpx.WithDetail(httpx.ErrBadRequest, "server is not active"))
		return
	}
	if err := h.ptero.Power(r.Context(), *s.PteroIdentifier, pterodactyl.PowerAction(in.Signal)); err != nil {
		httpx.WriteProblem(w, httpx.WithDetail(httpx.ErrInternal, err.Error()))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) fetchOwned(r *http.Request) (*Server, uuid.UUID, error) {
	uid, err := currentUser(r)
	if err != nil {
		return nil, uuid.Nil, err
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return nil, uuid.Nil, httpx.ErrBadRequest
	}
	s, err := h.repo.Get(r.Context(), id)
	if err != nil {
		return nil, uuid.Nil, err
	}
	return s, uid, nil
}

func currentUser(r *http.Request) (uuid.UUID, error) {
	idStr, _ := httpx.UserID(r.Context())
	uid, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil, httpx.ErrUnauthorized
	}
	return uid, nil
}
