package reply

import (
	"encoding/json"
	"fmt"
	"testing"

	"altlinux.space/alt-atomic/apm/internal/common/apmerr"
)

func TestOK_JSON(t *testing.T) {
	resp := OK(map[string]interface{}{"name": "test"})
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]interface{}
	if err = json.Unmarshal(b, &parsed); err != nil {
		t.Fatal(err)
	}

	if parsed["error"] != nil {
		t.Error("OK response should have null error")
	}
	data := parsed["data"].(map[string]interface{})
	if data["name"] != "test" {
		t.Errorf("expected name=test, got %v", data["name"])
	}
}

func TestErrorResponseFromError_PlainError(t *testing.T) {
	resp := ErrorResponseFromError(fmt.Errorf("something went wrong"))

	if resp.Error == nil {
		t.Fatal("error response should have error")
	}
	if resp.Error.ErrorCode != "" {
		t.Error("plain error should not have error code")
	}
	if resp.Error.Message != "something went wrong" {
		t.Errorf("expected error message, got %q", resp.Error.Message)
	}
}

func TestErrorResponseFromError_APMError(t *testing.T) {
	apmErr := apmerr.New(apmerr.ErrorTypeValidation, fmt.Errorf("invalid package name"))
	resp := ErrorResponseFromError(apmErr)

	if resp.Error == nil {
		t.Fatal("error response should have error")
	}
	if resp.Error.ErrorCode != apmerr.ErrorTypeValidation {
		t.Errorf("expected error code VALIDATION, got %q", resp.Error.ErrorCode)
	}
	if resp.Error.Message != "invalid package name" {
		t.Errorf("expected error message, got %q", resp.Error.Message)
	}
}

func TestErrorResponseFromError_WrappedAPMError(t *testing.T) {
	apmErr := apmerr.New(apmerr.ErrorTypeNotFound, fmt.Errorf("package foo not found"))
	wrapped := fmt.Errorf("operation failed: %w", apmErr)
	resp := ErrorResponseFromError(wrapped)

	if resp.Error == nil {
		t.Fatal("error response should have error")
	}
	if resp.Error.ErrorCode != apmerr.ErrorTypeNotFound {
		t.Errorf("expected error code NOT_FOUND, got %q", resp.Error.ErrorCode)
	}
}

func TestRegisterFieldFormatter(t *testing.T) {
	RegisterFieldFormatter(func(key string, value interface{}) (string, bool) {
		if key == "testField" {
			return "custom", true
		}
		return "", false
	})

	if got, ok := formatFieldValue("testField", 1); !ok || got != "custom" {
		t.Errorf("registered formatter not applied: got (%q, %v)", got, ok)
	}
	if _, ok := formatFieldValue("otherField", 1); ok {
		t.Error("unrelated key should not be handled")
	}
}
