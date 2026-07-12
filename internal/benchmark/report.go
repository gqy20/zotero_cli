package benchmark

import (
	"fmt"
	"sort"
	"strings"
)

func Markdown(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# CLI benchmark report\n\nGenerated: %s  \nBinary: `%s`  \nMode: `%s`  \nIterations: %d\n\n", r.GeneratedAt.Format("2006-01-02 15:04:05Z07:00"), r.Binary, r.Mode, r.Iterations)
	if len(r.Scenarios) > 0 {
		b.WriteString("## Runtime scenarios\n\n| Scenario | Command | Status | Source | Cold ms | Median ms | Net ms | P95 ms | Note |\n|---|---|---:|---|---:|---:|---:|---:|---|\n")
		for _, x := range r.Scenarios {
			fmt.Fprintf(&b, "| %s | `%s` | %s | %s | %.2f | %.2f | %.2f | %.2f | %s |\n", x.ID, x.Command, x.Status, x.ReadSource, x.ColdMS, x.MedianMS, x.NetMedianMS, x.P95MS, x.Reason)
		}
		b.WriteString("\n")
		writeHotspots(&b, r.Scenarios)
		writeComparisons(&b, r.Scenarios)
	}
	if len(r.Commands) > 0 {
		b.WriteString("## Command coverage and necessity audit\n\n| Command | Help ms | Status | Necessity | Overlap / replacement |\n|---|---:|---|---|---|\n")
		for _, x := range r.Commands {
			o := strings.Join(x.Overlaps, ", ")
			if x.Replacement != "" {
				o += " -> " + x.Replacement
			}
			fmt.Fprintf(&b, "| `%s` | %.2f | %s | %s | %s |\n", x.Path, x.MS, x.Status, x.Necessity, o)
		}
	}
	b.WriteString("\n## Interpretation\n\nHelp timings measure process startup and command construction, not Zotero operation latency. Removal decisions require runtime data plus capability overlap; latency alone is not sufficient.\n")
	return b.String()
}

func writeComparisons(b *strings.Builder, values []Result) {
	groups := map[string][]Result{}
	for _, value := range values {
		if value.Compare != "" && value.Status == "ok" {
			groups[value.Compare] = append(groups[value.Compare], value)
		}
	}
	if len(groups) == 0 {
		return
	}
	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)
	b.WriteString("## Backend comparisons\n\n| Comparison | Variant | Source | Median ms | Net ms |\n|---|---|---|---:|---:|\n")
	for _, name := range names {
		for _, value := range groups[name] {
			fmt.Fprintf(b, "| %s | %s | %s | %.2f | %.2f |\n", name, value.Variant, value.ReadSource, value.MedianMS, value.NetMedianMS)
		}
	}
	b.WriteString("\n")
}

func writeHotspots(b *strings.Builder, values []Result) {
	ok := make([]Result, 0, len(values))
	for _, value := range values {
		if value.Status == "ok" && value.ID != "version" {
			ok = append(ok, value)
		}
	}
	sort.Slice(ok, func(i, j int) bool { return ok[i].NetMedianMS > ok[j].NetMedianMS })
	if len(ok) > 8 {
		ok = ok[:8]
	}
	b.WriteString("## Runtime hotspots\n\n| Scenario | Source | Net median ms | Output bytes |\n|---|---|---:|---:|\n")
	for _, x := range ok {
		fmt.Fprintf(b, "| %s | %s | %.2f | %d |\n", x.ID, x.ReadSource, x.NetMedianMS, x.StdoutBytes)
	}
	b.WriteString("\nNet median subtracts the `version` median as an estimate of fixed process startup and command construction cost.\n\n")
}
