package tariffs

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/khfcre-afk/pterodactyl-billing/backend/internal/httpx"
)

type Handler struct {
	repo *Repo
	purchaser *PurchaseService
}

func NewHandler(repo *Repo, p *PurchaseService) *Handler {
	return &Handler{repo: repo, purchaser: p}
}

// List is the public list endpoint exposed at GET /api/tariffs.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) { h.list(w, r) }

// Purchase is the authed purchase endpoint exposed at POST /api/tariffs/{id}/purchase.
func (h *Handler) Purchase(w http.ResponseWriter, r *http.Request) { h.purchase(w, r) }

func (h *Handler) AdminRoutes() http.Handler {
	r := chi.NewRouter()
	r.Post("/", h.create)
	r.Patch("/{id}/activate", func(w http.ResponseWriter, r *http.Request) { h.setActive(w, r, true) })
	r.Patch("/{id}/deactivate", func(w http.ResponseWriter, r *http.Request) { h.setActive(w, r, false) })
	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.List(r.Context(), true)
	if err != nil {
		httpx.WriteProblem(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in CreateInput
	if err := httpx.DecodeAndValidate(r, &in); err != nil {
		httpx.WriteProblem(w, err)
		return
	}
	t, err := h.repo.Create(r.Context(), in)
	if err != nil {
		httpx.WriteProblem(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, t)
}

func (h *Handler) setActive(w http.ResponseWriter, r *http.Request, active bool) {
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

func (h *Handler) purchase(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteProblem(w, httpx.ErrBadRequest)
		return
	}
	uidStr, _ := httpx.UserID(r.Context())
	uid, err := uuid.Parse(uidStr)
	if err != nil {
		httpx.WriteProblem(w, httpx.ErrUnauthorized)
		return
	}
	srvID, err := h.purchaser.Purchase(r.Context(), uid, id)
	if err != nil {
		httpx.WriteProblem(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"server_id": srvID.String()})
}
