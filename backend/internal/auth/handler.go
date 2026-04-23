package auth

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/khfcre-afk/pterodactyl-billing/backend/internal/httpx"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Post("/register", h.register)
	r.Post("/login", h.login)
	r.Post("/refresh", h.refresh)
	r.Post("/logout", h.logout)
	return r
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var in RegisterInput
	if err := httpx.DecodeAndValidate(r, &in); err != nil {
		httpx.WriteProblem(w, err)
		return
	}
	pair, err := h.svc.Register(r.Context(), in)
	if err != nil {
		httpx.WriteProblem(w, err)
		return
	}
	h.setAuthCookies(w, pair)
	httpx.WriteJSON(w, http.StatusCreated, pair)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var in LoginInput
	if err := httpx.DecodeAndValidate(r, &in); err != nil {
		httpx.WriteProblem(w, err)
		return
	}
	pair, err := h.svc.Login(r.Context(), in)
	if err != nil {
		httpx.WriteProblem(w, err)
		return
	}
	h.setAuthCookies(w, pair)
	httpx.WriteJSON(w, http.StatusOK, pair)
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	token := refreshFrom(r)
	if token == "" {
		httpx.WriteProblem(w, httpx.ErrUnauthorized)
		return
	}
	pair, err := h.svc.Refresh(r.Context(), token)
	if err != nil {
		httpx.WriteProblem(w, err)
		return
	}
	h.setAuthCookies(w, pair)
	httpx.WriteJSON(w, http.StatusOK, pair)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if token := refreshFrom(r); token != "" {
		_ = h.svc.Logout(r.Context(), token)
	}
	h.clearCookies(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) setAuthCookies(w http.ResponseWriter, p *TokenPair) {
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    p.AccessToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  p.ExpiresAt,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    p.RefreshToken,
		Path:     "/api/auth",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(30 * 24 * time.Hour),
	})
}

func (h *Handler) clearCookies(w http.ResponseWriter) {
	for _, name := range []string{"access_token", "refresh_token"} {
		http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", HttpOnly: true, MaxAge: -1})
	}
}

func refreshFrom(r *http.Request) string {
	if ck, err := r.Cookie("refresh_token"); err == nil {
		return ck.Value
	}
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = httpx.DecodeAndValidate(r, &body)
	return body.RefreshToken
}
