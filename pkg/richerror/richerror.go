package richerror

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/abolfazlnorzad/graph/pkg/msg"
)

type Kind int

var ErrAlreadyRefunded = errors.New("execution already refunded")

const (
	KindInvalid      Kind = iota + 1 // (400)
	KindForbidden                    //   (403)
	KindNotFound                     //  (404)
	KindUnexpected                   //   (500)
	KindUnauthorized                 //   (401)
)

func (k Kind) String() string {
	switch k {
	case KindInvalid:
		return "Invalid"
	case KindForbidden:
		return "Forbidden"
	case KindNotFound:
		return "NotFound"
	case KindUnexpected:
		return "Unexpected"
	case KindUnauthorized:
		return "Unauthorized"
	default:
		return "Unknown"
	}
}

type Op string

type RichError struct {
	op           Op
	wrappedError error
	kind         Kind
	message      string
	userMsgKey   msg.Key
	meta         map[string]any
}

func (e RichError) Error() string {
	if e.wrappedError != nil {
		return fmt.Sprintf("%s: %s -> %v", e.op, e.message, e.wrappedError)
	}

	return fmt.Sprintf("%s: %s", e.op, e.message)
}

func New(op Op) RichError {
	_, file, line, _ := runtime.Caller(1)
	return RichError{op: op, meta: map[string]any{
		"file": file,
		"line": line,
	}}
}

func (e RichError) WithMessage(message string) RichError {
	e.message = message
	return e
}

func (e RichError) WithUserMsgKey(key msg.Key) RichError {
	e.userMsgKey = key
	return e
}

func (e RichError) WithKind(kind Kind) RichError {
	e.kind = kind
	return e
}

func (e RichError) WithErr(err error) RichError {
	e.wrappedError = err
	return e
}

func (e RichError) WithMeta(m map[string]any) RichError {
	e.meta = m
	return e
}

func (e RichError) Kind() Kind {
	if e.kind != 0 {
		return e.kind
	}
	var re RichError
	if errors.As(e.wrappedError, &re) {
		return re.Kind()
	}
	return KindUnexpected
}

func (e RichError) Message() string {
	return e.message
}

func (e RichError) UserMsgKey() msg.Key {
	if e.userMsgKey != "" {
		return e.userMsgKey
	}
	var re RichError
	if errors.As(e.wrappedError, &re) {
		return re.UserMsgKey()
	}

	switch e.Kind() {
	case KindInvalid:
		return msg.ErrInvalidInput
	case KindNotFound:
		return msg.ErrNotFound
	case KindForbidden:
		return msg.ErrForbidden
	case KindUnauthorized:
		return msg.ErrUnauthorized
	case KindUnexpected:
		return msg.ErrUnexpected
	default:
		return msg.ErrUnexpected
	}
}
func (e RichError) Op() Op {
	return e.op
}

func (e RichError) Meta() map[string]interface{} {
	return e.meta
}

func (e RichError) Unwrap() error {
	return e.wrappedError
}

func IsKind(err error, kind Kind) bool {
	var re RichError
	if errors.As(err, &re) {
		return re.Kind() == kind
	}
	return false
}
