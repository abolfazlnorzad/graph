package validation

import (
	"errors"

	"github.com/abolfazlnorzad/graph/entity"
	"github.com/abolfazlnorzad/graph/param"
	"github.com/abolfazlnorzad/graph/pkg/msg"
	"github.com/abolfazlnorzad/graph/pkg/richerror"
	validation "github.com/go-ozzo/ozzo-validation/v4"
)

type Validator struct {
}

func NewValidator() Validator {
	return Validator{}
}

func (v Validator) ValidateCreateTask(req param.CreateTaskRequest) error {
	const op = "service.ValidateCreateTask"

	err := validation.ValidateStruct(&req,
		validation.Field(&req.Title,
			validation.Required.Error(string(msg.ErrFieldIsRequired)),
			validation.Length(1, 255).Error(string(msg.ErrInvalidInput)),
		),
		validation.Field(&req.Status,
			validation.Required.Error(string(msg.ErrFieldIsRequired)),
			validation.In(
				entity.StatusTodo,
				entity.StatusInProgress,
				entity.StatusReview,
				entity.StatusDone,
				entity.StatusRejected,
				entity.StatusBlocked,
				entity.StatusCanceled,
			).Error(string(msg.ErrInvalidInput)),
		),
	)

	return v.wrapValidationErrors(op, err)
}

func (v Validator) wrapValidationErrors(op string, err error) error {
	if err == nil {
		return nil
	}

	var vErrs validation.Errors
	if errors.As(err, &vErrs) {
		validationErrors := make(map[string]string)
		for key, value := range vErrs {
			validationErrors[key] = value.Error()
		}

		return richerror.New(richerror.Op(op)).
			WithKind(richerror.KindInvalid).
			WithUserMsgKey(msg.ErrValidationFailed).
			WithMeta(map[string]any{"fields": validationErrors})
	}

	return richerror.New(richerror.Op(op)).WithErr(err).WithKind(richerror.KindUnexpected)
}
