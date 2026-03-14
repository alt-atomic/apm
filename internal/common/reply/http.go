package reply

import (
	"apm/internal/common/apmerr"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// WriteHTTPError записывает классифицированную ошибку в HTTP-ответ.
// Если ошибка является APMError, используется соответствующий HTTP-статус.
// Иначе возвращается 500 Internal Server Error.
func WriteHTTPError(rw http.ResponseWriter, err error) {
	rw.Header().Set("Content-Type", "application/json; charset=utf-8")

	var apmErr apmerr.APMError
	if errors.As(err, &apmErr) {
		rw.WriteHeader(apmErr.HTTPStatus())
	} else {
		rw.WriteHeader(http.StatusInternalServerError)
	}

	_ = json.NewEncoder(rw).Encode(ErrorResponseFromError(err))
}

// UnmarshalField извлекает и десериализует поле из тела запроса
func UnmarshalField(body map[string]json.RawMessage, key string, target interface{}) error {
	raw, ok := body[key]
	if !ok {
		return nil
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("invalid value for %q: %w", key, err)
	}
	return nil
}
