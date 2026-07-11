package references

import "testing"

func TestParseJATSReferences(t *testing.T) {
	data := []byte(`<?xml version="1.0"?><article><body><sec><title>Methods</title><p>We followed <xref ref-type="bibr" rid="B1">Smith et al. [1]</xref> for extraction.</p></sec></body><back><ref-list>
<ref id="B1"><element-citation publication-type="journal"><person-group person-group-type="author"><name><surname>Smith</surname><given-names>Jane A.</given-names></name><collab>Genome Consortium</collab></person-group><article-title>A <italic>structured</italic> reference</article-title><source>Genome Biology</source><year>2024</year><volume>25</volume><issue>2</issue><fpage>10</fpage><lpage>19</lpage><pub-id pub-id-type="doi">10.1000/Example.1</pub-id><pub-id pub-id-type="pmid">12345678</pub-id><pub-id pub-id-type="pmcid">PMC999999</pub-id></element-citation></ref>
<ref id="B2"><mixed-citation>Doe J. An unstructured reference. 2020.</mixed-citation></ref>
</ref-list></back></article>`)
	refs, err := parseJATSReferences(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 2 {
		t.Fatalf("len = %d, want 2", len(refs))
	}
	first := refs[0]
	if first.ID != "B1" || first.Title != "A structured reference" || first.DOI != "10.1000/example.1" || first.PMID != "12345678" || first.PMCID != "PMC999999" {
		t.Fatalf("first = %#v", first)
	}
	if first.Pages != "10-19" || len(first.Authors) != 2 || first.Authors[0].Family != "Smith" || first.Source != SourcePMC {
		t.Fatalf("first fields = %#v", first)
	}
	if refs[1].Raw == "" {
		t.Fatal("mixed citation raw text was not preserved")
	}
	allRefs, contexts, err := parseJATSDocument(data)
	if err != nil || len(allRefs) != 2 || len(contexts) != 1 {
		t.Fatalf("document: refs=%d contexts=%d err=%v", len(allRefs), len(contexts), err)
	}
	if contexts[0].ReferenceID != "B1" || contexts[0].ReferenceIndex != 1 || contexts[0].Section != "Methods" || contexts[0].Marker != "Smith et al. [1]" {
		t.Fatalf("context = %#v", contexts[0])
	}
}
