package promo

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/khfcre-afk/pterodactyl-billing/backend/internal/httpx"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Post("/redeem", h.redeem)
	return r
}

type redeemIn struct {
	Code string `json:"code" validate:"required,min=3,max=64"`
}

func (h *Handler) redeem(w http.ResponseWriter, r *http.Request) {
	var in redeemIn
	if err := httpx.DecodeAndValidate(r, &in); err != nil {
		httpx.WriteProblem(w, err)
		return
	}
	uidStr, _ := httpx.UserID(r.Context())
	uid, err := uuid.Parse(uidStr)
	if err != nil {
		httpx.WriteProblem(w, httpx.ErrUnauthorized)
		return
	}
	res, err := h.svc.Redeem(r.Context(), uid, in.Code)
	if err != nil {
		httpx.WriteProblem(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}
