package references

import "testing"

func TestSummarizeContextsStatusesAndCoverage(t *testing.T) {
	refs := []Reference{{Index: 1}, {Index: 2}, {Index: 3}}
	pmc := SummarizeContexts(string(SourcePMC), refs, []Context{{ReferenceIndex: 1}, {ReferenceIndex: 1}, {ReferenceIndex: 3}})
	if pmc.Status != ContextAvailable || pmc.ContextCount != 3 || pmc.ReferencesWithContext != 2 || pmc.ReferencesWithoutContext != 1 || pmc.Coverage != 2.0/3.0 {
		t.Fatalf("pmc=%+v", pmc)
	}
	missing := SummarizeContexts(string(SourcePMC), refs, nil)
	if missing.Status != ContextNotFound || missing.ReferencesWithoutContext != 3 {
		t.Fatalf("missing=%+v", missing)
	}
	pubmed := SummarizeContexts(string(SourcePubMed), refs, nil)
	if pubmed.Status != ContextNotSupported || pubmed.ReferencesWithoutContext != 3 {
		t.Fatalf("pubmed=%+v", pubmed)
	}
	AnnotateReferenceContexts(refs, []Context{{ReferenceIndex: 1}, {ReferenceIndex: 1}, {ReferenceIndex: 3}}, pmc.Status)
	if refs[0].ContextStatus != ContextAvailable || refs[0].ContextCount != 2 || refs[1].ContextStatus != ContextNotFound || refs[2].ContextCount != 1 {
		t.Fatalf("annotated refs=%+v", refs)
	}
}
