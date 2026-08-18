package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	openapi "github.com/nihatatay93/budget/internal/api/openapi"
)

func TestRecoveryUsesErrorEnvelope(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := requestIDMiddleware(recoverMiddleware(logger, http.HandlerFunc(
		func(http.ResponseWriter, *http.Request) {
			panic("test panic")
		},
	)))

	request := httptest.NewRequest(http.MethodGet, "/panic", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}

	var body openapi.ErrorResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Error.Code != "internal_error" {
		t.Fatalf("error code = %q, want internal_error", body.Error.Code)
	}
	if body.Error.RequestId == "" || body.Error.RequestId != response.Header().Get(requestIDHeader) {
		t.Fatalf("body request ID %q does not match response header", body.Error.RequestId)
	}
}
