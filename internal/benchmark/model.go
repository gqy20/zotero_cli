package benchmark

import "time"

type Manifest struct {
	Version   int        `json:"version"`
	Commands  []Command  `json:"commands"`
	Scenarios []Scenario `json:"scenarios"`
}

type Command struct {
	Path        string   `json:"path"`
	Capability  string   `json:"capability"`
	Necessity   string   `json:"necessity"`
	Overlaps    []string `json:"overlaps,omitempty"`
	Replacement string   `json:"replacement,omitempty"`
}

type Scenario struct {
	ID       string   `json:"id"`
	Command  string   `json:"command"`
	Args     []string `json:"args"`
	Tier     string   `json:"tier"`
	Risk     string   `json:"risk"`
	Requires []string `json:"requires,omitempty"`
	Compare  string   `json:"compare,omitempty"`
	Variant  string   `json:"variant,omitempty"`
}

type Options struct {
	Binary     string
	Mode       string
	Tier       string
	Iterations int
	Warmup     int
	Timeout    time.Duration
	Vars       map[string]string
	Match      string
}

type Report struct {
	GeneratedAt time.Time       `json:"generated_at"`
	Binary      string          `json:"binary"`
	Mode        string          `json:"mode"`
	Iterations  int             `json:"iterations"`
	Commands    []CommandResult `json:"commands"`
	Scenarios   []Result        `json:"scenarios"`
}

type CommandResult struct {
	Command
	Status string  `json:"status"`
	MS     float64 `json:"ms,omitempty"`
	Error  string  `json:"error,omitempty"`
}

type Result struct {
	ID          string    `json:"id"`
	Command     string    `json:"command"`
	Status      string    `json:"status"`
	Reason      string    `json:"reason,omitempty"`
	RunsMS      []float64 `json:"runs_ms,omitempty"`
	ColdMS      float64   `json:"cold_ms,omitempty"`
	MedianMS    float64   `json:"median_ms,omitempty"`
	P95MS       float64   `json:"p95_ms,omitempty"`
	MeanMS      float64   `json:"mean_ms,omitempty"`
	NetMedianMS float64   `json:"net_median_ms,omitempty"`
	ExitCode    int       `json:"exit_code,omitempty"`
	StdoutBytes int       `json:"stdout_bytes,omitempty"`
	StderrBytes int       `json:"stderr_bytes,omitempty"`
	ReadSource  string    `json:"read_source,omitempty"`
	Compare     string    `json:"compare,omitempty"`
	Variant     string    `json:"variant,omitempty"`
}
