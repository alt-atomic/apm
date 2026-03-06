package apmerr

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestNew_BasicError(t *testing.T) {
	err := New(ErrorTypeValidation, fmt.Errorf("bad input"))

	if err.Type != ErrorTypeValidation {
		t.Errorf("expected VALIDATION, got %s", err.Type)
	}
	if err.Error() != "bad input" {
		t.Errorf("expected 'bad input', got %s", err.Error())
	}
}

func TestNew_AptNotFound_ConvertsToNotFound(t *testing.T) {
	nfErr := &notFoundError{msg: "package foo not found"}
	err := New(ErrorTypeApt, nfErr)

	if err.Type != ErrorTypeNotFound {
		t.Errorf("APT not-found should be reclassified to NOT_FOUND, got %s", err.Type)
	}
}

func TestNew_AptRegularError_StaysApt(t *testing.T) {
	err := New(ErrorTypeApt, fmt.Errorf("broken dependency"))

	if err.Type != ErrorTypeApt {
		t.Errorf("regular APT error should stay APT, got %s", err.Type)
	}
}

func TestAPMError_Unwrap(t *testing.T) {
	inner := fmt.Errorf("root cause")
	err := New(ErrorTypeDatabase, inner)

	if !errors.Is(err, inner) {
		t.Error("Unwrap should expose inner error")
	}
}

func TestAPMError_ErrorsAs(t *testing.T) {
	err := New(ErrorTypePermission, fmt.Errorf("access denied"))
	wrapped := fmt.Errorf("operation failed: %w", err)

	var apmErr APMError
	if !errors.As(wrapped, &apmErr) {
		t.Fatal("errors.As should find APMError in chain")
	}
	if apmErr.Type != ErrorTypePermission {
		t.Errorf("expected PERMISSION, got %s", apmErr.Type)
	}
}

func TestDBusErrorName(t *testing.T) {
	cases := []struct {
		errorType string
		expected  string
	}{
		{ErrorTypeValidation, "org.altlinux.APM.Error.Validation"},
		{ErrorTypeNotFound, "org.altlinux.APM.Error.NotFound"},
		{ErrorTypeNoOperation, "org.altlinux.APM.Error.NoOperation"},
		{ErrorTypeDatabase, "org.altlinux.APM.Error.Database"},
		{ErrorTypePermission, "org.altlinux.APM.Error.Permission"},
		{ErrorTypeCanceled, "org.altlinux.APM.Error.Canceled"},
	}

	for _, c := range cases {
		err := New(c.errorType, fmt.Errorf("test"))
		got := err.DBusErrorName()
		if got != c.expected {
			t.Errorf("DBusErrorName(%s) = %s, want %s", c.errorType, got, c.expected)
		}
	}
}

func TestHTTPStatus(t *testing.T) {
	cases := []struct {
		errorType  string
		httpStatus int
	}{
		{ErrorTypeValidation, http.StatusBadRequest},
		{ErrorTypePermission, http.StatusForbidden},
		{ErrorTypeNotFound, http.StatusNotFound},
		{ErrorTypeCanceled, http.StatusConflict},
		{ErrorTypeNoOperation, http.StatusConflict},
		{ErrorTypeDatabase, http.StatusInternalServerError},
		{ErrorTypeApt, http.StatusInternalServerError},
		{ErrorTypeRepository, http.StatusInternalServerError},
		{ErrorTypeImage, http.StatusInternalServerError},
		{ErrorTypeKernel, http.StatusInternalServerError},
		{ErrorTypeContainer, http.StatusInternalServerError},
	}

	for _, c := range cases {
		err := New(c.errorType, fmt.Errorf("test"))
		got := err.HTTPStatus()
		if got != c.httpStatus {
			t.Errorf("HTTPStatus(%s) = %d, want %d", c.errorType, got, c.httpStatus)
		}
	}
}

// notFoundError реализует IsNotFound() для тестирования автоконвертации
type notFoundError struct {
	msg string
}

func (e *notFoundError) Error() string    { return e.msg }
func (e *notFoundError) IsNotFound() bool { return true }
