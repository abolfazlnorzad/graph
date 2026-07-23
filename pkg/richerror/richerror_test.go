package richerror_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/abolfazlnorzad/graph/pkg/msg"
	"github.com/abolfazlnorzad/graph/pkg/richerror"
)

func TestKindString(t *testing.T) {
	tests := []struct {
		kind     richerror.Kind
		expected string
	}{
		{richerror.KindInvalid, "Invalid"},
		{richerror.KindForbidden, "Forbidden"},
		{richerror.KindNotFound, "NotFound"},
		{richerror.KindUnexpected, "Unexpected"},
		{richerror.KindUnauthorized, "Unauthorized"},
		{richerror.Kind(999), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.kind.String(); got != tt.expected {
				t.Errorf("richerror.Kind(%d).String() = %q, want %q", tt.kind, got, tt.expected)
			}
		})
	}
}

func TestNew(t *testing.T) {
	e := richerror.New("pkg.fn")
	if e.Op() != "pkg.fn" {
		t.Errorf("op = %q, want %q", e.Op(), "pkg.fn")
	}
	if e.Meta() == nil {
		t.Fatal("meta is nil")
	}
	if _, ok := e.Meta()["file"]; !ok {
		t.Error("meta missing 'file'")
	}
	if _, ok := e.Meta()["line"]; !ok {
		t.Error("meta missing 'line'")
	}
}

func TestWithMessage(t *testing.T) {
	e := richerror.New("op").WithMessage("something failed")
	if e.Message() != "something failed" {
		t.Errorf("message = %q, want %q", e.Message(), "something failed")
	}
}

func TestWithUserMsgKey(t *testing.T) {
	e := richerror.New("op").WithUserMsgKey(msg.ErrNotFound)
	if e.UserMsgKey() != msg.ErrNotFound {
		t.Errorf("userMsgKey = %q, want %q", e.UserMsgKey(), msg.ErrNotFound)
	}
}

func TestWithKind(t *testing.T) {
	e := richerror.New("op").WithKind(richerror.KindInvalid)
	if e.Kind() != richerror.KindInvalid {
		t.Errorf("kind = %v, want %v", e.Kind(), richerror.KindInvalid)
	}
}

func TestWithErr(t *testing.T) {
	inner := errors.New("inner")
	e := richerror.New("op").WithErr(inner)
	if e.Unwrap() != inner {
		t.Error("wrappedError not set")
	}
}

func TestWithMeta(t *testing.T) {
	m := map[string]any{"key": "val"}
	e := richerror.New("op").WithMeta(m)
	if e.Meta()["key"] != "val" {
		t.Errorf("meta[key] = %v, want %v", e.Meta()["key"], "val")
	}
}

