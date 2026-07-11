package app

type Warning struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

type Result struct {
	Data     any
	Meta     map[string]any
	Text     string
	Warnings []Warning
}
