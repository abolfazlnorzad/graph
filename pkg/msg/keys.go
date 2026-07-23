package msg

type Key string

const (
	ErrUnexpected       Key = "error.unexpected"
	ErrInvalidInput     Key = "error.invalid_input"
	ErrForbidden        Key = "error.forbidden"
	ErrNotFound         Key = "error.not_found"
	ErrUnauthorized     Key = "error.unauthorized"
	ErrFieldIsRequired  Key = "error.field_is_required"
	ErrValidationFailed     = "error.validation_failed"
)
