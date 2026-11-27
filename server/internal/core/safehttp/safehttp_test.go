package safehttp

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
)

func TestIsPublic(t *testing.T) {
	for addr, want := range map[string]bool{
		"8.8.8.8":          true,
		"151.101.1.69":     true,
		"2606:4700::1111":  true,
		"127.0.0.1":        false,
		"127.8.9.10":       false,
		"10.1.2.3":         false,
		"172.16.0.1":       false,
		"172.31.255.255":   false,
		"192.168.1.1":      false,
		"169.254.169.254":  false, // metadata cloudu
		"100.64.0.1":       false, // CGNAT
		"100.127.255.254":  false,
		"0.0.0.0":          false,
		"224.0.0.1":        false,
		"255.255.255.255":  false,
		"::1":              false,
		"::":               false,
		"fc00::1":          false, // ULA
		"fd12:3456::1":     false,
		"fe80::1":          false, // link-local
		"ff02::1":          false, // multicast
		"::ffff:127.0.0.1": false, // IPv4 zabalená do IPv6
		"::ffff:10.0.0.1":  false,
		"64:ff9b::a00:1":   false, // NAT64 na 10.0.0.1
		"2002:a00:1::1":    false, // 6to4 s 10.0.0.1
	} {
		if got := IsPublic(netip.MustParseAddr(addr)); got != want {
			t.Errorf("IsPublic(%s) = %v, chtěno %v", addr, got, want)
		}
	}
}

func TestCheckPorts(t *testing.T) {
	c := newConfig(nil)
	for addr, ok := range map[string]bool{
		"8.8.8.8:443":   true,
		"8.8.8.8:80":    true,
		"8.8.8.8:8080":  false,
		"8.8.8.8:22":    false,
		"[::1]:443":     false,
		"127.0.0.1:443": false,
	} {
		err := c.check("tcp", addr)
		if (err == nil) != ok {
			t.Errorf("check(%s) = %v, chtěno ok=%v", addr, err, ok)
		}
		if err != nil && !IsBlocked(err) {
			t.Errorf("check(%s): chyba není ErrBlocked: %v", addr, err)
		}
	}
	if err := c.check("udp", "8.8.8.8:443"); err == nil {
		t.Error("UDP prošlo")
	}
}

// Bez výjimky se klient na 127.0.0.1 nepřipojí – server se o požadavku
// vůbec nedozví.
func TestClientBlocksLoopback(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
	}))
	defer srv.Close()

	_, err := NewClient().Get(srv.URL)
	if !IsBlocked(err) {
		t.Fatalf("spojení na loopback: %v, chtěno ErrBlocked", err)
	}
	if hits.Load() != 0 {
		t.Fatal("požadavek na loopback dorazil")
	}
}

// Povolená adresa (test proti httptest) projde, přesměrování na jinou
// místní adresu ale ne – kontrola běží u každého spojení.
func TestClientBlocksRedirectToLoopback(t *testing.T) {
	var internalHits atomic.Int32
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		internalHits.Add(1)
		_, _ = io.WriteString(w, "tajne")
	}))
	defer internal.Close()

	public := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			_, _ = io.WriteString(w, "ok")
		default:
			http.Redirect(w, r, internal.URL+"/admin", http.StatusFound)
		}
	}))
	defer public.Close()

	client := NewClient(AllowAddr(public.Listener.Addr().String()))
	resp, err := client.Get(public.URL + "/ok")
	if err != nil {
		t.Fatalf("povolená adresa: %v", err)
	}
	resp.Body.Close()

	_, err = client.Get(public.URL + "/redirect")
	if !IsBlocked(err) {
		t.Fatalf("přesměrování na loopback: %v, chtěno ErrBlocked", err)
	}
	if internalHits.Load() != 0 {
		t.Fatal("přesměrovaný požadavek dorazil na vnitřní server")
	}
}

func TestRedirectLimits(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/file":
			http.Redirect(w, r, "file:///etc/passwd", http.StatusFound)
		default:
			http.Redirect(w, r, srv.URL+"/loop", http.StatusFound)
		}
	}))
	defer srv.Close()
	client := NewClient(AllowAddr(srv.Listener.Addr().String()))

	if _, err := client.Get(srv.URL + "/loop"); !errors.Is(err, ErrTooManyRedirects) {
		t.Errorf("nekonečné přesměrování: %v", err)
	}
	if _, err := client.Get(srv.URL + "/file"); !errors.Is(err, ErrScheme) {
		t.Errorf("přesměrování na file://: %v", err)
	}
}

func TestHostMatches(t *testing.T) {
	for _, tt := range []struct {
		raw, host string
		want      bool
	}{
		{"https://www.databazeknih.cz/x", "databazeknih.cz", true},
		{"https://databazeknih.cz/x", "databazeknih.cz", true},
		{"http://DATABAZEKNIH.CZ/x", "databazeknih.cz", true},
		{"https://databazeknih.cz:443/x", "databazeknih.cz", true},
		{"http://databazeknih.cz:80/x", "databazeknih.cz", true},
		// Jiný port je jiná služba na stejném stroji.
		{"https://databazeknih.cz:8443/x", "databazeknih.cz", false},
		{"http://databazeknih.cz:443/x", "databazeknih.cz", false},
		{"https://databazeknih.cz:22/x", "databazeknih.cz", false},
		{"https://databazeknih.cz.zlo.net/x", "databazeknih.cz", false},
		{"https://zlodatabazeknih.cz/x", "databazeknih.cz", false},
		{"file:///etc/passwd", "databazeknih.cz", false},
		{"", "databazeknih.cz", false},
		{"https://x.cz/", "", false},
	} {
		if got := HostMatches(tt.raw, tt.host); got != tt.want {
			t.Errorf("HostMatches(%q, %q) = %v, chtěno %v", tt.raw, tt.host, got, tt.want)
		}
	}
}

func TestReadAllAndLimitBody(t *testing.T) {
	if _, err := ReadAll(strings.NewReader("0123456789"), 5); !errors.Is(err, ErrTooLarge) {
		t.Errorf("ReadAll nad strop: %v", err)
	}
	if data, err := ReadAll(strings.NewReader("01234"), 5); err != nil || string(data) != "01234" {
		t.Errorf("ReadAll = %q %v", data, err)
	}

	body := LimitBody(io.NopCloser(strings.NewReader("0123456789")), 4)
	data, err := io.ReadAll(body)
	if !errors.Is(err, ErrTooLarge) || string(data) != "0123" {
		t.Errorf("LimitBody nad strop: %q %v", data, err)
	}
	body = LimitBody(io.NopCloser(strings.NewReader("0123")), 4)
	if data, err := io.ReadAll(body); err != nil || string(data) != "0123" {
		t.Errorf("LimitBody na hranici: %q %v", data, err)
	}
}
