package importer

import (
	"bufio"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/text/unicode/norm"
)

// bookInfo jsou metadata z doprovodných souborů knihy (bookinfo.html
// z e-shopů s audioknihami a playlist.pls).
type bookInfo struct {
	Title       string
	Author      string
	Narrator    string
	Publisher   string
	Description string
	// Chapters jsou kapitoly v pořadí, v jakém je soubor uvádí.
	Chapters []infoChapter
}

type infoChapter struct {
	File  string // název souboru (bez adresáře)
	Title string
}

// parseBookInfo přečte bookinfo.html. Prvky se hledají podle id a tříd,
// popisky u nich bývají střídavě česky a německy („Autor“, „Liest“).
func parseBookInfo(absPath string) (*bookInfo, error) {
	f, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	doc, err := html.Parse(f)
	if err != nil {
		return nil, err
	}

	info := &bookInfo{}
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch attr(n, "id") {
			case "Title":
				info.Title = nodeText(n)
			case "Author":
				info.Author = nodeText(n)
			case "Reader":
				info.Narrator = nodeText(n)
			case "Publisher":
				info.Publisher = nodeText(n)
			case "GeneralDescription":
				info.Description = nodeText(n)
			}
			if strings.HasPrefix(attr(n, "id"), "Chapter-") {
				if ch := parseInfoChapter(n); ch.File != "" {
					info.Chapters = append(info.Chapters, ch)
				}
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return info, nil
}

// parseInfoChapter přečte blok jedné kapitoly (<h2 class="ChapterTitle">,
// <p class="Link">).
func parseInfoChapter(n *html.Node) infoChapter {
	var ch infoChapter
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch attr(n, "class") {
			case "ChapterTitle":
				ch.Title = nodeText(n)
			case "Link":
				ch.File = nodeText(n)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return ch
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

var blankLines = regexp.MustCompile(`\n{3,}`)

// nodeText vrátí text prvku; řádky zachová, nadbytečné mezery sloučí.
func nodeText(n *html.Node) string {
	var b strings.Builder
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		switch n.Type {
		case html.TextNode:
			b.WriteString(n.Data)
		case html.ElementNode:
			if n.Data == "br" {
				b.WriteString("\n")
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)

	text := strings.ReplaceAll(b.String(), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		lines[i] = strings.Join(strings.Fields(l), " ")
	}
	text = blankLines.ReplaceAllString(strings.Join(lines, "\n"), "\n\n")
	return norm.NFC.String(strings.TrimSpace(text))
}

var plsEntry = regexp.MustCompile(`(?i)^(file|title)(\d+)=(.*)$`)

// parsePLS přečte playlist.pls ([playlist], FileN=…, TitleN=…).
func parsePLS(absPath string) ([]infoChapter, error) {
	f, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	byIndex := map[int]*infoChapter{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		m := plsEntry.FindStringSubmatch(strings.TrimSpace(sc.Text()))
		if m == nil {
			continue
		}
		idx, _ := strconv.Atoi(m[2])
		ch := byIndex[idx]
		if ch == nil {
			ch = &infoChapter{}
			byIndex[idx] = ch
		}
		value := norm.NFC.String(strings.TrimSpace(m[3]))
		if strings.EqualFold(m[1], "file") {
			// Playlist může obsahovat cestu – páruje se podle názvu souboru.
			value = strings.ReplaceAll(value, "\\", "/")
			value = value[strings.LastIndex(value, "/")+1:]
			ch.File = value
		} else {
			ch.Title = value
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	indexes := make([]int, 0, len(byIndex))
	for idx := range byIndex {
		indexes = append(indexes, idx)
	}
	sort.Ints(indexes)

	out := make([]infoChapter, 0, len(indexes))
	for _, idx := range indexes {
		if ch := byIndex[idx]; ch.File != "" {
			out = append(out, *ch)
		}
	}
	return out, nil
}
