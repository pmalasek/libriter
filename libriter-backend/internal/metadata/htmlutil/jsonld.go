package htmlutil

import (
	"encoding/json"
	"strings"

	"golang.org/x/net/html"
)

// Person je výřez schema.org/Person z JSON-LD. Oba české weby
// (databazeknih.cz i cbdb.cz) ho na stránce autora mají a je mnohem
// stabilnější než hledání v HTML – proto se čte přednostně.
type Person struct {
	Type        string `json:"@type"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Image       string `json:"image"`
	Description string `json:"description"`
	BirthDate   string `json:"birthDate"`
	DeathDate   string `json:"deathDate"`
}

// FindPerson najde první JSON-LD blok typu Person. Vrací nil, pokud na
// stránce žádný není nebo se nepodaří rozparsovat.
func FindPerson(doc *html.Node) *Person {
	var found *Person

	Walk(doc, func(n *html.Node) bool {
		if found != nil {
			return false
		}
		if n.Type != html.ElementNode || n.Data != "script" {
			return true
		}
		if !strings.Contains(Attr(n, "type"), "ld+json") {
			return true
		}

		var person Person
		if err := json.Unmarshal([]byte(Text(n)), &person); err != nil {
			return true // jiný tvar JSON-LD (pole, graf) – prostě přeskočíme
		}
		if person.Type == "Person" && person.Name != "" {
			found = &person
		}
		return true
	})

	return found
}
