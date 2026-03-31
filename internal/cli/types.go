package cli

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
}
