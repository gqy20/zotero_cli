package benchmark

import "testing"

func TestSummarize(t *testing.T) {
	r := Result{RunsMS: []float64{4, 1, 3, 2, 5}}
	summarize(&r)
	if r.MedianMS != 3 || r.P95MS != 5 || r.MeanMS != 3 {
		t.Fatalf("unexpected summary: %+v", r)
	}
}

func TestDryRunSafetyGate(t *testing.T) {
	r := runScenario(Scenario{ID: "unsafe", Command: "item new", Tier: "default", Risk: "dry-run"}, Options{Tier: "default"})
	if r.Status != "blocked" {
		t.Fatalf("expected blocked, got %+v", r)
	}
}

func TestReadSource(t *testing.T) {
	got := readSource([]byte(`{"meta":{"read_source":"local"}}`))
	if got != "local" {
		t.Fatalf("expected local, got %q", got)
	}
}

func TestAnnotateNetLatency(t *testing.T) {
	values := []Result{{ID: "version", Status: "ok", MedianMS: 50}, {ID: "query", Status: "ok", MedianMS: 175}}
	annotateNetLatency(values)
	if values[1].NetMedianMS != 125 {
		t.Fatalf("expected 125ms net latency, got %.2f", values[1].NetMedianMS)
	}
}
