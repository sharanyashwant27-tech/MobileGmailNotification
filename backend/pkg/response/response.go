package response

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/yashs/mobile-gmail-notification/pkg/apperrors"
)

type envelope struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *errorBody  `json:"error,omitempty"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// JSON writes a successful JSON response.
func JSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope{Success: true, Data: data})
}

// Error writes an error response derived from err.
func Error(w http.ResponseWriter, logger *slog.Logger, err error) {
	ae, ok := apperrors.AsAppError(err)
	if !ok {
		logger.Error("unhandled error", "error", err)
		ae = apperrors.ErrInternal
	} else if ae.HTTPStatus >= 500 {
		logger.Error("server error", "code", ae.Code, "error", ae.Err, "message", ae.Message)
	} else {
		logger.Warn("client error", "code", ae.Code, "message", ae.Message)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(ae.HTTPStatus)
	_ = json.NewEncoder(w).Encode(envelope{
		Success: false,
		Error:   &errorBody{Code: ae.Code, Message: ae.Message},
	})
}
