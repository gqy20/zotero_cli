package zoteroapi

import (
	"strings"

	nethtml "golang.org/x/net/html"
)

func bibliographyText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if !strings.Contains(value, "<") {
		return value
	}

	doc, err := nethtml.Parse(strings.NewReader(value))
	if err != nil {
		return compactWhitespace(stripHTML(value))
	}
	entries := bibliographyEntries(doc)
	if len(entries) == 0 {
		return compactWhitespace(stripHTML(value))
	}
	return strings.Join(entries, "\n")
}

func bibliographyEntries(root *nethtml.Node) []string {
	entries := make([]string, 0)
	var walk func(*nethtml.Node)
	walk = func(node *nethtml.Node) {
		if node.Type == nethtml.ElementNode && hasHTMLClass(node, "csl-entry") {
			if text := compactWhitespace(nodeText(node)); text != "" {
				entries = append(entries, text)
			}
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return entries
}

func hasHTMLClass(node *nethtml.Node, class string) bool {
	for _, attr := range node.Attr {
		if attr.Key == "class" {
			for _, value := range strings.Fields(attr.Val) {
				if value == class {
					return true
				}
			}
		}
	}
	return false
}

func nodeText(node *nethtml.Node) string {
	var text strings.Builder
	var walk func(*nethtml.Node)
	walk = func(current *nethtml.Node) {
		if current.Type == nethtml.TextNode {
			text.WriteString(current.Data)
			text.WriteByte(' ')
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return text.String()
}
