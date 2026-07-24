package validation

import (
	"errors"
	"fmt"

	"github.com/abolfazlnorzad/graph/entity"
	"github.com/abolfazlnorzad/graph/param"
	"github.com/abolfazlnorzad/graph/pkg/msg"
	"github.com/abolfazlnorzad/graph/pkg/richerror"
	validation "github.com/go-ozzo/ozzo-validation/v4"
)

type Validator struct{}

func NewValidator() Validator {
	return Validator{}
}

func validTaskStatus() validation.Rule {
	return validation.In(
		entity.StatusTodo,
		entity.StatusInProgress,
		entity.StatusReview,
		entity.StatusDone,
		entity.StatusRejected,
		entity.StatusBlocked,
		entity.StatusCanceled,
	).Error(string(msg.ErrInvalidInput))
}

func (v Validator) ValidateCreateTask(req param.CreateTaskRequest) error {
	const op = "service.ValidateCreateTask"

	err := validation.ValidateStruct(&req,
		validation.Field(&req.Title,
			validation.Required.Error(string(msg.ErrFieldIsRequired)),
			validation.Length(1, 255).Error(string(msg.ErrInvalidInput)),
		),
		validation.Field(&req.Description,
			validation.When(req.Description != nil,
				validation.Length(0, 1000).Error(string(msg.ErrInvalidInput)),
			),
		),
		validation.Field(&req.Status,
			validation.Required.Error(string(msg.ErrFieldIsRequired)),
			validTaskStatus(),
		),
		validation.Field(&req.Assignee,
			validation.When(req.Assignee != nil,
				validation.Length(1, 100).Error(string(msg.ErrInvalidInput)),
			),
		),
	)

	return v.wrapValidationErrors(op, err)
}

func (v Validator) ValidateUpdateTask(req param.UpdateTaskRequest) error {
	const op = "service.ValidateUpdateTask"

	err := validation.ValidateStruct(&req,
		validation.Field(&req.ID,
			validation.Required.Error(string(msg.ErrFieldIsRequired)),
			validation.By(func(value interface{}) error {
				id, ok := value.(entity.ID)
				if !ok || id == 0 {
					return fmt.Errorf(string(msg.ErrInvalidInput))
				}
				return nil
			}),
		),
		validation.Field(&req.Version,
			validation.Required.Error(string(msg.ErrFieldIsRequired)),
			validation.By(func(value interface{}) error {
				v, ok := value.(int16)
				if !ok || v < 1 {
					return fmt.Errorf(string(msg.ErrInvalidInput))
				}
				return nil
			}),
		),
		validation.Field(&req.Title,
			validation.When(req.Title != nil,
				validation.Required.Error(string(msg.ErrFieldIsRequired)),
				validation.Length(1, 255).Error(string(msg.ErrInvalidInput)),
			),
		),
		validation.Field(&req.Description,
			validation.When(req.Description != nil,
				validation.Length(0, 1000).Error(string(msg.ErrInvalidInput)),
			),
		),
		validation.Field(&req.Status,
			validation.When(req.Status != nil,
				validation.Required.Error(string(msg.ErrFieldIsRequired)),
				validTaskStatus(),
			),
		),
		validation.Field(&req.Assignee,
			validation.When(req.Assignee != nil,
				validation.Length(1, 100).Error(string(msg.ErrInvalidInput)),
			),
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
