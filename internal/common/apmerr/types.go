package apmerr

import (
	"errors"
	"net/http"
	"strings"
)

const dbusErrorPrefix = "org.altlinux.APM.Error."

const (
	ErrorTypeDatabase    = "DATABASE"
	ErrorTypeRepository  = "REPOSITORY"
	ErrorTypeApt         = "APT"
	ErrorTypeValidation  = "VALIDATION"
	ErrorTypePermission  = "PERMISSION"
	ErrorTypeCanceled    = "CANCELED"
	ErrorTypeImage       = "IMAGE"
	ErrorTypeKernel      = "KERNEL"
	ErrorTypeContainer   = "CONTAINER"
	ErrorTypeNoOperation = "NO_OPERATION"
	ErrorTypeNotFound    = "NOT_FOUND"
)

type APMError struct {
	Type string
	Err  error
}

// New создание новой классифицированной ошибки
func New(errorType string, err error) APMError {
	if errorType == ErrorTypeApt {
		var nf interface{ IsNotFound() bool }
		if errors.As(err, &nf) && nf.IsNotFound() {
			errorType = ErrorTypeNotFound
		}
	}
	return APMError{Type: errorType, Err: err}
}

func (e APMError) Error() string {
	return e.Err.Error()
}

func (e APMError) Unwrap() error {
	return e.Err
}

// DBusErrorName возвращает полное DBus-имя ошибки
func (e APMError) DBusErrorName() string {
	parts := strings.Split(e.Type, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
		}
	}
	return dbusErrorPrefix + strings.Join(parts, "")
}

// HTTPStatus возвращает HTTP статус-код для данного типа ошибки.
func (e APMError) HTTPStatus() int {
	switch e.Type {
	case ErrorTypeValidation:
		return http.StatusBadRequest
	case ErrorTypePermission:
		return http.StatusForbidden
	case ErrorTypeNotFound:
		return http.StatusNotFound
	case ErrorTypeCanceled, ErrorTypeNoOperation:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
