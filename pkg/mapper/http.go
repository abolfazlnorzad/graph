package mapper

import (
	"errors"
	"net/http"

	"github.com/abolfazlnorzad/graph/pkg/richerror"
)

func MapToStatusCode(err error) int {
	var re richerror.RichError

	if !errors.As(err, &re) {
		return http.StatusInternalServerError
	}

	switch re.Kind() {
	case richerror.KindConflict:
		return http.StatusConflict // 409
	case richerror.KindInvalid:
		return http.StatusBadRequest // 400
	case richerror.KindNotFound:
		return http.StatusNotFound // 404
	case richerror.KindForbidden:
		return http.StatusForbidden // 403
	case richerror.KindUnauthorized:
		return http.StatusUnauthorized // 401
	case richerror.KindUnexpected:
		return http.StatusInternalServerError // 500
	default:
		return http.StatusInternalServerError // Fallback
	}
}
