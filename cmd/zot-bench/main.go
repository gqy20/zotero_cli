package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"zotero_cli/internal/benchmark"
)

type varsFlag map[string]string

func (v varsFlag) String() string { return "KEY=VALUE" }
func (v varsFlag) Set(s string) error {
	p := strings.SplitN(s, "=", 2)
	if len(p) != 2 {
		return fmt.Errorf("expected KEY=VALUE")
	}
	v[p[0]] = p[1]
	return nil
}

func main() {
	vars := varsFlag{"QUERY": "benchmark", "LIMIT": "5"}
	manifest := flag.String("manifest", "benchmarks/cli/manifest.json", "benchmark manifest")
	defaultBinary := "./zot"
	if runtime.GOOS == "windows" {
		defaultBinary = ".\\zot.exe"
	}
	binary := flag.String("binary", defaultBinary, "zot binary to benchmark")
	mode := flag.String("mode", "all", "coverage, runtime, or all")
	tier := flag.String("tier", "default", "default, data, or extended")
	iterations := flag.Int("iterations", 7, "measured runs per scenario")
	warmup := flag.Int("warmup", 1, "warmup runs per scenario")
	timeout := flag.Duration("timeout", 30*time.Second, "per-run timeout")
	match := flag.String("case", "", "run command paths or scenario IDs containing this value")
	jsonOut := flag.String("json-out", "benchmarks/results/latest.json", "JSON report path")
	mdOut := flag.String("md-out", "benchmarks/results/latest.md", "Markdown report path")
	flag.Var(vars, "var", "scenario variable KEY=VALUE (repeatable)")
	flag.Parse()
	b, err := os.ReadFile(*manifest)
	must(err)
	var m benchmark.Manifest
	must(json.Unmarshal(b, &m))
	r := benchmark.Run(m, benchmark.Options{Binary: *binary, Mode: *mode, Tier: *tier, Iterations: *iterations, Warmup: *warmup, Timeout: *timeout, Vars: vars, Match: *match})
	must(os.MkdirAll(filepath.Dir(*jsonOut), 0755))
	must(os.MkdirAll(filepath.Dir(*mdOut), 0755))
	j, err := json.MarshalIndent(r, "", "  ")
	must(err)
	must(os.WriteFile(*jsonOut, append(j, '\n'), 0644))
	must(os.WriteFile(*mdOut, []byte(benchmark.Markdown(r)), 0644))
	fmt.Printf("wrote %s and %s\n", *jsonOut, *mdOut)
}
func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
