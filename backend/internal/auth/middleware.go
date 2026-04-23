package auth

import (
	"net/http"
	"strings"

	"github.com/khfcre-afk/pterodactyl-billing/backend/internal/httpx"
)

func Middleware(issuer *Issuer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := bearer(r)
			if token == "" {
				if ck, err := r.Cookie("access_token"); err == nil {
					token = ck.Value
				}
			}
			if token == "" {
				httpx.WriteProblem(w, httpx.ErrUnauthorized)
				return
			}
			claims, err := issuer.Parse(token)
			if err != nil {
				httpx.WriteProblem(w, httpx.WithDetail(httpx.ErrUnauthorized, err.Error()))
				return
			}
			ctx := httpx.WithUser(r.Context(), claims.UserID, claims.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if httpx.Role(r.Context()) != "admin" {
			httpx.WriteProblem(w, httpx.ErrForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return ""
}
