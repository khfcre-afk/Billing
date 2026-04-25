package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New(validator.WithRequiredStructEnabled())

func DecodeAndValidate(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return WithDetail(ErrBadRequest, "invalid json: "+err.Error())
	}
	if err := validate.Struct(dst); err != nil {
		var verrs validator.ValidationErrors
		if errors.As(err, &verrs) {
			return WithDetail(ErrBadRequest, verrs.Error())
		}
		return WithDetail(ErrBadRequest, err.Error())
	}
	return nil
}
