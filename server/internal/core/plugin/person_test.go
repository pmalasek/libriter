package plugin

import (
	"encoding/json"
	"testing"
)

func TestPersonUnmarshal(t *testing.T) {
	var c VideoCredits
	err := json.Unmarshal([]byte(`{"directors":["Lana Wachowski"],
		"cast":[{"id":"6384","name":"Keanu Reeves","character":"Neo","image_url":"https://image.tmdb.org/p.jpg"}]}`), &c)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Directors) != 1 || c.Directors[0].Name != "Lana Wachowski" || c.Directors[0].ID != "" {
		t.Errorf("řetězec %+v", c.Directors)
	}
	if len(c.Cast) != 1 || c.Cast[0].ID != "6384" || c.Cast[0].Character != "Neo" || c.Cast[0].ImageURL == "" {
		t.Errorf("objekt %+v", c.Cast)
	}
}
