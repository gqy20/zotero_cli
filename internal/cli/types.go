package cli

import (
	"errors"
	"fmt"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/zoteroapi"
)

// Exit codes — 统一规范，供 AI Agent 解析命令执行结果
const (
	ExitOK     = 0 // 成功
	ExitError  = 1 // 运行时错误（API 失败、条目不存在、权限拒绝等）
	ExitUsage  = 2 // 参数/用法错误（缺少参数、未知选项等）
	ExitConfig = 3 // 配置错误（配置文件缺失、认证失败等）
)

type versionsArgs struct {
	Since                  int
	IncludeTrashed         bool
	IfModifiedSinceVersion int
}

type jsonResponse struct {
	OK      bool           `json:"ok"`
	Command string         `json:"command"`
	Data    any            `json:"data"`
	Meta    map[string]any `json:"meta,omitempty"`
	Code    int            `json:"code,omitempty"`
}

// errorData is the structured error payload inside JSON error responses.
// Agents can check "type" for programmatic handling and "error" for display.
type errorData struct {
	Error string `json:"error"`          // human-readable message
	Type  string `json:"type,omitempty"` // machine-readable category
	Code  int    `json:"code,omitempty"` // exit code
}

func (c *CLI) jsonError(err error, command string) int {
	code := ExitError
	if e, ok := err.(interface{ Code() int }); ok {
		code = e.Code()
	}
	msg := err.Error()
	errType := classifyErrorType(err)
	if c.jsonErrorsEnabled() {
		return c.writeJSON(jsonResponse{
			OK:      false,
			Command: command,
			Data:    errorData{Error: msg, Type: errType, Code: code},
			Code:    code,
		})
	}
	fmt.Fprintf(c.stderr, "error: %s\n", msg)
	return code
}

// classifyErrorType returns a machine-readable error category string.
func classifyErrorType(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, backend.ErrItemNotFound):
		return "not_found"
	case errors.Is(err, backend.ErrUnsupportedFeature):
		return "unsupported_feature"
	case errors.Is(err, backend.ErrLocalTemporarilyUnavailable):
		return "temporarily_unavailable"
	default:
		var apiErr *zoteroapi.APIError
		if errors.As(err, &apiErr) {
			return classifyAPIErrorType(apiErr)
		}
		return "unknown"
	}
}

func classifyAPIErrorType(apiErr *zoteroapi.APIError) string {
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
