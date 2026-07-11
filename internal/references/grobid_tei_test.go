package references

import "testing"

func TestParseGrobidTEI(t *testing.T) {
	data := []byte(`<TEI xmlns="http://www.tei-c.org/ns/1.0"><text><body><div><head>Methods</head><p>We used <ref type="bibr" target="#b0">[1]</ref> and <ref type="bibr" target="#b0 #b1">[1,2]</ref>.</p></div></body><back><div><listBibl><biblStruct xml:id="b0"><analytic><title>First paper</title><author><persName><forename>Jane</forename><surname>Smith</surname></persName></author></analytic><monogr><title>Journal</title><imprint><date when="2024"/><biblScope unit="volume">2</biblScope><biblScope unit="page" from="10" to="12"/></imprint></monogr><idno type="DOI">10.1/ABC</idno><note type="raw_reference">Smith. First paper.</note></biblStruct><biblStruct xml:id="b1"><analytic><title>Second paper</title></analytic></biblStruct></listBibl></div></back></text></TEI>`)
	refs, contexts, err := ParseGrobidTEI(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 2 || refs[0].ID != "b0" || refs[0].DOI != "10.1/abc" || refs[0].Pages != "10-12" || refs[0].Authors[0].Family != "Smith" {
		t.Fatalf("refs=%+v", refs)
	}
	if len(contexts) != 3 || contexts[0].ReferenceIndex != 1 || contexts[2].ReferenceIndex != 2 || contexts[0].Section != "Methods" {
		t.Fatalf("contexts=%+v", contexts)
	}
}
