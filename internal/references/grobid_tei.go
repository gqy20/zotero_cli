package references

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

type teiBibl struct {
	XMLID    string `xml:"http://www.w3.org/XML/1998/namespace id,attr"`
	Analytic struct {
		Title   innerText `xml:"title"`
		Authors []struct {
			Pers struct {
				Surname   string   `xml:"surname"`
				Forenames []string `xml:"forename"`
			} `xml:"persName"`
		} `xml:"author"`
	} `xml:"analytic"`
	Monogr struct {
		Title   innerText `xml:"title"`
		Imprint struct {
			Date struct {
				When string `xml:"when,attr"`
				Text string `xml:",chardata"`
			} `xml:"date"`
			Scopes []struct {
				Unit string `xml:"unit,attr"`
				From string `xml:"from,attr"`
				To   string `xml:"to,attr"`
				Text string `xml:",chardata"`
			} `xml:"biblScope"`
		} `xml:"imprint"`
	} `xml:"monogr"`
	IDs []struct {
		Type string `xml:"type,attr"`
		Text string `xml:",chardata"`
	} `xml:"idno"`
	Notes []struct {
		Type string `xml:"type,attr"`
		Text string `xml:",chardata"`
	} `xml:"note"`
}

func ParseGrobidTEI(data []byte) ([]Reference, []Context, error) {
	refs, err := parseGrobidRefs(data)
	if err != nil {
		return nil, nil, err
	}
	contexts, err := parseGrobidContexts(data)
	if err != nil {
		return nil, nil, err
	}
	linkContextIndexes(refs, contexts)
	return refs, contexts, nil
}
func parseGrobidRefs(data []byte) ([]Reference, error) {
	d := xml.NewDecoder(strings.NewReader(string(data)))
	depth := 0
	var out []Reference
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("decode GROBID TEI: %w", err)
		}
		switch v := tok.(type) {
		case xml.StartElement:
			if v.Name.Local == "listBibl" {
				depth++
			} else if depth > 0 && v.Name.Local == "biblStruct" {
				var b teiBibl
				if err := d.DecodeElement(&b, &v); err != nil {
					return nil, err
				}
				r := Reference{Index: len(out) + 1, ID: b.XMLID, Title: cleanSpace(b.Analytic.Title.Text), Container: cleanSpace(b.Monogr.Title.Text), Source: SourceGROBID}
				for _, a := range b.Analytic.Authors {
					r.Authors = append(r.Authors, Author{Family: cleanSpace(a.Pers.Surname), Given: cleanSpace(strings.Join(a.Pers.Forenames, " "))})
				}
				r.Year = yearFromDate(firstNonEmptyRef(b.Monogr.Imprint.Date.When, b.Monogr.Imprint.Date.Text))
				for _, s := range b.Monogr.Imprint.Scopes {
					val := cleanSpace(s.Text)
					if s.From != "" {
						val = s.From
						if s.To != "" && s.To != s.From {
							val += "-" + s.To
						}
					}
					switch strings.ToLower(s.Unit) {
					case "volume":
						r.Volume = val
					case "issue":
						r.Issue = val
					case "page":
						r.Pages = val
					}
				}
				for _, id := range b.IDs {
					switch strings.ToLower(id.Type) {
					case "doi":
						r.DOI = normalizeDOI(id.Text)
					case "pmid":
						r.PMID = cleanSpace(id.Text)
					case "pmcid":
						r.PMCID = normalizePMCID(id.Text)
					}
				}
				for _, n := range b.Notes {
					if n.Type == "raw_reference" {
						r.Raw = cleanSpace(n.Text)
					}
				}
				out = append(out, r)
			}
		case xml.EndElement:
			if v.Name.Local == "listBibl" && depth > 0 {
				depth--
			}
		}
	}
	return out, nil
}
func parseGrobidContexts(data []byte) ([]Context, error) {
	d := xml.NewDecoder(strings.NewReader(string(data)))
	inBody := false
	sections := []string{}
	var out []Context
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch v := tok.(type) {
		case xml.StartElement:
			switch v.Name.Local {
			case "body":
				inBody = true
			case "div":
				if inBody {
					sections = append(sections, "")
				}
			case "head":
				if inBody && len(sections) > 0 {
					var x innerText
					if err := d.DecodeElement(&x, &v); err != nil {
						return nil, err
					}
					sections[len(sections)-1] = cleanSpace(x.Text)
				}
			case "p":
				if inBody {
					cs, err := decodeTEIParagraph(d, v, teiCurrentSection(sections))
					if err != nil {
						return nil, err
					}
					out = append(out, cs...)
				}
			}
		case xml.EndElement:
			if v.Name.Local == "body" {
				inBody = false
			} else if v.Name.Local == "div" && inBody && len(sections) > 0 {
				sections = sections[:len(sections)-1]
			}
		}
	}
	return out, nil
}

func teiCurrentSection(sections []string) string {
	for i := len(sections) - 1; i >= 0; i-- {
		if sections[i] != "" {
			return sections[i]
		}
	}
	return ""
}
func decodeTEIParagraph(d *xml.Decoder, start xml.StartElement, section string) ([]Context, error) {
	var p struct {
		Inner string `xml:",innerxml"`
	}
	if err := d.DecodeElement(&p, &start); err != nil {
		return nil, err
	}
	plain := cleanSpace(stripXMLText(p.Inner))
	pd := xml.NewDecoder(strings.NewReader("<p>" + p.Inner + "</p>"))
	var out []Context
	for {
		tok, err := pd.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if s, ok := tok.(xml.StartElement); ok && s.Name.Local == "ref" {
			typ, target := "", ""
			for _, a := range s.Attr {
				if a.Name.Local == "type" {
					typ = a.Value
				}
				if a.Name.Local == "target" {
					target = a.Value
				}
			}
			var marker innerText
			if err := pd.DecodeElement(&marker, &s); err != nil {
				return nil, err
			}
			if typ == "bibr" {
				for _, id := range strings.Fields(target) {
					out = append(out, Context{ReferenceID: strings.TrimPrefix(id, "#"), Marker: cleanSpace(marker.Text), Section: section, Paragraph: plain, Source: SourceGROBID})
				}
			}
		}
	}
	return out, nil
}
func stripXMLText(inner string) string {
	d := xml.NewDecoder(strings.NewReader("<x>" + inner + "</x>"))
	var b strings.Builder
	for {
		tok, err := d.Token()
		if err != nil {
			break
		}
		if c, ok := tok.(xml.CharData); ok {
			b.Write(c)
			b.WriteByte(' ')
		}
	}
	return b.String()
}
func yearFromDate(s string) string {
	for _, f := range strings.FieldsFunc(s, func(r rune) bool { return r < '0' || r > '9' }) {
		if len(f) == 4 {
			return f
		}
	}
	return ""
}
func firstNonEmptyRef(v ...string) string {
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}