func TestError_WithWrapped(t *testing.T) {
	inner := errors.New("disk full")
	e := richerror.New("db.Save").WithMessage("write failed").WithErr(inner)
	got := e.Error()
	want := "db.Save: write failed -> disk full"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestError_WithoutWrapped(t *testing.T) {
	e := richerror.New("api.Call").WithMessage("timeout")
	got := e.Error()
	want := "api.Call: timeout"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestKind_WithExplicitKind(t *testing.T) {
	e := richerror.New("op").WithKind(richerror.KindForbidden)
	if got := e.Kind(); got != richerror.KindForbidden {
		t.Errorf("richerror.Kind() = %v, want %v", got, richerror.KindForbidden)
	}
}

func TestKind_WithWrappedRichError(t *testing.T) {
	inner := richerror.New("inner.Op()").WithKind(richerror.KindNotFound)
	outer := richerror.New("outer.Op()").WithErr(inner)
	if got := outer.Kind(); got != richerror.KindNotFound {
		t.Errorf("richerror.Kind() = %v, want %v", got, richerror.KindNotFound)
	}
}

func TestKind_Default(t *testing.T) {
	e := richerror.New("op")
	if got := e.Kind(); got != richerror.KindUnexpected {
		t.Errorf("richerror.Kind() = %v, want %v", got, richerror.KindUnexpected)
	}
}

func TestMessage(t *testing.T) {
	e := richerror.New("op").WithMessage("test msg")
	if got := e.Message(); got != "test msg" {
		t.Errorf("Message() = %q, want %q", got, "test msg")
	}
}

func TestUserMsgKey_Explicit(t *testing.T) {
	e := richerror.New("op").WithUserMsgKey(msg.ErrForbidden)
	if got := e.UserMsgKey(); got != msg.ErrForbidden {
		t.Errorf("UserMsgKey() = %q, want %q", got, msg.ErrForbidden)
	}
}

func TestUserMsgKey_FallbackToWrapped(t *testing.T) {
	inner := richerror.New("inner").WithUserMsgKey(msg.ErrNotFound)
	outer := richerror.New("outer").WithErr(inner)
	if got := outer.UserMsgKey(); got != msg.ErrNotFound {
		t.Errorf("UserMsgKey() = %q, want %q", got, msg.ErrNotFound)
	}
}

func TestUserMsgKey_FallbackToKind(t *testing.T) {
	tests := []struct {
		kind     richerror.Kind
		expected msg.Key
	}{
		{richerror.KindInvalid, msg.ErrInvalidInput},
		{richerror.KindNotFound, msg.ErrNotFound},
		{richerror.KindForbidden, msg.ErrForbidden},
		{richerror.KindUnauthorized, msg.ErrUnauthorized},
		{richerror.KindUnexpected, msg.ErrUnexpected},
	}

	for _, tt := range tests {
		t.Run(tt.kind.String(), func(t *testing.T) {
			e := richerror.New("op").WithKind(tt.kind)
			if got := e.UserMsgKey(); got != tt.expected {
				t.Errorf("UserMsgKey() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestOp(t *testing.T) {
	e := richerror.New("my.op")
	if got := e.Op(); got != "my.op" {
		t.Errorf("Op() = %q, want %q", got, "my.op")
	}
}

func TestMeta(t *testing.T) {
	e := richerror.New("op")
	m := e.Meta()
	if m == nil {
		t.Fatal("Meta() returned nil")
	}
	if _, ok := m["file"]; !ok {
		t.Error("Meta() missing 'file'")
	}
}

func TestUnwrap(t *testing.T) {
	inner := errors.New("inner")
	e := richerror.New("op").WithErr(inner)
	if got := e.Unwrap(); got != inner {
		t.Errorf("Unwrap() = %v, want %v", got, inner)
	}
}

func TestUnwrap_Nil(t *testing.T) {
	e := richerror.New("op")
	if got := e.Unwrap(); got != nil {
		t.Errorf("Unwrap() = %v, want nil", got)
	}
}

func TestIsKind_Match(t *testing.T) {
	e := richerror.New("op").WithKind(richerror.KindInvalid)
	if !richerror.IsKind(e, richerror.KindInvalid) {
		t.Error("richerror.IsKind returned false, want true")
	}
}

func TestIsKind_NoMatch(t *testing.T) {
	e := richerror.New("op").WithKind(richerror.KindInvalid)
	if richerror.IsKind(e, richerror.KindNotFound) {
		t.Error("richerror.IsKind returned true, want false")
	}
}

func TestIsKind_NonRichError(t *testing.T) {
	err := errors.New("plain error")
	if richerror.IsKind(err, richerror.KindInvalid) {
		t.Error("richerror.IsKind returned true for plain error")
	}
}

func TestIsKind_ResolvedKind(t *testing.T) {
	inner := richerror.New("inner").WithKind(richerror.KindNotFound)
	outer := fmt.Errorf("wrap: %w", inner)
	if !richerror.IsKind(outer, richerror.KindNotFound) {
		t.Error("richerror.IsKind should resolve kind from wrapped RichError")
	}
}

func TestIsKind_ExplicitBeatsWrapped(t *testing.T) {
	inner := richerror.New("inner").WithKind(richerror.KindNotFound)
	outer := richerror.New("outer").WithKind(richerror.KindForbidden).WithErr(inner)
	if !richerror.IsKind(outer, richerror.KindForbidden) {
		t.Error("richerror.IsKind should return outer's explicit kind, not inner's")
	}
}
