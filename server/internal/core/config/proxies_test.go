package config

import (
	"net/netip"
	"path/filepath"
	"slices"
	"testing"
)

func TestParseTrustedProxies(t *testing.T) {
	got, err := ParseTrustedProxies([]string{"127.0.0.0/8", "::1", " 10.0.0.5 ", "", "192.168.1.77/24", "::ffff:172.24.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	want := []netip.Prefix{
		netip.MustParsePrefix("127.0.0.0/8"),
		netip.MustParsePrefix("::1/128"),
		netip.MustParsePrefix("10.0.0.5/32"),
		netip.MustParsePrefix("192.168.1.0/24"),
		netip.MustParsePrefix("172.24.0.1/32"),
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, chci %v", got, want)
	}

	for _, bad := range []string{"localhost", "10.0.0.0/33", "1.2.3"} {
		if _, err := ParseTrustedProxies([]string{bad}); err == nil {
			t.Errorf("%q: chci chybu", bad)
		}
	}
}

func TestLoadTrustedProxies(t *testing.T) {
	t.Setenv("LIBRITER_ENV_FILE", filepath.Join(t.TempDir(), "missing.env"))
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef-test")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(cfg.Server.TrustedProxies, netip.MustParsePrefix("127.0.0.0/8")) ||
		!slices.Contains(cfg.Server.TrustedProxies, netip.MustParsePrefix("::1/128")) {
		t.Fatalf("výchozí TRUSTED_PROXIES: %v", cfg.Server.TrustedProxies)
	}

	t.Setenv("TRUSTED_PROXIES", "172.24.0.0/16")
	if cfg, err = Load(); err != nil || len(cfg.Server.TrustedProxies) != 1 {
		t.Fatalf("vlastní TRUSTED_PROXIES: %v %v", cfg, err)
	}

	t.Setenv("TRUSTED_PROXIES", "172.24.0.0/16,nginx")
	if _, err := Load(); err == nil {
		t.Fatal("neplatná položka TRUSTED_PROXIES musí start zastavit")
	}
}
