package app

type CommandPath struct {
	Resource string
	Action   string
}

func (p CommandPath) String() string {
	if p.Action == "" {
		return p.Resource
	}
	return p.Resource + " " + p.Action
}

type OutputOptions struct {
	Format  string
	Verbose bool
	Color   bool
}

type RuntimeOptions struct {
	Mode    string
	Timeout string
}

type SafetyOptions struct {
	DryRun    bool
	Yes       bool
	IfVersion int
	Confirm   func(string) bool
}

type Invocation struct {
	Path    CommandPath
	Keys    []string
	Query   string
	Output  OutputOptions
	Runtime RuntimeOptions
	Safety  SafetyOptions
}
