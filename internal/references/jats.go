package references

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

type jatsRef struct {
	ID        string
	Raw       string
	Title     string
	Container string
	Year      string
	Volume    string
	Issue     string
	FPage     string
	LPage     string
	DOI       string
	PMID      string
	PMCID     string
	Authors   []Author
}

func parseJATSReferences(data []byte) ([]Reference, error) {
	decoder := xml.NewDecoder(strings.NewReader(string(data)))
	refs := []Reference{}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("decode PMC JATS: %w", err)
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "ref" {
			continue
		}
		ref, err := decodeJATSRef(decoder, start)
		if err != nil {
			return nil, err
		}
		pages := ref.FPage
		if ref.LPage != "" && ref.LPage != ref.FPage {
			pages += "-" + ref.LPage
		}
		refs = append(refs, Reference{Index: len(refs) + 1, ID: ref.ID, Raw: ref.Raw, Title: ref.Title, Authors: ref.Authors, Container: ref.Container, Year: ref.Year, Volume: ref.Volume, Issue: ref.Issue, Pages: pages, DOI: ref.DOI, PMID: ref.PMID, PMCID: ref.PMCID, Source: SourcePMC})
	}
	return refs, nil
}

func decodeJATSRef(decoder *xml.Decoder, start xml.StartElement) (jatsRef, error) {
	ref := jatsRef{}
	for _, attr := range start.Attr {
		if attr.Name.Local == "id" {
			ref.ID = attr.Value
		}
	}
	var raw strings.Builder
	stack := []xml.StartElement{start}
	var currentAuthor *Author
	for len(stack) > 0 {
		token, err := decoder.Token()
		if err != nil {
			return ref, fmt.Errorf("decode PMC reference: %w", err)
		}
		switch value := token.(type) {
		case xml.StartElement:
			stack = append(stack, value)
			if value.Name.Local == "name" {
				currentAuthor = &Author{}
			}
		case xml.CharData:
			text := cleanSpace(string(value))
			if text == "" {
				continue
			}
			raw.WriteString(" ")
			raw.WriteString(text)
			name := semanticJATSElement(stack)
			switch name {
			case "article-title", "chapter-title":
				ref.Title = appendText(ref.Title, text)
			case "source":
				ref.Container = appendText(ref.Container, text)
			case "year":
				ref.Year = appendText(ref.Year, text)
			case "volume":
				ref.Volume = appendText(ref.Volume, text)
			case "issue":
				ref.Issue = appendText(ref.Issue, text)
			case "fpage":
				ref.FPage = appendText(ref.FPage, text)
			case "lpage":
				ref.LPage = appendText(ref.LPage, text)
			case "surname":
				if currentAuthor != nil {
					currentAuthor.Family = appendText(currentAuthor.Family, text)
				}
			case "given-names":
				if currentAuthor != nil {
					currentAuthor.Given = appendText(currentAuthor.Given, text)
				}
			case "collab":
				ref.Authors = append(ref.Authors, Author{Name: text})
			case "pub-id":
				kind := ""
				for _, attr := range stack[len(stack)-1].Attr {
					if attr.Name.Local == "pub-id-type" {
						kind = strings.ToLower(attr.Value)
					}
				}
				switch kind {
				case "doi":
					ref.DOI = normalizeDOI(text)
				case "pmid":
					ref.PMID = text
				case "pmcid", "pmc":
					ref.PMCID = normalizePMCID(text)
				}
			}
		case xml.EndElement:
			if value.Name.Local == "name" && currentAuthor != nil {
				ref.Authors = append(ref.Authors, *currentAuthor)
				currentAuthor = nil
			}
			stack = stack[:len(stack)-1]
		}
	}
	ref.Raw = cleanSpace(raw.String())
	return ref, nil
}

func semanticJATSElement(stack []xml.StartElement) string {
	for i := len(stack) - 1; i >= 0; i-- {
		switch stack[i].Name.Local {
		case "article-title", "chapter-title", "source", "year", "volume", "issue", "fpage", "lpage", "surname", "given-names", "collab", "pub-id":
			return stack[i].Name.Local
		}
	}
	return stack[len(stack)-1].Name.Local
}

func appendText(current, extra string) string {
	if current == "" {
		return extra
	}
	return current + " " + extra
}
