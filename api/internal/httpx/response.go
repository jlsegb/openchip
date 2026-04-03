package httpx

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type Envelope struct {
	Data  interface{} `json:"data"`
	Error *APIError   `json:"error"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(Envelope{Data: data, Error: nil}); err != nil {
		logEncodeFailure(w, err)
	}
}

func WriteError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(Envelope{
		Data:  nil,
		Error: &APIError{Code: code, Message: message},
	}); err != nil {
		logEncodeFailure(w, err)
	}
}

func logEncodeFailure(w http.ResponseWriter, err error) {
	logger := slog.Default()
	requestID := ""
	if withLogger, ok := w.(interface{ Logger() *slog.Logger }); ok && withLogger.Logger() != nil {
		logger = withLogger.Logger()
	}
	if withRequestID, ok := w.(interface{ RequestID() string }); ok {
		requestID = withRequestID.RequestID()
	}
	logger.Error("failed to encode response", slog.String("request_id", requestID), slog.String("error", err.Error()))
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}
