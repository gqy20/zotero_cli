package app

import (
	"errors"
	"fmt"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
	"zotero_cli/internal/zoteroapi"
)

type UsageError struct {
	Message string
}

func (e *UsageError) Error() string { return e.Message }

func NewUsageError(message string) error { return &UsageError{Message: message} }

func IsUsageError(err error) bool {
	var target *UsageError
	return errors.As(err, &target)
}

func ClassifyError(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case IsUsageError(err):
		return "usage"
	case errors.Is(err, config.ErrNotFound):
		return "config_not_found"
	case errors.Is(err, backend.ErrItemNotFound):
		return "not_found"
	case errors.Is(err, backend.ErrUnsupportedFeature):
		return "unsupported_feature"
	case errors.Is(err, backend.ErrLocalTemporarilyUnavailable):
		return "temporarily_unavailable"
	}
	var apiErr *zoteroapi.APIError
	if !errors.As(err, &apiErr) {
		return "unknown"
	}
	switch apiErr.StatusCode {
	case 400:
		return "bad_request"
	case 401:
		return "unauthorized"
	case 403:
		return "forbidden"
	case 404:
		return "not_found"
	case 405:
		return "method_not_allowed"
	case 409:
		return "conflict"
	case 412:
		return "precondition_failed"
	case 413:
		return "payload_too_large"
	case 429:
		return "rate_limited"
	case 500, 502, 503, 504:
		return fmt.Sprintf("server_error_%d", apiErr.StatusCode)
	default:
		if apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 {
			return "client_error"
		}
		if apiErr.StatusCode >= 500 {
			return "server_error"
		}
		return "api_error"
	}
}
