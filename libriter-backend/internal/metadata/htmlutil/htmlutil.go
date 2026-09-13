// Package htmlutil obsahuje drobné pomocníky pro scrapery, které čtou HTML
// cizích webů (databazeknih.cz, cbdb.cz).
package htmlutil

import (
	"strings"

	"golang.org/x/net/html"
)

// Attr vrátí hodnotu atributu, nebo prázdný řetězec.
func Attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// HasClass zjistí, jestli má uzel danou CSS třídu.
func HasClass(n *html.Node, class string) bool {
	for _, c := range strings.Fields(Attr(n, "class")) {
		if c == class {
			return true
		}
	}
	return false
}

// Text posbírá veškerý textový obsah podstromu.
func Text(n *html.Node) string {
	var sb strings.Builder
	Walk(n, func(n *html.Node) bool {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		return true
	})
	return sb.String()
}

// Collapse sloučí bílé znaky na jednotlivé mezery a ořízne okraje.
func Collapse(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// Walk projde strom do hloubky. Vrátí-li fn false, potomci uzlu se přeskočí.
func Walk(n *html.Node, fn func(*html.Node) bool) {
	if n == nil || !fn(n) {
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		Walk(c, fn)
	}
}

// Find vrátí první uzel, pro který match vrátí true.
func Find(n *html.Node, match func(*html.Node) bool) *html.Node {
	var found *html.Node
	Walk(n, func(n *html.Node) bool {
		if found != nil {
			return false
		}
		if match(n) {
			found = n
			return false
		}
		return true
	})
	return found
}

// ByID hledá element s daným id.
func ByID(n *html.Node, id string) *html.Node {
	return Find(n, func(n *html.Node) bool {
		return n.Type == html.ElementNode && Attr(n, "id") == id
	})
}

// Element hledá první element daného jména (například "h1").
func Element(n *html.Node, name string) *html.Node {
	return Find(n, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == name
	})
}

// MetaContent vrátí obsah <meta property="..."> nebo <meta name="...">.
func MetaContent(doc *html.Node, key string) string {
	node := Find(doc, func(n *html.Node) bool {
		if n.Type != html.ElementNode || n.Data != "meta" {
			return false
		}
		return Attr(n, "property") == key || Attr(n, "name") == key
	})
	if node == nil {
		return ""
	}
	return Attr(node, "content")
}

// ResolveURL doplní relativní odkaz na absolutní vůči base.
// Zvládá i odkazy bez úvodního lomítka ("kniha-975-…"), které cizí weby používají.
func ResolveURL(base, href string) string {
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	if strings.HasPrefix(href, "//") {
		return "https:" + href
	}
	return strings.TrimSuffix(base, "/") + "/" + strings.TrimPrefix(href, "/")
}
