package httpx

import (
	"net/http/httptest"
	"testing"
)

func TestParsePage(t *testing.T) {
	cases := []struct {
		url    string
		paged  bool
		limit  int
		offset int
	}{
		{"/x", false, 0, 0},
		{"/x?offset=10", false, 0, 0},
		{"/x?limit=50", true, 50, 0},
		{"/x?limit=50&offset=200", true, 50, 200},
		{"/x?limit=0", true, 1, 0},
		{"/x?limit=99999&offset=-5", true, MaxPageLimit, 0},
		{"/x?limit=abc", true, DefaultPageLimit, 0},
	}
	for _, c := range cases {
		p, paged := ParsePage(httptest.NewRequest("GET", c.url, nil))
		if paged != c.paged || p.Limit != c.limit || p.Offset != c.offset {
			t.Errorf("%s: dostal %+v paged=%v", c.url, p, paged)
		}
	}
}
