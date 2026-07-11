package references

import (
	"encoding/xml"
	"strings"
)

type pubmedArticleSet struct {
	Articles []pubmedArticle `xml:"PubmedArticle"`
}

type pubmedArticle struct {
	Citation struct {
		PMID    string `xml:"PMID"`
		Article struct {
			Title   innerText `xml:"ArticleTitle"`
			Journal struct {
				Title string `xml:"Title"`
				Issue struct {
					Volume  string `xml:"Volume"`
					Issue   string `xml:"Issue"`
					PubDate struct {
						Year    string `xml:"Year"`
						Medline string `xml:"MedlineDate"`
					} `xml:"PubDate"`
				} `xml:"JournalIssue"`
			} `xml:"Journal"`
			Pagination struct {
				Pages string `xml:"MedlinePgn"`
			} `xml:"Pagination"`
			Authors []struct {
				Collective string `xml:"CollectiveName"`
				Last       string `xml:"LastName"`
				Fore       string `xml:"ForeName"`
			} `xml:"AuthorList>Author"`
			PublicationTypes []innerText `xml:"PublicationTypeList>PublicationType"`
			Grants           []struct {
				ID      string `xml:"GrantID"`
				Agency  string `xml:"Agency"`
				Country string `xml:"Country"`
				Acronym string `xml:"Acronym"`
			} `xml:"GrantList>Grant"`
		} `xml:"Article"`
		Mesh []struct {
			Descriptor struct {
				UI    string `xml:"UI,attr"`
				Major string `xml:"MajorTopicYN,attr"`
				Text  string `xml:",chardata"`
			} `xml:"DescriptorName"`
			Qualifiers []struct {
				UI    string `xml:"UI,attr"`
				Major string `xml:"MajorTopicYN,attr"`
				Text  string `xml:",chardata"`
			} `xml:"QualifierName"`
		} `xml:"MeshHeadingList>MeshHeading"`
		Keywords  []innerText `xml:"KeywordList>Keyword"`
		Chemicals []struct {
			Registry  string `xml:"RegistryNumber"`
			Substance struct {
				UI   string `xml:"UI,attr"`
				Text string `xml:",chardata"`
			} `xml:"NameOfSubstance"`
		} `xml:"ChemicalList>Chemical"`
		Corrections []struct {
			Type string `xml:"RefType,attr"`
			PMID string `xml:"PMID"`
			Ref  string `xml:"RefSource"`
		} `xml:"CommentsCorrectionsList>CommentsCorrections"`
	} `xml:"MedlineCitation"`
	Data struct {
		IDs []struct {
			Type  string `xml:"IdType,attr"`
			Value string `xml:",chardata"`
		} `xml:"ArticleIdList>ArticleId"`
	} `xml:"PubmedData"`
}

type pubmedRecord struct {
	PMID      string
	PMCID     string
	DOI       string
	Title     string
	Authors   []Author
	Container string
	Year      string
	Volume    string
	Issue     string
	Pages     string
	Metadata  PubMedMetadata
}

func (article pubmedArticle) record() pubmedRecord {
	record := pubmedRecord{
		PMID:      strings.TrimSpace(article.Citation.PMID),
		Title:     cleanSpace(article.Citation.Article.Title.Text),
		Container: cleanSpace(article.Citation.Article.Journal.Title),
		Year:      cleanSpace(article.Citation.Article.Journal.Issue.PubDate.Year),
		Volume:    cleanSpace(article.Citation.Article.Journal.Issue.Volume),
		Issue:     cleanSpace(article.Citation.Article.Journal.Issue.Issue),
		Pages:     cleanSpace(article.Citation.Article.Pagination.Pages),
	}
	if record.Year == "" {
		date := cleanSpace(article.Citation.Article.Journal.Issue.PubDate.Medline)
		if len(date) >= 4 {
			record.Year = date[:4]
		}
	}
	for _, author := range article.Citation.Article.Authors {
		record.Authors = append(record.Authors, Author{Family: cleanSpace(author.Last), Given: cleanSpace(author.Fore), Name: cleanSpace(author.Collective)})
	}
	for _, value := range article.Citation.Article.PublicationTypes {
		if v := cleanSpace(value.Text); v != "" {
			record.Metadata.PublicationTypes = append(record.Metadata.PublicationTypes, v)
		}
	}
	for _, value := range article.Citation.Keywords {
		if v := cleanSpace(value.Text); v != "" {
			record.Metadata.Keywords = append(record.Metadata.Keywords, v)
		}
	}
	for _, heading := range article.Citation.Mesh {
		term := MeSHTerm{UI: cleanSpace(heading.Descriptor.UI), Name: cleanSpace(heading.Descriptor.Text), MajorTopic: strings.EqualFold(heading.Descriptor.Major, "Y")}
		for _, q := range heading.Qualifiers {
			term.Qualifiers = append(term.Qualifiers, MeSHQualifier{UI: cleanSpace(q.UI), Name: cleanSpace(q.Text), MajorTopic: strings.EqualFold(q.Major, "Y")})
		}
		if term.Name != "" {
			record.Metadata.MeSH = append(record.Metadata.MeSH, term)
		}
	}
	for _, value := range article.Citation.Chemicals {
		record.Metadata.Chemicals = append(record.Metadata.Chemicals, Chemical{UI: cleanSpace(value.Substance.UI), Name: cleanSpace(value.Substance.Text), RegistryNumber: cleanSpace(value.Registry)})
	}
	for _, value := range article.Citation.Article.Grants {
		record.Metadata.Grants = append(record.Metadata.Grants, Grant{ID: cleanSpace(value.ID), Agency: cleanSpace(value.Agency), Country: cleanSpace(value.Country), Acronym: cleanSpace(value.Acronym)})
	}
	for _, value := range article.Citation.Corrections {
		record.Metadata.Corrections = append(record.Metadata.Corrections, Correction{Type: cleanSpace(value.Type), PMID: cleanSpace(value.PMID), Ref: cleanSpace(value.Ref)})
	}
	for _, id := range article.Data.IDs {
		switch strings.ToLower(id.Type) {
		case "doi":
			record.DOI = normalizeDOI(id.Value)
		case "pmc":
			record.PMCID = normalizePMCID(id.Value)
		case "pubmed":
			if record.PMID == "" {
				record.PMID = strings.TrimSpace(id.Value)
			}
		}
	}
	return record
}

func (record pubmedRecord) reference(index int, source Source) Reference {
	return Reference{Index: index, Title: record.Title, Authors: record.Authors, Container: record.Container, Year: record.Year, Volume: record.Volume, Issue: record.Issue, Pages: record.Pages, DOI: record.DOI, PMID: record.PMID, PMCID: record.PMCID, Source: source}
}

type innerText struct {
	Text string
}

func (text *innerText) UnmarshalXML(decoder *xml.Decoder, start xml.StartElement) error {
	var builder strings.Builder
	depth := 1
	for depth > 0 {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch value := token.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		case xml.CharData:
			builder.Write(value)
		}
	}
	text.Text = cleanSpace(builder.String())
	return nil
}
