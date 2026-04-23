package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
)

type Problem struct {
	Status  int    `json:"status"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

func (p Problem) Error() string { return p.Code + ": " + p.Message }

var (
	ErrUnauthorized     = Problem{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "Authentication required"}
	ErrForbidden        = Problem{Status: http.StatusForbidden, Code: "forbidden", Message: "Access denied"}
	ErrNotFound         = Problem{Status: http.StatusNotFound, Code: "not_found", Message: "Resource not found"}
	ErrBadRequest       = Problem{Status: http.StatusBadRequest, Code: "bad_request", Message: "Invalid request"}
	ErrConflict         = Problem{Status: http.StatusConflict, Code: "conflict", Message: "Resource conflict"}
	ErrRateLimited      = Problem{Status: http.StatusTooManyRequests, Code: "rate_limited", Message: "Too many requests"}
	ErrInternal         = Problem{Status: http.StatusInternalServerError, Code: "internal", Message: "Internal server error"}
	ErrPaymentsDisabled = Problem{Status: http.StatusForbidden, Code: "payments_disabled", Message: "Money-in payments are disabled. Use a promo code."}
)

func WithDetail(p Problem, detail string) Problem {
	p.Detail = detail
	return p
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func WriteProblem(w http.ResponseWriter, err error) {
	var p Problem
	if errors.As(err, &p) {
		WriteJSON(w, p.Status, p)
		return
	}
	WriteJSON(w, http.StatusInternalServerError, WithDetail(ErrInternal, err.Error()))
}
