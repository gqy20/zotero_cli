package cli

import "fmt"

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

func (c *CLI) jsonError(err error, command string) int {
	code := ExitError
	var msg string
	if e, ok := err.(interface{ Code() int }); ok {
		code = e.Code()
	}
	msg = err.Error()
	if c.jsonErrorsEnabled() {
		return c.writeJSON(jsonResponse{OK: false, Command: command, Data: msg, Code: code})
	}
	fmt.Fprintf(c.stderr, "error: %s\n", msg)
	return code
}
