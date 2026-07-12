package benchmark

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"sort"
	"strings"
	"time"
)

var tiers = map[string]int{"default": 0, "data": 1, "extended": 2}

func Run(m Manifest, o Options) Report {
	r := Report{GeneratedAt: time.Now(), Binary: o.Binary, Mode: o.Mode, Iterations: o.Iterations}
	if o.Mode == "coverage" || o.Mode == "all" {
		for _, c := range m.Commands {
			if o.Match != "" && !strings.Contains(c.Path, o.Match) {
				continue
			}
			ms, err := invoke(o, append(strings.Fields(c.Path), "--help")...)
			cr := CommandResult{Command: c, Status: "ok", MS: ms.duration}
			if err != nil {
				cr.Status, cr.Error = "failed", err.Error()
			}
			r.Commands = append(r.Commands, cr)
		}
	}
	if o.Mode == "runtime" || o.Mode == "all" {
		for _, s := range m.Scenarios {
			if o.Match != "" && s.ID != "version" && !strings.Contains(s.ID, o.Match) {
				continue
			}
			r.Scenarios = append(r.Scenarios, runScenario(s, o))
		}
		annotateNetLatency(r.Scenarios)
	}
	return r
}

type invocation struct {
	duration            float64
	code, out, errBytes int
	stdout              []byte
}

func invoke(o Options, args ...string) (invocation, error) {
	ctx, cancel := context.WithTimeout(context.Background(), o.Timeout)
	defer cancel()
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, o.Binary, args...)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	start := time.Now()
	err := cmd.Run()
	d := float64(time.Since(start).Microseconds()) / 1000
	code := 0
	if cmd.ProcessState != nil {
		code = cmd.ProcessState.ExitCode()
	}
	inv := invocation{duration: d, code: code, out: stdout.Len(), errBytes: stderr.Len(), stdout: append([]byte(nil), stdout.Bytes()...)}
	if ctx.Err() == context.DeadlineExceeded {
		return inv, fmt.Errorf("timeout after %s", o.Timeout)
	}
	return inv, err
}

func runScenario(s Scenario, o Options) Result {
	r := Result{ID: s.ID, Command: s.Command, Status: "skipped", Compare: s.Compare, Variant: s.Variant}
	if tiers[s.Tier] > tiers[o.Tier] {
		r.Reason = "tier " + s.Tier + " not enabled"
		return r
	}
	if s.Risk != "read" && s.Risk != "dry-run" {
		r.Reason = "risk " + s.Risk + " is never automatic"
		return r
	}
	args := append(strings.Fields(s.Command), s.Args...)
	for _, key := range s.Requires {
		if o.Vars[key] == "" {
			r.Reason = "missing variable " + key
			return r
		}
	}
	for i, arg := range args {
		for key, value := range o.Vars {
			arg = strings.ReplaceAll(arg, "{{"+key+"}}", value)
		}
		args[i] = arg
	}
	if s.Risk == "dry-run" && !contains(args, "--dry-run") {
		r.Status, r.Reason = "blocked", "dry-run scenario lacks --dry-run"
		return r
	}
	cold, err := invoke(o, args...)
	if err != nil {
		r.Status, r.Reason = "failed", err.Error()
		return r
	}
	r.ColdMS = cold.duration
	for i := 0; i < o.Warmup; i++ {
		if _, err := invoke(o, args...); err != nil {
			r.Status, r.Reason = "failed", err.Error()
			return r
		}
	}
	for i := 0; i < o.Iterations; i++ {
		inv, err := invoke(o, args...)
		r.ExitCode, r.StdoutBytes, r.StderrBytes = inv.code, inv.out, inv.errBytes
		r.ReadSource = readSource(inv.stdout)
		if err != nil {
			r.Status, r.Reason = "failed", err.Error()
			return r
		}
		r.RunsMS = append(r.RunsMS, inv.duration)
	}
	r.Status = "ok"
	summarize(&r)
	return r
}

func readSource(data []byte) string {
	var envelope struct {
		Meta map[string]any `json:"meta"`
	}
	if json.Unmarshal(data, &envelope) != nil {
		return ""
	}
	value, _ := envelope.Meta["read_source"].(string)
	return value
}

func annotateNetLatency(results []Result) {
	baseline := 0.0
	for _, result := range results {
		if result.ID == "version" && result.Status == "ok" {
			baseline = result.MedianMS
			break
		}
	}
	for i := range results {
		if results[i].Status != "ok" {
			continue
		}
		results[i].NetMedianMS = results[i].MedianMS - baseline
		if results[i].NetMedianMS < 0 {
			results[i].NetMedianMS = 0
		}
	}
}

func summarize(r *Result) {
	if len(r.RunsMS) == 0 {
		return
	}
	v := append([]float64(nil), r.RunsMS...)
	sort.Float64s(v)
	r.MedianMS = v[len(v)/2]
	r.P95MS = v[int(math.Ceil(float64(len(v))*0.95))-1]
	for _, n := range v {
		r.MeanMS += n
	}
	r.MeanMS /= float64(len(v))
}

func contains(v []string, target string) bool {
	for _, x := range v {
		if x == target {
			return true
		}
	}
	return false
}
